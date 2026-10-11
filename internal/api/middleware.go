package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/tchevalleraud/EasyNetServices/internal/auth"
	"github.com/tchevalleraud/EasyNetServices/internal/store"
)

const sessionCookie = "ens_session"

type ctxKey int

const (
	ctxUser ctxKey = iota
	ctxToken
)

func userFrom(ctx context.Context) *store.User {
	u, _ := ctx.Value(ctxUser).(*store.User)
	return u
}

func tokenFrom(ctx context.Context) string {
	t, _ := ctx.Value(ctxToken).(string)
	return t
}

// requireAuth rejects requests without a valid session. Unless allowPending is
// set, users who still have to change their password are rejected as well.
func (s *Server) requireAuth(next http.HandlerFunc, allowPending bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil || cookie.Value == "" {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
			return
		}
		user, err := s.auth.Authenticate(r.Context(), cookie.Value)
		if errors.Is(err, auth.ErrInvalidSession) {
			s.clearSessionCookie(w)
			writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
			return
		}
		if err != nil {
			s.internalError(w, r, err)
			return
		}
		if user.MustChangePassword && !allowPending {
			writeError(w, http.StatusForbidden, "password_change_required", "the password must be changed first")
			return
		}
		ctx := context.WithValue(r.Context(), ctxUser, user)
		ctx = context.WithValue(ctx, ctxToken, cookie.Value)
		next(w, r.WithContext(ctx))
	})
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   s.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
}

func (s *Server) internalError(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

// checkOrigin rejects cross-origin state-changing requests, as a defense in depth
// on top of SameSite cookies.
func checkOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || !strings.EqualFold(u.Host, r.Host) {
					writeError(w, http.StatusForbidden, "invalid_origin", "cross-origin request rejected")
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Content-Security-Policy",
			"default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.Debug("http request",
			"method", r.Method, "path", r.URL.Path, "status", rec.status,
			"duration", time.Since(start), "remote", r.RemoteAddr)
	})
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				s.log.Error("panic in handler", "method", r.Method, "path", r.URL.Path, "panic", v)
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// rateLimiter counts failed attempts per client IP over a fixed window.
type rateLimiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	entries map[string]*rateEntry
}

type rateEntry struct {
	count int
	reset time.Time
}

func newRateLimiter(max int, window time.Duration) *rateLimiter {
	return &rateLimiter{max: max, window: window, entries: map[string]*rateEntry{}}
}

// blocked reports whether key has exhausted its attempts in the current window.
func (l *rateLimiter) blocked(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[key]
	if !ok {
		return false
	}
	if time.Now().After(e.reset) {
		delete(l.entries, key)
		return false
	}
	return e.count >= l.max
}

// fail records a failed attempt for key.
func (l *rateLimiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	// Drop stale entries opportunistically to bound memory usage.
	for k, e := range l.entries {
		if now.After(e.reset) {
			delete(l.entries, k)
		}
	}
	e, ok := l.entries[key]
	if !ok {
		e = &rateEntry{reset: now.Add(l.window)}
		l.entries[key] = e
	}
	e.count++
}

// reset clears the attempts of key.
func (l *rateLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
