package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sor.hosting/backend/internal/config"
	"sor.hosting/backend/internal/database"
	"sor.hosting/backend/internal/database/sqlc"
	"testing"
	"time"
)

type harness struct {
	service *Service
	router  http.Handler
	now     time.Time
}

func authHarness(t *testing.T) *harness {
	t.Helper()
	db, e := database.Open(context.Background(), filepath.Join(t.TempDir(), "auth.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	if e = database.Migrate(context.Background(), db); e != nil {
		t.Fatal(e)
	}
	hash, e := HashPassword("correct-test-password")
	if e != nil {
		t.Fatal(e)
	}
	q := sqlc.New(db)
	e = q.CreateUser(context.Background(), sqlc.CreateUserParams{ID: "admin-test-123", Email: "admin@example.com", PasswordHash: hash, Role: "admin", CreatedAt: "2026-10-27T00:00:00Z"})
	if e != nil {
		t.Fatal(e)
	}
	s := NewService(db, config.Config{Mode: "production", Origin: "https://panel.example.com", Key: bytes.Repeat([]byte{1}, 32)})
	h := &harness{service: s, now: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
	s.Clock = func() time.Time { return h.now }
	r := chi.NewRouter()
	RegisterRoutes(r, s)
	h.router = r
	return h
}
func (h *harness) request(method, path, body string, cookie *http.Cookie, csrf, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	r.RemoteAddr = "192.0.2.10:1234"
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if csrf != "" {
		r.Header.Set("X-CSRF-Token", csrf)
	}
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	h.router.ServeHTTP(w, r)
	return w
}
func (h *harness) login(t *testing.T) *http.Cookie {
	t.Helper()
	w := h.request("POST", "/auth/login", `{"email":"admin@example.com","password":"correct-test-password"}`, nil, "", "https://panel.example.com")
	if w.Code != 200 {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing session cookie")
	}
	return cookies[0]
}
func (h *harness) csrf(t *testing.T, c *http.Cookie) string {
	t.Helper()
	w := h.request("GET", "/auth/csrf", "", c, "", "")
	var v struct {
		Token string `json:"csrf_token"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &v) != nil || v.Token == "" {
		t.Fatal("CSRF token unavailable")
	}
	return v.Token
}
func TestCookieFlagsAndHashedStorage(t *testing.T) {
	h := authHarness(t)
	c := h.login(t)
	if c.Name != "__Host-sor_session" || !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteStrictMode || c.Path != "/" || c.Domain != "" {
		t.Fatalf("unsafe session cookie: %+v", c)
	}
	var stored string
	if e := h.service.DB.QueryRow("SELECT token_hash FROM sessions").Scan(&stored); e != nil {
		t.Fatal(e)
	}
	if stored == c.Value {
		t.Fatal("raw session token stored in database")
	}
}
func TestSessionIdleExpiry(t *testing.T) {
	h := authHarness(t)
	c := h.login(t)
	h.now = h.now.Add(30*time.Minute + time.Second)
	if w := h.request("GET", "/auth/me", "", c, "", ""); w.Code != 401 {
		t.Fatalf("idle session accepted: %d", w.Code)
	}
}
func TestAbsoluteExpiryDespiteActivity(t *testing.T) {
	h := authHarness(t)
	c := h.login(t)
	for i := 0; i < 35; i++ {
		h.now = h.now.Add(20 * time.Minute)
		if w := h.request("GET", "/auth/me", "", c, "", ""); w.Code != 200 {
			t.Fatalf("active session expired prematurely: %d", w.Code)
		}
	}
	h.now = h.now.Add(20*time.Minute + time.Second)
	if w := h.request("GET", "/auth/me", "", c, "", ""); w.Code != 401 {
		t.Fatalf("absolute session expiry bypassed: %d", w.Code)
	}
}
func TestLogoutRequiresCSRFAndInvalidatesSession(t *testing.T) {
	h := authHarness(t)
	c := h.login(t)
	token := h.csrf(t, c)
	for _, input := range []struct{ csrf, origin string }{{"", "https://panel.example.com"}, {token, "https://foreign.example"}} {
		if w := h.request("POST", "/auth/logout", "", c, input.csrf, input.origin); w.Code != 403 {
			t.Fatalf("CSRF bypass: %d", w.Code)
		}
	}
	if w := h.request("POST", "/auth/logout", "", c, token, "https://panel.example.com"); w.Code != 204 {
		t.Fatalf("logout failed: %d", w.Code)
	}
	if w := h.request("GET", "/auth/me", "", c, "", ""); w.Code != 401 {
		t.Fatal("old cookie still grants access")
	}
}
func TestPasswordChangeRevokesAllSessions(t *testing.T) {
	h := authHarness(t)
	first := h.login(t)
	second := h.login(t)
	token := h.csrf(t, first)
	w := h.request("POST", "/auth/password", `{"current_password":"correct-test-password","new_password":"another-test-password"}`, first, token, "https://panel.example.com")
	if w.Code != 204 {
		t.Fatalf("password change: %d %s", w.Code, w.Body.String())
	}
	for _, c := range []*http.Cookie{first, second} {
		if h.request("GET", "/auth/me", "", c, "", "").Code != 401 {
			t.Fatal("existing session survived password change")
		}
	}
}
