package httpapi

import (
	"context"
	"net/http"
	"sor.hosting/backend/internal/auth"
	"sor.hosting/backend/internal/database/sqlc"
	"sor.hosting/backend/internal/httpx"
	"strings"
	"time"
)

func (a *App) settings(w http.ResponseWriter, r *http.Request) {
	s, e := a.Q.Server(r.Context())
	if e != nil {
		fail(w, r, e)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	status, e := a.Agent.Status(ctx)
	httpx.JSON(w, 200, map[string]any{"server": s, "integrations": map[string]bool{"agent": e == nil && status.Connected, "prometheus": a.Cfg.PrometheusURL != ""}, "backup_destination": "Directorio local del agente"})
}

func (a *App) rename(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Name string `json:"server_name"`
	}
	if !httpx.Decode(w, r, &in) {
		return
	}
	if len(strings.TrimSpace(in.Name)) < 1 || len(in.Name) > 80 {
		invalid(w, r)
		return
	}
	tx, e := a.DB.BeginTx(r.Context(), nil)
	if e != nil {
		fail(w, r, e)
		return
	}
	defer tx.Rollback()
	q := a.Q.WithTx(tx)
	e = q.RenameServer(r.Context(), strings.TrimSpace(in.Name))
	u, _ := auth.CurrentUser(r.Context())
	if e == nil {
		e = q.InsertAudit(r.Context(), sqlc.InsertAuditParams{ID: auth.RandomID(), UserID: u.ID, Action: "settings.rename", ResourceID: "local", Outcome: "succeeded", CreatedAt: time.Now().UTC().Format(time.RFC3339)})
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		fail(w, r, e)
		return
	}
	s, e := a.Q.Server(r.Context())
	if e != nil {
		fail(w, r, e)
		return
	}
	httpx.JSON(w, 200, map[string]any{"server": s})
}
