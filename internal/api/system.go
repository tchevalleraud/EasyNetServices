package api

import (
	"net/http"

	"github.com/tchevalleraud/EasyNetServices/internal/system"
)

func (s *Server) handleGetSystemSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.system.Settings(r.Context())
	if err != nil {
		s.internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handlePutSystemSettings(w http.ResponseWriter, r *http.Request) {
	var settings system.Settings
	if !decodeJSON(w, r, &settings) {
		return
	}
	saved, err := s.system.Update(r.Context(), settings)
	if err != nil {
		s.writeUpdateError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) handleGetSystemTime(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.system.Now())
}
