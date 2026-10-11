package api

import (
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/tchevalleraud/EasyNetServices/internal/auth"
	"github.com/tchevalleraud/EasyNetServices/internal/store"
	"github.com/tchevalleraud/EasyNetServices/internal/version"
)

type userResponse struct {
	Username           string `json:"username"`
	Source             string `json:"source"`
	MustChangePassword bool   `json:"mustChangePassword"`
}

func toUserResponse(u *store.User) userResponse {
	return userResponse{Username: u.Username, Source: u.Source, MustChangePassword: u.MustChangePassword}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "unhealthy", "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type versionResponse struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, versionResponse{Version: version.Version, Commit: version.Commit, Date: version.Date})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.loginLimiter.blocked(ip) {
		writeError(w, http.StatusTooManyRequests, "too_many_attempts", "too many failed attempts, try again later")
		return
	}

	var req loginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "username and password are required")
		return
	}

	token, user, err := s.auth.Login(r.Context(), req.Username, req.Password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		s.loginLimiter.fail(ip)
		s.log.Warn("failed login attempt", "username", req.Username, "remote", ip)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid username or password")
		return
	}
	if err != nil {
		s.internalError(w, r, err)
		return
	}

	s.loginLimiter.reset(ip)
	s.log.Info("user logged in", "username", user.Username, "remote", ip)
	s.setSessionCookie(w, token)
	writeJSON(w, http.StatusOK, toUserResponse(user))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.auth.Logout(r.Context(), tokenFrom(r.Context())); err != nil {
		s.internalError(w, r, err)
		return
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, toUserResponse(userFrom(r.Context())))
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req changePasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	user := userFrom(r.Context())
	err := s.auth.ChangePassword(r.Context(), user, tokenFrom(r.Context()), req.CurrentPassword, req.NewPassword)
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusBadRequest, "invalid_current_password", "current password is incorrect")
	case errors.Is(err, auth.ErrWeakPassword):
		writeError(w, http.StatusBadRequest, "weak_password", err.Error())
	case errors.Is(err, auth.ErrNotLocalUser):
		writeError(w, http.StatusBadRequest, "not_local_user", err.Error())
	case err != nil:
		s.internalError(w, r, err)
	default:
		s.log.Info("password changed", "username", user.Username)
		w.WriteHeader(http.StatusNoContent)
	}
}

type systemResponse struct {
	Version  string `json:"version"`
	Role     string `json:"role"`
	Hostname string `json:"hostname"`
	Uptime   int64  `json:"uptimeSeconds"`
}

func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	writeJSON(w, http.StatusOK, systemResponse{
		Version:  version.Version,
		Role:     string(s.cfg.Role),
		Hostname: hostname,
		Uptime:   int64(time.Since(s.startedAt).Seconds()),
	})
}
