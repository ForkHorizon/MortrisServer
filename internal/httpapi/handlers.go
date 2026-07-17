package httpapi

import (
	"net/http"
	"time"

	"github.com/ForkHorizon/Mortris/internal/contracts"
)

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	requestID := newRequestID()
	start := time.Now()

	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	data, err := readBody(w, r)
	if err != nil {
		status := writeError(w, s.Log, requestID, badRequest(err))
		s.logRequest(r, requestID, status, start, nil)
		return
	}

	req, err := contracts.DecodeRegisterRequest(data)
	if err != nil {
		status := writeError(w, s.Log, requestID, decodeErr(err))
		s.logRequest(r, requestID, status, start, nil)
		return
	}

	resp, err := s.Ingest.Register(r.Context(), req, sourceIP(r))
	if err != nil {
		status := writeError(w, s.Log, requestID, err)
		s.logRequest(r, requestID, status, start, nil)
		return
	}
	writeJSON(w, http.StatusOK, resp)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}

func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request) {
	requestID := newRequestID()
	start := time.Now()

	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	data, err := readBody(w, r)
	if err != nil {
		status := writeError(w, s.Log, requestID, badRequest(err))
		s.logRequest(r, requestID, status, start, nil)
		return
	}

	req, decodeRejections, err := contracts.DecodeBatchIngestRequest(data)
	if err != nil {
		status := writeError(w, s.Log, requestID, decodeErr(err))
		s.logRequest(r, requestID, status, start, nil)
		return
	}

	resp, err := s.Ingest.Batch(r.Context(), req, decodeRejections, bearerToken(r), sourceIP(r))
	if err != nil {
		status := writeError(w, s.Log, requestID, err)
		s.logRequest(r, requestID, status, start, nil)
		return
	}
	writeJSON(w, http.StatusOK, resp)
	s.logRequest(r, requestID, http.StatusOK, start, map[string]any{
		"accepted":   len(resp.Accepted),
		"duplicates": len(resp.Duplicates),
		"rejected":   len(resp.Rejected),
	})
}

func (s *Server) handlePolicy(w http.ResponseWriter, r *http.Request) {
	requestID := newRequestID()
	start := time.Now()

	data, err := readBody(w, r)
	if err != nil {
		status := writeError(w, s.Log, requestID, badRequest(err))
		s.logRequest(r, requestID, status, start, nil)
		return
	}

	req, err := contracts.DecodePolicyRequest(data)
	if err != nil {
		status := writeError(w, s.Log, requestID, decodeErr(err))
		s.logRequest(r, requestID, status, start, nil)
		return
	}

	resp, err := s.Ingest.Policy(r.Context(), req, bearerToken(r), sourceIP(r))
	if err != nil {
		status := writeError(w, s.Log, requestID, err)
		s.logRequest(r, requestID, status, start, nil)
		return
	}
	writeJSON(w, http.StatusOK, resp)
	s.logRequest(r, requestID, http.StatusOK, start, nil)
}
