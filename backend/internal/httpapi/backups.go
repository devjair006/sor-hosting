package httpapi

import (
	"github.com/go-chi/chi/v5"
	"net/http"
	"sor.hosting/backend/internal/httpx"
	"sor.hosting/shared/agentapi"
)

func (a *App) backups(w http.ResponseWriter, r *http.Request) {
	rows, e := a.Q.ListBackups(r.Context())
	if e != nil {
		fail(w, r, e)
		return
	}
	page(w, r, rows)
}

func (a *App) createBackup(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Confirm bool `json:"confirm_maintenance"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	id := chi.URLParam(r, "id")
	if !in.Confirm {
		invalid(w, r)
		return
	}
	if _, e := a.Q.Site(r.Context(), id); e != nil {
		fail(w, r, e)
		return
	}
	if !a.mutableSite(w, r, id) {
		return
	}
	a.enqueue(w, r, "backup.create", id, agentapi.BackupInput{ConfirmMaintenance: true, SiteID: id})
}

func (a *App) restore(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Confirm bool   `json:"confirm_maintenance"`
		Domain  string `json:"confirm_domain"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	b, e := a.Q.Backup(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		fail(w, r, e)
		return
	}
	s, e := a.Q.Site(r.Context(), b.SiteID)
	if e != nil {
		fail(w, r, e)
		return
	}
	if !a.mutableSite(w, r, s.ID) {
		return
	}
	if !in.Confirm || in.Domain != s.Domain {
		invalid(w, r)
		return
	}
	a.enqueue(w, r, "backup.restore", b.ID, agentapi.BackupInput{ConfirmMaintenance: true, ConfirmDomain: in.Domain, SiteID: s.ID, BackupID: b.ID})
}
