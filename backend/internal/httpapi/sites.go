package httpapi

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
	"sor.hosting/backend/internal/database/sqlc"
	"sor.hosting/backend/internal/httpx"
	"sor.hosting/backend/internal/sites"
	"sor.hosting/shared/agentapi"
	"strings"
)

func (a *App) sites(w http.ResponseWriter, r *http.Request) {
	rows, e := a.Q.ListSites(r.Context())
	if e != nil {
		fail(w, r, e)
		return
	}
	out := []siteView{}
	q := strings.ToLower(r.URL.Query().Get("q"))
	state := r.URL.Query().Get("status")
	for _, s := range rows {
		if strings.Contains(strings.ToLower(s.Name+" "+s.Domain), q) && (state == "" || s.State == state) {
			out = append(out, a.view(s))
		}
	}
	page(w, r, out)
}

func (a *App) site(w http.ResponseWriter, r *http.Request) {
	s, e := a.Q.Site(r.Context(), chi.URLParam(r, "id"))
	if e != nil {
		fail(w, r, e)
		return
	}
	containers := []agentapi.Container{}
	all, _ := a.Agent.Containers(r.Context())
	var meta agentapi.SiteResources
	_ = json.Unmarshal([]byte(s.Metadata), &meta)
	for _, c := range all {
		if s.Management == "external" && (c.ID == meta.WebID || c.ID == meta.DBID) {
			c.CanManage = false
			c.CanReadLogs = false
			c.Managed = false
			containers = append(containers, c)
			continue
		}
		if c.SiteID == s.ID {
			containers = append(containers, c)
		}
	}
	domains, e := a.Q.ListDomains(r.Context())
	if e != nil {
		fail(w, r, e)
		return
	}
	ds := []sqlc.Domain{}
	for _, d := range domains {
		if d.SiteID == s.ID {
			ds = append(ds, d)
		}
	}
	backups, e := a.Q.ListBackups(r.Context())
	if e != nil {
		fail(w, r, e)
		return
	}
	bs := []sqlc.Backup{}
	for _, b := range backups {
		if b.SiteID == s.ID {
			bs = append(bs, b)
		}
	}
	httpx.JSON(w, 200, map[string]any{"site": a.view(s), "containers": containers, "domains": ds, "backups": bs})
}

func (a *App) install(w http.ResponseWriter, r *http.Request) {
	var in agentapi.SiteInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	if sites.ValidateInstall(&in) != nil {
		invalid(w, r)
		return
	}
	a.enqueue(w, r, "site.install", "", in)
}

func (a *App) adopt(w http.ResponseWriter, r *http.Request) {
	var in agentapi.AdoptInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	if sites.ValidateAdoption(&in) != nil {
		invalid(w, r)
		return
	}
	a.enqueue(w, r, "site.adopt", "", in)
}
