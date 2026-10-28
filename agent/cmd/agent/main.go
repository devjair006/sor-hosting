package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sor.hosting/agent/internal/backups"
	"sor.hosting/agent/internal/certbot"
	"sor.hosting/agent/internal/config"
	"sor.hosting/agent/internal/docker"
	"sor.hosting/agent/internal/journal"
	"sor.hosting/agent/internal/nginx"
	"sor.hosting/agent/internal/operations"
	"sor.hosting/agent/internal/rpc"
	"sor.hosting/agent/internal/wordpress"
	"syscall"
	"time"
)

func run() error {
	path := flag.String("config", "/etc/sor-hosting/agent.json", "Agent configuration file")
	flag.Parse()
	cfg, e := config.Load(*path)
	if e != nil {
		return e
	}
	for _, dir := range []string{cfg.StateDir, cfg.BackupDir, filepath.Join(cfg.StateDir, "sites")} {
		if e = os.MkdirAll(dir, 0700); e != nil {
			return e
		}
		if e = nginx.PrivatePath(dir); e != nil {
			return e
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	store, e := journal.Open(ctx, filepath.Join(cfg.StateDir, "agent"), cfg.Key)
	if e != nil {
		return e
	}
	defer store.DB.Close()
	engine, e := docker.New(store, cfg.ProtectedIDs)
	if e != nil {
		return e
	}
	defer engine.API.Close()
	proxy := &nginx.Manager{Dir: cfg.NginxDir, Webroot: cfg.ACMEWebroot}
	certs := &certbot.Client{Webroot: cfg.ACMEWebroot, Email: cfg.CertbotEmail, PublicIPs: cfg.PublicIPs}
	wp := &wordpress.Manager{Cfg: cfg, Docker: engine, Store: store, Nginx: proxy, Certbot: certs}
	engine.Secrets = wp.Secrets
	backup := &backups.Manager{Cfg: cfg, Store: store, WP: wp}
	dispatcher := &operations.Dispatcher{Store: store, Docker: engine, WP: wp, Backups: backup}
	srv, l, e := rpc.Server(cfg, dispatcher)
	if e != nil {
		return e
	}
	defer l.Close()
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		if e := dispatcher.Run(ctx); e != nil && ctx.Err() == nil {
			slog.Error("agent worker stopped")
			cancel()
		}
	}()
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdown, c := context.WithTimeout(context.Background(), 30*time.Second)
		defer c()
		if srv.Shutdown(shutdown) != nil {
			_ = srv.Close()
		}
	}()
	e = srv.Serve(l)
	cancel()
	<-shutdownDone
	select {
	case <-workerDone:
	case <-time.After(130 * time.Second):
	}
	if e == http.ErrServerClosed {
		return nil
	}
	return e
}
func main() {
	if e := run(); e != nil {
		slog.Error("agent failed to start", "error", e.Error())
		os.Exit(1)
	}
}
