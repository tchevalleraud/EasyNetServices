package api

import (
	"errors"
	"net/http"

	"github.com/tchevalleraud/EasyNetServices/internal/dns"
	"github.com/tchevalleraud/EasyNetServices/internal/validation"
)

// writeDNSError maps the errors of the DNS manager to API errors.
func (s *Server) writeDNSError(w http.ResponseWriter, r *http.Request, err error) {
	var rejected *dns.RejectedError
	switch {
	case errors.Is(err, dns.ErrZoneNotFound):
		writeError(w, http.StatusNotFound, "not_found", "zone or record not found")
	case errors.Is(err, dns.ErrZoneExists):
		writeError(w, http.StatusConflict, "zone_exists", "the zone already exists")
	case errors.Is(err, dns.ErrAuthUnavailable):
		writeError(w, http.StatusServiceUnavailable, "dns_unavailable", "the DNS server is not available")
	case errors.Is(err, dns.ErrApply):
		writeError(w, http.StatusInternalServerError, "apply_failed", err.Error())
	case errors.As(err, &rejected):
		// PowerDNS validates record contents; report its message on the records.
		s.writeUpdateError(w, r, validation.Errors{{Field: "records", Message: rejected.Message}})
	default:
		s.writeUpdateError(w, r, err)
	}
}

func (s *Server) handleGetDNSSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.dns.Settings(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handlePutDNSSettings(w http.ResponseWriter, r *http.Request) {
	var settings dns.Settings
	if !decodeJSON(w, r, &settings) {
		return
	}
	saved, err := s.dns.UpdateSettings(r.Context(), settings)
	if err != nil {
		s.writeDNSError(w, r, err)
		return
	}
	s.log.Info("DNS settings updated", "username", userFrom(r.Context()).Username)
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleGetDNSStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.dns.Status(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleListZones(w http.ResponseWriter, r *http.Request) {
	zones, err := s.dns.Zones(r.Context())
	if err != nil {
		s.writeDNSError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, zones)
}

func (s *Server) handleCreateZone(w http.ResponseWriter, r *http.Request) {
	var req dns.NewZone
	if !decodeJSON(w, r, &req) {
		return
	}
	zone, err := s.dns.CreateZone(r.Context(), req)
	if err != nil {
		s.writeDNSError(w, r, err)
		return
	}
	s.log.Info("DNS zone created", "zone", zone.Name, "username", userFrom(r.Context()).Username)
	writeJSON(w, http.StatusCreated, zone)
}

func (s *Server) handleGetZone(w http.ResponseWriter, r *http.Request) {
	zone, err := s.dns.Zone(r.Context(), r.PathValue("zone"))
	if err != nil {
		s.writeDNSError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, zone)
}

func (s *Server) handleDeleteZone(w http.ResponseWriter, r *http.Request) {
	if err := s.dns.DeleteZone(r.Context(), r.PathValue("zone")); err != nil {
		s.writeDNSError(w, r, err)
		return
	}
	s.log.Info("DNS zone deleted", "zone", r.PathValue("zone"), "username", userFrom(r.Context()).Username)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePutRRSet(w http.ResponseWriter, r *http.Request) {
	var rr dns.RRSet
	if !decodeJSON(w, r, &rr) {
		return
	}
	saved, err := s.dns.ReplaceRRSet(r.Context(), r.PathValue("zone"), rr)
	if err != nil {
		s.writeDNSError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleDeleteRRSet(w http.ResponseWriter, r *http.Request) {
	err := s.dns.DeleteRRSet(r.Context(), r.PathValue("zone"), r.PathValue("name"), r.PathValue("type"))
	if err != nil {
		s.writeDNSError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
