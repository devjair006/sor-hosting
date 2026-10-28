package httpapi

import (
	"github.com/go-chi/chi/v5"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sor.hosting/backend/internal/auth"
	"sor.hosting/backend/internal/database/sqlc"
	"strings"
	"time"
)

func (a *App) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		http.NotFound(w, r)
		return
	}
	for _, part := range strings.Split(r.URL.Path, "/") {
		if part == ".." || strings.HasPrefix(part, ".") || strings.Contains(part, "\\") {
			http.NotFound(w, r)
			return
		}
	}
	path := filepath.Join(a.Cfg.StaticDir, filepath.FromSlash(strings.TrimPrefix(r.URL.Path, "/")))
	st, e := os.Stat(path)
	if e == nil && st.IsDir() {
		e = os.ErrNotExist
	}
	if e != nil {
		if filepath.Ext(r.URL.Path) != "" {
			http.NotFound(w, r)
			return
		}
		path = filepath.Join(a.Cfg.StaticDir, "index.html")
	}
	if _, e = os.Stat(path); e != nil {
		http.Error(w, "Frontend no compilado. Ejecuta npm run build en frontend.", 503)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	if strings.Contains(r.URL.Path, "/assets/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	http.ServeFile(w, r, path)
}
func (a *App) download(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	b, e := a.Q.Backup(r.Context(), id)
	if e != nil {
		fail(w, r, e)
		return
	}
	reader, file, e := a.Agent.Download(r.Context(), b.ID)
	if e != nil {
		fail(w, r, e)
		return
	}
	defer reader.Close()
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": file.Name}))
	w.Header().Set("Cache-Control", "no-store")
	u, _ := auth.CurrentUser(r.Context())
	if e = a.Q.InsertAudit(r.Context(), sqlc.InsertAuditParams{ID: auth.RandomID(), UserID: u.ID, Action: "backup.download", ResourceID: id, Outcome: "started", CreatedAt: time.Now().UTC().Format(time.RFC3339)}); e != nil {
		fail(w, r, e)
		return
	}
	w.WriteHeader(200)
	_, _ = io.Copy(w, reader)
}
