package api

import (
	"errors"
	"net/http"

	"github.com/tchevalleraud/EasyNetServices/internal/ntp"
)

func (s *Server) handleGetNTPConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.ntp.Config(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) handlePutNTPConfig(w http.ResponseWriter, r *http.Request) {
	var cfg ntp.Config
	if !decodeJSON(w, r, &cfg) {
		return
	}
	saved, err := s.ntp.Update(r.Context(), cfg)
	if errors.Is(err, ntp.ErrApply) {
		writeError(w, http.StatusInternalServerError, "apply_failed", err.Error())
		return
	}
	if err != nil {
		s.writeUpdateError(w, r, err)
		return
	}
	s.log.Info("NTP configuration updated", "username", userFrom(r.Context()).Username)
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleGetNTPStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.ntp.Status(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}
