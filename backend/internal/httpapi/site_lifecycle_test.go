package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sor.hosting/backend/internal/auth"
	"sor.hosting/backend/internal/config"
	"sor.hosting/backend/internal/database"
	"sor.hosting/backend/internal/database/sqlc"
	"sor.hosting/shared/agentapi"
	"strings"
	"testing"
)

type lifecycleAgent struct {
	offline    bool
	containers []agentapi.Container
}

func (s *lifecycleAgent) Status(context.Context) (agentapi.Status, error) {
	if s.offline {
		return agentapi.Status{}, errors.New("offline")
	}
	return agentapi.Status{Connected: true, Docker: true, InstallReady: true, SiteLifecycle: true}, nil
}
func (s *lifecycleAgent) Containers(context.Context) ([]agentapi.Container, error) {
	return s.containers, nil
}
func (*lifecycleAgent) Logs(context.Context, agentapi.LogsRequest) (agentapi.LogsResult, error) {
	return agentapi.LogsResult{}, nil
}
func (*lifecycleAgent) Submit(context.Context, agentapi.OperationRequest) (agentapi.Operation, error) {
	return agentapi.Operation{}, errors.New("no worker in this test")
}
func (*lifecycleAgent) Operation(context.Context, string) (agentapi.Operation, error) {
	return agentapi.Operation{}, errors.New("no worker")
}
func (*lifecycleAgent) Download(context.Context, string) (io.ReadCloser, agentapi.BackupFile, error) {
	return nil, agentapi.BackupFile{}, errors.New("no files")
}
func TestSiteLifecycleHTTPGuardsAndIdempotency(t *testing.T) {
	ctx := context.Background()
	db, e := database.Open(ctx, filepath.Join(t.TempDir(), "panel.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = database.Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	cfg := config.Config{Mode: "development", Origin: "http://panel.test", Key: bytes.Repeat([]byte{1}, 32)}
	app := New(db, cfg)
	spy := &lifecycleAgent{}
	app.Agent = spy
	hash, e := auth.HashPassword("test-password-1234")
	if e != nil {
		t.Fatal(e)
	}
	if e = app.Q.CreateUser(ctx, sqlc.CreateUserParams{ID: "admin123456", Email: "admin@example.com", PasswordHash: hash, Role: "admin", CreatedAt: "now"}); e != nil {
		t.Fatal(e)
	}
	handler := app.Router()
	var cookie *http.Cookie
	csrf := ""
	request := func(path, body, key string, authenticated bool) *httptest.ResponseRecorder {
		method := "POST"
		if body == "" {
			method = "GET"
		}
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Origin", cfg.Origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", key)
		if authenticated && cookie != nil {
			r.AddCookie(cookie)
			r.Header.Set("X-CSRF-Token", csrf)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	login := request("/api/v1/auth/login", `{"email":"admin@example.com","password":"test-password-1234"}`, "", false)
	if login.Code != 200 {
		t.Fatal(login.Code, login.Body.String())
	}
	cookie = login.Result().Cookies()[0]
	cr := request("/api/v1/auth/csrf", "", "", true)
	var c struct {
		Token string `json:"csrf_token"`
	}
	json.Unmarshal(cr.Body.Bytes(), &c)
	csrf = c.Token
	for _, site := range []struct{ id, mode string }{{"managed123", "managed"}, {"external123", "external"}} {
		if e = app.Q.CreateSite(ctx, sqlc.CreateSiteParams{ID: site.id, ServerID: "local", Name: site.id, Domain: site.id + ".example", State: "active", Metadata: `{}`, CreatedAt: "now", UpdatedAt: "now"}); e != nil {
			t.Fatal(e)
		}
		if e = app.Q.SetSiteManagement(ctx, sqlc.SetSiteManagementParams{ID: site.id, Management: site.mode, Lifecycle: "present"}); e != nil {
			t.Fatal(e)
		}
	}
	for _, tc := range []struct {
		path, body string
		auth       bool
		status     int
	}{
		{"/api/v1/sites/managed123/delete", `{"confirm_domain":"managed123.example","keep_backups":true}`, false, 401},
		{"/api/v1/sites/managed123/delete", `{"confirm_domain":"managed123","keep_backups":true}`, true, 422},
		{"/api/v1/sites/external123/delete", `{"confirm_domain":"external123.example","keep_backups":true}`, true, 403},
		{"/api/v1/sites/external123/backups", `{"confirm_maintenance":true}`, true, 403},
		{"/api/v1/domains", `{"site_id":"external123","hostname":"alias.example"}`, true, 403},
	} {
		w := request(tc.path, tc.body, "guard-key12345", tc.auth)
		if w.Code != tc.status {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
	key := "delete-key-12345"
	body := `{"confirm_domain":"managed123.example","keep_backups":true}`
	first := request("/api/v1/sites/managed123/delete", body, key, true)
	if first.Code != 202 {
		t.Fatal(first.Code, first.Body.String())
	}
	spy.offline = true
	again := request("/api/v1/sites/managed123/delete", body, key, true)
	if again.Code != 202 || again.Body.String() != first.Body.String() {
		t.Fatal("retry must return original job even offline", again.Code, again.Body.String())
	}
	site, _ := app.Q.Site(ctx, "managed123")
	if site.Lifecycle != "deleting" {
		t.Fatal(site)
	}
	jobs, _ := app.Q.PendingJobs(ctx)
	if len(jobs) != 1 {
		t.Fatal("duplicate job", len(jobs))
	}
	// Read-only registration accepts protected domains; it never requests infrastructure writes.
	spy.offline = false
	external := request("/api/v1/sites/external", `{"name":"Current WP","domain":"panel.test","web_container_id":"","database_container_id":""}`, "external-key-123", true)
	if external.Code != 202 {
		t.Fatal(external.Code, external.Body.String())
	}
}
