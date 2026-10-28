package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"net/http"
	"net/url"
	"sor.hosting/backend/internal/agentclient"
	"sor.hosting/backend/internal/auth"
	"sor.hosting/backend/internal/config"
	"sor.hosting/backend/internal/database/sqlc"
	"sor.hosting/backend/internal/httpx"
	"sor.hosting/backend/internal/jobs"
	"sor.hosting/backend/internal/monitoring"
	"sor.hosting/shared/agentapi"
	"strconv"
	"strings"
	"time"
)

type App struct {
	DB      *sql.DB
	Q       *sqlc.Queries
	Cfg     config.Config
	Auth    *auth.Service
	Agent   agentapi.Client
	Jobs    *jobs.Service
	Monitor *monitoring.Client
	Health  *monitoring.Checker
}

func New(db *sql.DB, cfg config.Config) *App {
	q := sqlc.New(db)
	a := agentclient.New(cfg.AgentSocket)
	return &App{DB: db, Q: q, Cfg: cfg, Auth: auth.NewService(db, cfg), Agent: a, Jobs: &jobs.Service{DB: db, Q: q, Agent: a, Key: cfg.Key}, Monitor: monitoring.New(cfg.PrometheusURL, cfg.PrometheusInstance, cfg.Filesystem), Health: monitoring.NewChecker()}
}

func (a *App) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer, security)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { httpx.JSON(w, 200, map[string]string{"status": "ok"}) })
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if a.DB.PingContext(ctx) != nil {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
	})
	r.Route("/api/v1", func(r chi.Router) {
		auth.RegisterRoutes(r, a.Auth)
		r.Group(func(r chi.Router) {
			r.Use(a.Auth.RequireSession, a.Auth.RequireCSRF)
			r.Get("/sites", a.sites)
			r.Get("/sites/{id}", a.site)
			r.Post("/sites", a.install)
			r.Post("/sites/adopt", a.adopt)
			r.Post("/sites/{id}/backups", a.createBackup)
			r.Get("/containers", a.containers)
			r.Get("/containers/{id}/logs", a.logs)
			r.Post("/containers/{id}/actions", a.containerAction)
			r.Get("/domains", a.domains)
			r.Post("/domains", a.addDomain)
			r.Post("/domains/{id}/check", a.checkDomain)
			r.Get("/certificates", a.certificates)
			r.Post("/certificates/{id}/renew", a.renewCertificate)
			r.Get("/backups", a.backups)
			r.Get("/backups/{id}/download", a.download)
			r.Post("/backups/{id}/restore", a.restore)
			r.Get("/jobs/{id}", a.job)
			r.Get("/audit", a.audit)
			r.Get("/settings", a.settings)
			r.Patch("/settings", a.rename)
			r.Get("/dashboard", a.dashboard)
			r.Get("/monitoring", a.history)
		})
	})
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			httpx.Error(w, r, 404, "NOT_FOUND", "Ruta no encontrada.")
			return
		}
		a.static(w, r)
	})
	return r
}

func security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; object-src 'none'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

func fail(w http.ResponseWriter, r *http.Request, e error) {
	if errors.Is(e, sql.ErrNoRows) {
		httpx.Error(w, r, 404, "NOT_FOUND", "Recurso no encontrado.")
		return
	}
	httpx.Error(w, r, 503, "INTEGRATION_UNAVAILABLE", "No se pudo obtener la información del servidor.")
}

func invalid(w http.ResponseWriter, r *http.Request) {
	httpx.Error(w, r, 422, "VALIDATION_ERROR", "Revisa los campos y las confirmaciones.")
}

func page[T any](w http.ResponseWriter, r *http.Request, rows []T) {
	limit := 25
	if n, e := strconv.Atoi(r.URL.Query().Get("limit")); e == nil && n >= 1 && n <= 100 {
		limit = n
	}
	offset := 0
	if s := r.URL.Query().Get("cursor"); s != "" {
		n, e := strconv.Atoi(s)
		if e != nil || n < 0 {
			invalid(w, r)
			return
		}
		offset = n
	}
	if offset > len(rows) {
		offset = len(rows)
	}
	end := offset + limit
	var next any
	if end < len(rows) {
		next = strconv.Itoa(end)
	} else {
		end = len(rows)
	}
	items := rows[offset:end]
	if items == nil {
		items = []T{}
	}
	httpx.JSON(w, 200, map[string]any{"items": items, "next_cursor": next})
}

func (a *App) enqueue(w http.ResponseWriter, r *http.Request, kind, id string, payload any) {
	u, _ := auth.CurrentUser(r.Context())
	b, e := json.Marshal(payload)
	if e != nil {
		invalid(w, r)
		return
	}
	request := agentapi.OperationRequest{Kind: kind, ResourceID: id, Payload: b}
	existing, found, e := a.Jobs.Existing(r.Context(), u.ID, r.Header.Get("Idempotency-Key"), request)
	if e != nil {
		if errors.Is(e, jobs.ErrConflict) {
			httpx.Error(w, r, 409, "IDEMPOTENCY_CONFLICT", "La clave ya corresponde a otra solicitud.")
		} else {
			invalid(w, r)
		}
		return
	}
	if found {
		httpx.JSON(w, 202, map[string]any{"job": existing})
		return
	}
	var names struct {
		Domain   string `json:"domain"`
		Hostname string `json:"hostname"`
	}
	_ = json.Unmarshal(b, &names)
	if names.Domain != "" && a.protectedHost(names.Domain) || names.Hostname != "" && a.protectedHost(names.Hostname) {
		httpx.Error(w, r, 403, "PROTECTED_DOMAIN", "El dominio del panel está protegido.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	status, e := a.Agent.Status(ctx)
	if e != nil || !status.Connected || status.RecoveryRequired {
		httpx.Error(w, r, 503, "INTEGRATION_UNAVAILABLE", "El agente está desconectado o necesita recuperar una operación.")
		return
	}
	if kind == "site.install" && (!status.Docker || !status.InstallReady) {
		httpx.Error(w, r, 503, "INSTALLER_UNAVAILABLE", "El instalador no está disponible. Comprueba Docker y la configuración del servidor.")
		return
	}
	job, e := a.Jobs.Enqueue(r.Context(), u.ID, r.Header.Get("Idempotency-Key"), agentapi.OperationRequest{Kind: kind, ResourceID: id, Payload: b})
	if e != nil {
		switch {
		case errors.Is(e, jobs.ErrBusy):
			httpx.Error(w, r, 409, "RESOURCE_BUSY", "Hay una operación pendiente para este recurso.")
		case errors.Is(e, jobs.ErrConflict):
			httpx.Error(w, r, 409, "IDEMPOTENCY_CONFLICT", "La solicitud cambió. Vuelve a iniciar la acción.")
		default:
			invalid(w, r)
		}
		return
	}
	httpx.JSON(w, 202, map[string]any{"job": job})
}

func (a *App) protectedHost(host string) bool {
	u, e := url.Parse(a.Cfg.Origin)
	return e != nil || strings.EqualFold(strings.TrimSuffix(host, "."), u.Hostname())
}

func (a *App) recoverJob(w http.ResponseWriter, r *http.Request, kind, id string, payload any) bool {
	u, _ := auth.CurrentUser(r.Context())
	b, e := json.Marshal(payload)
	if e != nil {
		invalid(w, r)
		return true
	}
	job, found, e := a.Jobs.Existing(r.Context(), u.ID, r.Header.Get("Idempotency-Key"), agentapi.OperationRequest{Kind: kind, ResourceID: id, Payload: b})
	if e != nil {
		if errors.Is(e, jobs.ErrConflict) {
			httpx.Error(w, r, 409, "IDEMPOTENCY_CONFLICT", "La clave ya corresponde a otra solicitud.")
		} else {
			invalid(w, r)
		}
		return true
	}
	if found {
		httpx.JSON(w, 202, map[string]any{"job": job})
		return true
	}
	return false
}
