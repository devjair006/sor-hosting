package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/moby/moby/client"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sor.hosting/agent/internal/config"
	"sor.hosting/agent/internal/operations"
	"sor.hosting/shared/agentapi"
	"strconv"
	"syscall"
	"time"
)

func response(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func Server(cfg config.Config, d *operations.Dispatcher) (*http.Server, net.Listener, error) {
	dir := filepath.Dir(cfg.Socket)
	if e := os.MkdirAll(dir, 0750); e != nil {
		return nil, nil, e
	}
	if resolved, e := filepath.EvalSymlinks(dir); e != nil || resolved != dir {
		return nil, nil, errors.New("socket directory must not contain symlinks")
	}
	if e := os.Chown(dir, 0, int(cfg.PeerGID)); e != nil {
		return nil, nil, e
	}
	if e := os.Chmod(dir, 0750); e != nil {
		return nil, nil, e
	}
	if st, e := os.Lstat(cfg.Socket); e == nil {
		if st.Mode()&os.ModeSocket == 0 {
			return nil, nil, errors.New("socket path occupied")
		}
		c, e := net.DialTimeout("unix", cfg.Socket, 200*time.Millisecond)
		if e == nil {
			c.Close()
			return nil, nil, errors.New("agent already running")
		}
		if !errors.Is(e, syscall.ECONNREFUSED) && !errors.Is(e, syscall.ENOENT) {
			return nil, nil, e
		}
		if e = os.Remove(cfg.Socket); e != nil && !os.IsNotExist(e) {
			return nil, nil, e
		}
	}
	l, e := net.Listen("unix", cfg.Socket)
	if e != nil {
		return nil, nil, e
	}
	if e = os.Chown(cfg.Socket, 0, int(cfg.PeerGID)); e == nil {
		e = os.Chmod(cfg.Socket, 0660)
	}
	if e != nil {
		l.Close()
		return nil, nil, e
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		_, e := d.Docker.API.Ping(ctx, client.PingOptions{})
		nginxReady := d.WP.Nginx.Available(ctx)
		certReady := d.WP.Certbot.Available(ctx)
		recoveries, recoveryErr := d.Store.Q.Recoveries(ctx)
		markers, markerErr := filepath.Glob(filepath.Join(cfg.NginxDir, ".sor-*.rollback"))
		response(w, 200, agentapi.Status{SiteLifecycle: true, InstallReady: d.WP.TemplateReady() && nginxReady, Connected: true, Docker: e == nil, Nginx: nginxReady, Certbot: certReady, RecoveryRequired: recoveryErr != nil || len(recoveries) > 0 || markerErr != nil || len(markers) > 0, Version: "0.3.0"})
	})
	mux.HandleFunc("GET /containers", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
		defer cancel()
		v, e := d.Docker.List(ctx)
		if e != nil {
			response(w, 503, map[string]string{"error": "docker unavailable"})
			return
		}
		response(w, 200, v)
	})
	mux.HandleFunc("GET /containers/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
		n, e := strconv.Atoi(r.URL.Query().Get("tail"))
		if e != nil {
			response(w, 422, nil)
			return
		}
		v, e := d.Docker.Logs(r.Context(), r.PathValue("id"), n)
		if e != nil {
			response(w, 403, nil)
			return
		}
		response(w, 200, v)
	})
	mux.HandleFunc("POST /operations", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 65536)
		b, e := io.ReadAll(r.Body)
		var in agentapi.OperationRequest
		if e != nil || agentapi.Decode(b, &in) != nil || in.Validate() != nil {
			response(w, 422, nil)
			return
		}
		recoveries, e := d.Store.Q.Recoveries(r.Context())
		if e != nil || len(recoveries) > 0 {
			response(w, 409, nil)
			return
		}
		pending, e := d.Store.Queued(r.Context())
		if e != nil || len(pending) > 128 {
			response(w, 503, nil)
			return
		}
		v, e := d.Store.Reserve(r.Context(), in)
		if e != nil {
			response(w, 409, nil)
			return
		}
		response(w, 202, v)
	})
	mux.HandleFunc("GET /operations/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := d.Store.Operation(r.Context(), r.PathValue("id"))
		if e != nil {
			response(w, 404, nil)
			return
		}
		response(w, 200, v)
	})
	mux.HandleFunc("GET /backups/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		f, b, e := d.Backups.Download(r.Context(), r.PathValue("id"))
		if e != nil {
			response(w, 404, nil)
			return
		}
		defer f.Close()
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Length", strconv.FormatInt(b.SizeBytes, 10))
		w.WriteHeader(200)
		_, _ = io.Copy(w, f)
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowed(r.Context(), cfg.PeerUID, cfg.PeerGID) {
			response(w, 403, nil)
			return
		}
		mux.ServeHTTP(w, r)
	})
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second, ConnContext: credentials}
	return srv, l, nil
}
