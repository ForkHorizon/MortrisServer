// Package httpapi is the net/http layer: routing, body limits, timeouts,
// JSON encoding/decoding, and error rendering for the SDK-facing
// endpoints (section 5) and health checks (section 13.4). Business logic
// lives in internal/ingest — handlers here only translate HTTP <-> Go.
package httpapi

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ForkHorizon/Mortris/internal/apierr"
	"github.com/ForkHorizon/Mortris/internal/contracts"
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

type Server struct {
	Ingest *ingest.Service
	Pool   *pgxpool.Pool
	Log    *slog.Logger

	sem chan struct{}
}

func NewServer(ingestSvc *ingest.Service, pool *pgxpool.Pool) *Server {
	return &Server{
		Ingest: ingestSvc,
		Pool:   pool,
		Log:    slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		sem:    make(chan struct{}, ingestSemaphoreSize),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/installs/register", s.handleRegister)
	mux.HandleFunc("POST /v1/events/batch", s.handleBatch)
	mux.HandleFunc("POST /v1/client/policy", s.handlePolicy)
	mux.HandleFunc("GET /health/live", s.handleLive)
	mux.HandleFunc("GET /health/ready", s.handleReady)
	return mux
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
	r.Body = http.MaxBytesReader(w, r.Body, maxCompressedBody)

	if r.Header.Get("Content-Encoding") != "gzip" {
		return io.ReadAll(r.Body)
	}

	gz, err := gzip.NewReader(r.Body)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	limited := io.LimitReader(gz, maxDecompressedBody+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if len(data) > maxDecompressedBody {
		return nil, errors.New("decompressed body exceeds limit")
	}
	return data, nil
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
