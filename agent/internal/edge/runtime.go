// Package edge wires the same native/Docker adapters for the agent and integration tests.
package edge

import (
	"context"
	"github.com/moby/moby/client"
	"sor.hosting/agent/internal/certbot"
	"sor.hosting/agent/internal/config"
	"sor.hosting/agent/internal/dockerexec"
	"sor.hosting/agent/internal/journal"
	"sor.hosting/agent/internal/nginx"
)

func New(cfg config.Config, api *client.Client, store *journal.Store) (*nginx.Manager, *certbot.Client) {
	proxy := &nginx.Manager{Dir: cfg.NginxDir, Webroot: cfg.ACMEWebroot, ContainerDir: cfg.Nginx.IncludeDir, ContainerWebroot: cfg.Nginx.Webroot, CertificatesDir: cfg.Nginx.CertificatesDir, ProtectedDomains: cfg.ProtectedDomains}
	certs := &certbot.Client{Directory: cfg.Certbot.Directory, Webroot: cfg.ACMEWebroot, Email: cfg.CertbotEmail, PublicIPs: cfg.PublicIPs, ProtectedDomains: cfg.ProtectedDomains, Server: cfg.Certbot.Server, ExecutionDirectory: cfg.Certbot.Directory}
	if cfg.Nginx.Mode == "docker" {
		runtime := &dockerexec.Nginx{API: api, Container: cfg.Nginx.Container, Network: cfg.Nginx.ProxyNetwork, Mounts: []dockerexec.Bind{{Source: cfg.NginxDir, Target: cfg.Nginx.IncludeDir}, {Source: cfg.Certbot.ConfigSource, Target: cfg.Nginx.CertificatesDir}, {Source: cfg.Certbot.WebrootSource, Target: cfg.Nginx.Webroot}}}
		proxy.Runner = runtime
		proxy.Health = runtime.Alive
		proxy.Check = func(ctx context.Context) error { _, e := runtime.Inspect(ctx); return e }
	}
	if cfg.Certbot.Mode == "docker" {
		certs.Check = func() error { return requireAliasMount(cfg.Certbot.ConfigSource, cfg.Certbot.Directory) }
		certs.Runner = &dockerexec.Certbot{API: api, Image: cfg.Certbot.Image, ConfigSource: cfg.Certbot.ConfigSource, WebrootSource: cfg.Certbot.WebrootSource}
		certs.ContainerWebroot = "/var/www/certbot"
		certs.ExecutionDirectory = "/etc/letsencrypt"
	}
	certs.Managed = func(ctx context.Context, id, host, name string) bool {
		if store == nil {
			return false
		}
		d, e := store.Domain(ctx, id)
		return e == nil && d.Hostname == host && d.Certificate != nil && d.Certificate.CertName == name
	}
	return proxy, certs
}
