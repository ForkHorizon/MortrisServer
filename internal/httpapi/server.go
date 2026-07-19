// Package httpapi is the net/http layer: routing, body limits, timeouts,
// JSON encoding/decoding, and error rendering for the SDK-facing
// endpoints (section 5) and health checks (section 13.4). Business logic
// lives in internal/ingest — handlers here only translate HTTP <-> Go.
package httpapi

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/adminauth"
	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
	"github.com/ForkHorizon/Mortris/internal/diskstate"
	"github.com/ForkHorizon/Mortris/internal/ingest"
)

const (
	maxCompressedBody   = 256 * 1024
	maxDecompressedBody = 1024 * 1024
	handlerTimeout      = 10 * time.Second
	// ingestSemaphoreSize bounds concurrent decompression + ingestion work
	// (section 13.2), independent of Go's usual one-goroutine-per-request.
	ingestSemaphoreSize = 64
)

var errBodyTooLarge = errors.New("request body exceeds size limit")

type Server struct {
	Ingest        *ingest.Service
	Pool          *pgxpool.Pool // writer pool: SDK endpoints + admin auth (section 8.1)
	ReaderPool    *pgxpool.Pool // dashboard analytics queries only (section 8.1, 10.1)
	Log           *slog.Logger
	LoginThrottle *adminauth.Throttle

	sem                 chan struct{}
	dashboardFS         fs.FS
	dashboardFileServer http.Handler
}

func NewServer(ingestSvc *ingest.Service, pool, readerPool *pgxpool.Pool) *Server {
	dfs := dashboardFS()
	return &Server{
		Ingest:              ingestSvc,
		Pool:                pool,
		ReaderPool:          readerPool,
		Log:                 slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		LoginThrottle:       adminauth.NewThrottle(),
		sem:                 make(chan struct{}, ingestSemaphoreSize),
		dashboardFS:         dfs,
		dashboardFileServer: http.FileServer(http.FS(dfs)),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/installs/register", s.handleRegister)
	mux.HandleFunc("POST /v1/events/batch", s.handleBatch)
	mux.HandleFunc("POST /v1/client/policy", s.handlePolicy)
	mux.HandleFunc("GET /health/live", s.handleLive)
	mux.HandleFunc("GET /health/ready", s.handleReady)

	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/v1/auth/logout", s.requireSession(s.handleLogout))
	mux.HandleFunc("GET /api/v1/auth/session", s.requireSession(s.handleSessionInfo))

	mux.HandleFunc("GET /api/v1/analytics/overview", s.requireSession(s.handleOverview))
	mux.HandleFunc("GET /api/v1/analytics/events", s.requireSession(s.handleEventExplorer))
	mux.HandleFunc("GET /api/v1/analytics/funnel", s.requireSession(s.handleFunnel))
	mux.HandleFunc("GET /api/v1/analytics/retention", s.requireSession(s.handleRetention))
	mux.HandleFunc("GET /api/v1/analytics/installations/{id}", s.requireSession(s.handleInstallationTimeline))
	mux.HandleFunc("GET /api/v1/analytics/catalog", s.requireSession(s.handleCatalog))
	mux.HandleFunc("GET /api/v1/system", s.requireSession(s.handleSystemHealth))

	mux.HandleFunc("GET /api/v1/policy", s.requireSession(s.handlePolicyList))
	mux.HandleFunc("POST /api/v1/policy", s.requireSession(s.handlePolicyCreate))
	mux.HandleFunc("DELETE /api/v1/policy/{id}", s.requireSession(s.handlePolicyDelete))

	// Catch-all: the embedded dashboard SPA (section 13.1). Lowest
	// priority in ServeMux's pattern matching, so it never shadows any
	// route above.
	mux.HandleFunc("/", s.handleDashboard)

	return mux
}

// currentDiskState reads the live disk-pressure state from the same
// monitor internal/ingest gates batch ingestion on (section 12) — nil-safe
// since CLI-only wiring paths never set it.
func (s *Server) currentDiskState() diskstate.State {
	if s.Ingest == nil || s.Ingest.Disk == nil {
		return diskstate.Normal
	}
	return s.Ingest.Disk.Get()
}

// NewHTTPServer applies section 13.2's timeout requirements around Routes.
func (s *Server) NewHTTPServer(addr string) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           s.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	var one int
	if err := s.Pool.QueryRow(ctx, "SELECT 1").Scan(&one); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// readBody enforces the compressed/decompressed body limits (section
// 5.4, 13.2) and transparently gunzips when Content-Encoding: gzip is
// set. It does not trust Content-Length — the decompressed limit is
// enforced by capping the reader, not by checking a header.
func readBody(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.ContentLength > maxCompressedBody {
		return nil, errBodyTooLarge
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxCompressedBody)

	if r.Header.Get("Content-Encoding") != "gzip" {
		data, err := io.ReadAll(r.Body)
		return data, normalizeBodyError(err)
	}

	gz, err := gzip.NewReader(r.Body)
	if err != nil {
		return nil, normalizeBodyError(err)
	}
	defer func() { _ = gz.Close() }()

	limited := io.LimitReader(gz, maxDecompressedBody+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, normalizeBodyError(err)
	}
	if len(data) > maxDecompressedBody {
		return nil, errBodyTooLarge
	}
	return data, nil
}

func normalizeBodyError(err error) error {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return errBodyTooLarge
	}
	return err
}

// decodeJSONStrict is the dashboard-API equivalent of internal/contracts'
// strict decoder — unknown fields rejected, exactly one JSON value. Kept
// local since dashboard request bodies aren't part of the SDK wire
// contract package.
func decodeJSONStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("body must contain exactly one JSON value")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, log *slog.Logger, requestID string, err error) int {
	var ae *apierr.Error
	if errors.As(err, &ae) {
		if ae.RetryAfter > 0 {
			w.Header().Set("Retry-After", itoaSeconds(ae.RetryAfter))
		}
		writeJSON(w, ae.Status, contracts.ErrorResponse{
			ServerTime: nowRFC3339Millis(),
			Code:       ae.Code,
			Message:    ae.Message,
			RequestID:  requestID,
		})
		return ae.Status
	}

	log.Error("unhandled error", "request_id", requestID, "error", err)
	writeJSON(w, http.StatusInternalServerError, contracts.ErrorResponse{
		ServerTime: nowRFC3339Millis(),
		Code:       contracts.CodeInternal,
		Message:    "internal error",
		RequestID:  requestID,
	})
	return http.StatusInternalServerError
}

func itoaSeconds(d time.Duration) string {
	secs := int64(d.Seconds())
	if secs < 1 {
		secs = 1
	}
	buf := [20]byte{}
	i := len(buf)
	for secs > 0 {
		i--
		buf[i] = byte('0' + secs%10)
		secs /= 10
	}
	return string(buf[i:])
}

func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func nowRFC3339Millis() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

// sourceIP extracts the client IP for rate limiting. Trusts RemoteAddr
// directly — a reverse-proxy X-Forwarded-For rule belongs in Caddy
// (deploy/Caddyfile), not duplicated here where it could be spoofed by a
// direct connection.
func sourceIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) > len(prefix) && h[:len(prefix)] == prefix {
		return h[len(prefix):]
	}
	return ""
}
