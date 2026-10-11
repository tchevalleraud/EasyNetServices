package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/tchevalleraud/EasyNetServices/internal/auth"
	"github.com/tchevalleraud/EasyNetServices/internal/config"
	"github.com/tchevalleraud/EasyNetServices/internal/store"
)

type testClient struct {
	t      *testing.T
	srv    *httptest.Server
	client *http.Client
}

func newTestClient(t *testing.T) *testClient {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	local, err := auth.NewLocalProvider(st)
	if err != nil {
		t.Fatal(err)
	}
	authSvc := auth.NewService(st, time.Hour, local)
	if _, err := authSvc.EnsureDefaultAdmin(ctx); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{Role: config.RoleStandalone, TLSMode: config.TLSOff}
	ui := fstest.MapFS{
		"index.html":    {Data: []byte("<html>app</html>")},
		"assets/app.js": {Data: []byte("console.log('app')")},
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewServer(NewServer(cfg, st, authSvc, ui, log).Handler())
	t.Cleanup(srv.Close)

	jar, _ := cookiejar.New(nil)
	return &testClient{t: t, srv: srv, client: &http.Client{Jar: jar}}
}

func (c *testClient) do(method, path, body string, headers ...string) (int, map[string]any) {
	c.t.Helper()
	req, err := http.NewRequest(method, c.srv.URL+path, strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := c.client.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func errorCode(body map[string]any) string {
	e, _ := body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func TestAuthFlow(t *testing.T) {
	c := newTestClient(t)

	if status, _ := c.do("GET", "/api/v1/auth/me", ""); status != http.StatusUnauthorized {
		t.Fatalf("expected 401 without session, got %d", status)
	}

	status, body := c.do("POST", "/api/v1/auth/login", `{"username":"admin","password":"wrong"}`)
	if status != http.StatusUnauthorized || errorCode(body) != "invalid_credentials" {
		t.Fatalf("expected invalid_credentials, got %d %v", status, body)
	}

	status, body = c.do("POST", "/api/v1/auth/login", `{"username":"admin","password":"password"}`)
	if status != http.StatusOK || body["mustChangePassword"] != true {
		t.Fatalf("expected login with forced password change, got %d %v", status, body)
	}

	status, body = c.do("GET", "/api/v1/system", "")
	if status != http.StatusForbidden || errorCode(body) != "password_change_required" {
		t.Fatalf("expected password_change_required, got %d %v", status, body)
	}

	status, body = c.do("POST", "/api/v1/auth/password", `{"currentPassword":"password","newPassword":"password"}`)
	if status != http.StatusBadRequest || errorCode(body) != "weak_password" {
		t.Fatalf("expected weak_password, got %d %v", status, body)
	}

	if status, _ = c.do("POST", "/api/v1/auth/password", `{"currentPassword":"password","newPassword":"n3w-password"}`); status != http.StatusNoContent {
		t.Fatalf("expected password change to succeed, got %d", status)
	}

	status, body = c.do("GET", "/api/v1/system", "")
	if status != http.StatusOK || body["role"] != "standalone" {
		t.Fatalf("expected system info, got %d %v", status, body)
	}

	if status, _ = c.do("POST", "/api/v1/auth/logout", ""); status != http.StatusNoContent {
		t.Fatalf("expected logout to succeed, got %d", status)
	}
	if status, _ = c.do("GET", "/api/v1/auth/me", ""); status != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", status)
	}
}

func TestCrossOriginRejected(t *testing.T) {
	c := newTestClient(t)
	status, body := c.do("POST", "/api/v1/auth/login", `{"username":"admin","password":"password"}`,
		"Origin", "https://evil.example")
	if status != http.StatusForbidden || errorCode(body) != "invalid_origin" {
		t.Fatalf("expected invalid_origin, got %d %v", status, body)
	}
}

func TestLoginRateLimit(t *testing.T) {
	c := newTestClient(t)
	for range 10 {
		c.do("POST", "/api/v1/auth/login", `{"username":"admin","password":"wrong"}`)
	}
	status, body := c.do("POST", "/api/v1/auth/login", `{"username":"admin","password":"password"}`)
	if status != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after too many failures, got %d %v", status, body)
	}
}

func TestUIAndUnknownAPI(t *testing.T) {
	c := newTestClient(t)

	if status, body := c.do("GET", "/api/v1/nope", ""); status != http.StatusNotFound || errorCode(body) != "not_found" {
		t.Fatalf("expected JSON 404 for unknown API route, got %d %v", status, body)
	}

	for _, path := range []string{"/", "/dashboard", "/assets/app.js"} {
		resp, err := c.client.Get(c.srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", path, resp.StatusCode)
		}
		if path != "/assets/app.js" && string(data) != "<html>app</html>" {
			t.Fatalf("%s: expected SPA fallback, got %q", path, data)
		}
	}
}
