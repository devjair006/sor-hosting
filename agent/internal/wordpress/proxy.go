package wordpress

import (
	"context"
	"errors"
	"github.com/moby/moby/client"
	"sor.hosting/shared/agentapi"
	"strings"
)

func (m *Manager) ProxyAlias(ctx context.Context, s agentapi.SiteResources) (string, error) {
	web, e := m.Docker.API.ContainerInspect(ctx, s.WebID, client.ContainerInspectOptions{})
	if e != nil {
		return "", e
	}
	proxy, e := m.Docker.API.ContainerInspect(ctx, m.Cfg.Nginx.Container, client.ContainerInspectOptions{})
	if e != nil {
		return "", e
	}
	if web.Container.NetworkSettings == nil || proxy.Container.NetworkSettings == nil {
		return "", errors.New("proxy network missing")
	}
	a, b := web.Container.NetworkSettings.Networks[m.Cfg.Nginx.ProxyNetwork], proxy.Container.NetworkSettings.Networks[m.Cfg.Nginx.ProxyNetwork]
	if a == nil || b == nil || a.NetworkID == "" || a.NetworkID != b.NetworkID {
		return "", errors.New("WordPress web and Nginx must share the explicit proxy network")
	}
	alias := strings.TrimPrefix(web.Container.Name, "/")
	if web.Container.Config != nil && agentapi.OpaqueID.MatchString(web.Container.Config.Labels["sor.hosting/project"]) {
		alias = "sor-" + web.Container.Config.Labels["sor.hosting/project"]
	}
	for _, v := range a.Aliases {
		if v == alias {
			return alias, nil
		}
	}
	return "", errors.New("unique web alias missing from proxy network")
}
func (m *Manager) ApplyProxy(ctx context.Context, host string, s agentapi.SiteResources, cert string) error {
	if m.Cfg.Nginx.Mode != "docker" {
		return m.Nginx.Apply(ctx, host, s.Port, cert)
	}
	alias, e := m.ProxyAlias(ctx, s)
	if e != nil {
		return e
	}
	return m.Nginx.ApplyDNS(ctx, host, alias, cert)
}

func (m *Manager) CheckProxy(ctx context.Context) error {
	if m.Nginx != nil && m.Nginx.Check != nil {
		return m.Nginx.Check(ctx)
	}
	return nil
}
