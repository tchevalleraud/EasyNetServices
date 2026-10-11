// Package api implements the REST API and serves the web interface.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/tchevalleraud/EasyNetServices/internal/auth"
	"github.com/tchevalleraud/EasyNetServices/internal/config"
	"github.com/tchevalleraud/EasyNetServices/internal/store"
)

const maxBodySize = 1 << 20

// Server holds the dependencies of the HTTP handlers.
type Server struct {
	cfg          *config.Config
	store        *store.Store
	auth         *auth.Service
	ui           fs.FS
	log          *slog.Logger
	startedAt    time.Time
	secureCookie bool
	loginLimiter *rateLimiter
}

// NewServer returns the HTTP server. ui contains the built web interface.
func NewServer(cfg *config.Config, st *store.Store, authSvc *auth.Service, ui fs.FS, log *slog.Logger) *Server {
	return &Server{
		cfg:          cfg,
		store:        st,
		auth:         authSvc,
		ui:           ui,
		log:          log,
		startedAt:    time.Now(),
		secureCookie: cfg.TLSMode != config.TLSOff,
		loginLimiter: newRateLimiter(10, 5*time.Minute),
	}
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleHealth)

	mux.HandleFunc("GET /api/v1/version", s.handleVersion)
	mux.HandleFunc("POST /api/v1/auth/login", s.handleLogin)
	mux.Handle("POST /api/v1/auth/logout", s.requireAuth(s.handleLogout, true))
	mux.Handle("GET /api/v1/auth/me", s.requireAuth(s.handleMe, true))
	mux.Handle("POST /api/v1/auth/password", s.requireAuth(s.handleChangePassword, true))
	mux.Handle("GET /api/v1/system", s.requireAuth(s.handleSystem, false))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "endpoint not found")
	})

	mux.Handle("/", s.uiHandler())

	return s.recoverer(s.logRequests(securityHeaders(checkOrigin(mux))))
}

// apiError is the error payload returned by every endpoint.
type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		json.NewEncoder(w).Encode(v)
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var e apiError
	e.Error.Code = code
	e.Error.Message = message
	writeJSON(w, status, e)
}

// decodeJSON decodes a JSON request body into v, writing an error response on failure.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "request body too large")
		} else {
			writeError(w, http.StatusBadRequest, "invalid_body", "invalid JSON body")
		}
		return false
	}
	if _, err := dec.Token(); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid_body", "unexpected data after JSON body")
		return false
	}
	return true
}
