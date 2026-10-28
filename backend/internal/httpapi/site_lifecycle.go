package httpapi

import (
	"github.com/go-chi/chi/v5"
	"net/http"
	"sor.hosting/backend/internal/httpx"
	"sor.hosting/shared/agentapi"
	"sor.hosting/shared/domainname"
	"strings"
)

func (a *App) registerExternal(w http.ResponseWriter, r *http.Request) {
	var in agentapi.ExternalInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	host, e := domainname.Normalize(in.Domain)
	if e != nil || len(strings.TrimSpace(in.Name)) < 1 || len(in.Name) > 80 {
		invalid(w, r)
		return
	}
	in.Domain = host
	for _, id := range []string{in.WebContainerID, in.DatabaseContainerID} {
		if id != "" && !agentapi.ContainerID.MatchString(id) {
			invalid(w, r)
			return
		}
	}
	a.enqueue(w, r, "site.external", "", in)
}
func (a *App) deleteSite(w http.ResponseWriter, r *http.Request) {
	var in agentapi.DeleteInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	id := chi.URLParam(r, "id")
	if a.recoverJob(w, r, "site.delete", id, in) {
		return
	}
	site, e := a.Q.Site(r.Context(), id)
	if e != nil {
		fail(w, r, e)
		return
	}
	if site.Management == "external" || site.Lifecycle != "present" {
		httpx.Error(w, r, 403, "READ_ONLY_SITE", "Este sitio es de solo lectura o está en eliminación.")
		return
	}
	if in.ConfirmDomain != site.Domain {
		invalid(w, r)
		return
	}
	a.enqueue(w, r, "site.delete", id, in)
}
func (a *App) mutableSite(w http.ResponseWriter, r *http.Request, id string) bool {
	s, e := a.Q.Site(r.Context(), id)
	if e != nil {
		fail(w, r, e)
		return false
	}
	if s.Management == "external" || s.Lifecycle != "present" {
		httpx.Error(w, r, 403, "READ_ONLY_SITE", "No se permiten cambios sobre este sitio.")
		return false
	}
	return true
}
