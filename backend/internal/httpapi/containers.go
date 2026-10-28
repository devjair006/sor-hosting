package httpapi

import (
	"github.com/go-chi/chi/v5"
	"net/http"
	"sor.hosting/backend/internal/containers"
	"sor.hosting/backend/internal/httpx"
	"sor.hosting/shared/agentapi"
	"strconv"
	"strings"
)

func (a *App) allowedContainer(r *http.Request, id string, logs bool) (bool, error) {
	all, e := a.Agent.Containers(r.Context())
	if e != nil {
		return false, e
	}
	for _, c := range all {
		if c.ID == id && containers.CanUse(c, logs) {
			_, e = a.Q.Site(r.Context(), c.SiteID)
			return e == nil, e
		}
	}
	return false, nil
}

func (a *App) containers(w http.ResponseWriter, r *http.Request) {
	rows, e := a.Agent.Containers(r.Context())
	if e != nil {
		fail(w, r, e)
		return
	}
	for i := range rows {
		if rows[i].SiteID != "" {
			if _, e = a.Q.Site(r.Context(), rows[i].SiteID); e != nil {
				rows[i].CanManage = false
				rows[i].CanReadLogs = false
			}
		}
	}
	filtered := []agentapi.Container{}
	term := strings.ToLower(r.URL.Query().Get("q"))
	for _, c := range rows {
		if strings.Contains(strings.ToLower(c.Name+" "+c.Image), term) {
			filtered = append(filtered, c)
		}
	}
	page(w, r, filtered)
}

func (a *App) containerAction(w http.ResponseWriter, r *http.Request) {
	var in agentapi.ContainerInput
	if !httpx.Decode(w, r, &in) {
		return
	}
	if in.Action != "start" && in.Action != "stop" && in.Action != "restart" {
		invalid(w, r)
		return
	}
	id := chi.URLParam(r, "id")
	if a.recoverJob(w, r, "container."+in.Action, id, in) {
		return
	}
	ok, e := a.allowedContainer(r, id, false)
	if e != nil {
		fail(w, r, e)
		return
	}
	if !ok {
		httpx.Error(w, r, 403, "FORBIDDEN", "Contenedor no autorizado o protegido.")
		return
	}
	a.enqueue(w, r, "container."+in.Action, id, in)
}

func (a *App) logs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ok, e := a.allowedContainer(r, id, true)
	if e != nil {
		fail(w, r, e)
		return
	}
	if !ok {
		httpx.Error(w, r, 403, "FORBIDDEN", "No tienes acceso a estos logs.")
		return
	}
	tail := 200
	if value := r.URL.Query().Get("tail"); value != "" {
		tail, e = strconv.Atoi(value)
		if e != nil || tail < 1 || tail > 1000 {
			invalid(w, r)
			return
		}
	}
	out, e := a.Agent.Logs(r.Context(), agentapi.LogsRequest{ContainerID: id, Tail: tail})
	if e != nil {
		fail(w, r, e)
		return
	}
	httpx.JSON(w, 200, out)
}
