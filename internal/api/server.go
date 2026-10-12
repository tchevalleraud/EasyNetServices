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
	"github.com/tchevalleraud/EasyNetServices/internal/dhcp"
	"github.com/tchevalleraud/EasyNetServices/internal/dns"
	"github.com/tchevalleraud/EasyNetServices/internal/ntp"
	"github.com/tchevalleraud/EasyNetServices/internal/store"
	"github.com/tchevalleraud/EasyNetServices/internal/system"
	"github.com/tchevalleraud/EasyNetServices/internal/validation"
)

const maxBodySize = 1 << 20

// Server holds the dependencies of the HTTP handlers.
type Server struct {
	cfg          *config.Config
	store        *store.Store
	auth         *auth.Service
	ntp          *ntp.Manager
	dns          *dns.Manager
	dhcp         *dhcp.Manager
	system       *system.Manager
	ui           fs.FS
	log          *slog.Logger
	startedAt    time.Time
	secureCookie bool
	loginLimiter *rateLimiter
}

// Services groups the service managers exposed by the API.
type Services struct {
	System *system.Manager
	NTP    *ntp.Manager
	DNS    *dns.Manager
	DHCP   *dhcp.Manager
}

// NewServer returns the HTTP server. ui contains the built web interface.
func NewServer(cfg *config.Config, st *store.Store, authSvc *auth.Service, svc Services, ui fs.FS, log *slog.Logger) *Server {
	return &Server{
		cfg:          cfg,
		store:        st,
		auth:         authSvc,
		ntp:          svc.NTP,
		system:       svc.System,
		dns:          svc.DNS,
		dhcp:         svc.DHCP,
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
	mux.Handle("GET /api/v1/system/settings", s.requireAuth(s.handleGetSystemSettings, false))
	mux.Handle("PUT /api/v1/system/settings", s.requireAuth(s.handlePutSystemSettings, false))
	mux.Handle("GET /api/v1/system/time", s.requireAuth(s.handleGetSystemTime, false))
	mux.Handle("GET /api/v1/ntp/config", s.requireAuth(s.handleGetNTPConfig, false))
	mux.Handle("PUT /api/v1/ntp/config", s.requireAuth(s.handlePutNTPConfig, false))
	mux.Handle("GET /api/v1/ntp/status", s.requireAuth(s.handleGetNTPStatus, false))
	mux.Handle("GET /api/v1/dns/settings", s.requireAuth(s.handleGetDNSSettings, false))
	mux.Handle("PUT /api/v1/dns/settings", s.requireAuth(s.handlePutDNSSettings, false))
	mux.Handle("GET /api/v1/dns/status", s.requireAuth(s.handleGetDNSStatus, false))
	mux.Handle("GET /api/v1/dns/zones", s.requireAuth(s.handleListZones, false))
	mux.Handle("POST /api/v1/dns/zones", s.requireAuth(s.handleCreateZone, false))
	mux.Handle("GET /api/v1/dns/zones/{zone}", s.requireAuth(s.handleGetZone, false))
	mux.Handle("DELETE /api/v1/dns/zones/{zone}", s.requireAuth(s.handleDeleteZone, false))
	mux.Handle("PUT /api/v1/dns/zones/{zone}/rrsets", s.requireAuth(s.handlePutRRSet, false))
	mux.Handle("DELETE /api/v1/dns/zones/{zone}/rrsets/{name}/{type}", s.requireAuth(s.handleDeleteRRSet, false))
	mux.Handle("GET /api/v1/dhcp/settings", s.requireAuth(s.handleGetDHCPSettings, false))
	mux.Handle("PUT /api/v1/dhcp/settings", s.requireAuth(s.handlePutDHCPSettings, false))
	mux.Handle("GET /api/v1/dhcp/status", s.requireAuth(s.handleGetDHCPStatus, false))
	mux.Handle("GET /api/v1/dhcp/interfaces", s.requireAuth(s.handleListDHCPInterfaces, false))
	mux.Handle("GET /api/v1/dhcp/{family}/subnets", s.requireAuth(s.handleListSubnets, false))
	mux.Handle("POST /api/v1/dhcp/{family}/subnets", s.requireAuth(s.handleCreateSubnet, false))
	mux.Handle("GET /api/v1/dhcp/{family}/subnets/{id}", s.requireAuth(s.handleGetSubnet, false))
	mux.Handle("PUT /api/v1/dhcp/{family}/subnets/{id}", s.requireAuth(s.handlePutSubnet, false))
	mux.Handle("DELETE /api/v1/dhcp/{family}/subnets/{id}", s.requireAuth(s.handleDeleteSubnet, false))
	mux.Handle("GET /api/v1/dhcp/{family}/leases", s.requireAuth(s.handleListLeases, false))
	mux.Handle("DELETE /api/v1/dhcp/{family}/leases/{address}", s.requireAuth(s.handleDeleteLease, false))
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
		// Details lists the invalid fields of a validation_failed error.
		Details validation.Errors `json:"details,omitempty"`
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

// writeUpdateError reports the error of a configuration update: field errors
// are returned as validation_failed, anything else as an internal error.
func (s *Server) writeUpdateError(w http.ResponseWriter, r *http.Request, err error) {
	var errs validation.Errors
	if errors.As(err, &errs) {
		var e apiError
		e.Error.Code = "validation_failed"
		e.Error.Message = "the configuration is invalid"
		e.Error.Details = errs
		writeJSON(w, http.StatusBadRequest, e)
		return
	}
	s.internalError(w, r, err)
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
