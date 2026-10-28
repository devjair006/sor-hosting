package deletion

import (
	"context"
	"errors"
	"github.com/moby/moby/client"
	"sor.hosting/agent/internal/backups"
	"sor.hosting/agent/internal/wordpress"
	"sor.hosting/shared/agentapi"
	"sort"
)

type Manager struct {
	WP      *wordpress.Manager
	Backups *backups.Manager
}
type Group struct {
	Ref, Network, NetworkID string
	Containers              []string
	Volumes                 []Volume
}
type Volume struct{ Name, CreatedAt string }
type Plan struct {
	Suspended   bool
	Kind        string `json:"kind"`
	ID          string
	Site        agentapi.SiteResources
	KeepBackups bool
	Committed   bool
	Groups      []Group
	Running     []string
	Files       map[string][]byte
	Domains     []agentapi.DomainResult
}

var ErrPending = errors.New("DELETE_PENDING")

func reject() error { return errors.New("resource ownership/shared/protected verification failed") }

func (m *Manager) preflight(ctx context.Context, s agentapi.SiteResources) (Plan, error) {
	p := Plan{Kind: "site.delete", Site: s}
	if !s.Mutable() || s.Adopted || len(s.Writers) > 0 || !agentapi.OpaqueID.MatchString(s.ID) {
		return p, reject()
	}
	if e := m.WP.CheckProxy(ctx); e != nil {
		return p, e
	}
	allSites, e := m.WP.Store.Sites(ctx)
	if e != nil {
		return p, e
	}
	all, e := m.WP.Docker.API.ContainerList(ctx, client.ContainerListOptions{All: true})
	if e != nil {
		return p, e
	}
	groups := map[string]*Group{}
	selected := map[string]bool{}
	for _, c := range all.Items {
		if c.Labels["sor.hosting/site"] != s.ID {
			continue
		}
		ref := c.Labels["sor.hosting/project"]
		if !agentapi.OpaqueID.MatchString(ref) || c.Labels["com.docker.compose.project"] != "sor-"+ref || (c.Labels["com.docker.compose.service"] != "web" && c.Labels["com.docker.compose.service"] != "db") || m.WP.Docker.IsProtected(c.ID, c.Image, c.Labels, c.Names...) {
			return p, reject()
		}
		if groups[ref] == nil {
			groups[ref] = &Group{Ref: ref, Network: "sor_" + ref + "_network"}
		}
		groups[ref].Containers = append(groups[ref].Containers, c.ID)
		selected[c.ID] = true
	}
	if !selected[s.WebID] || !selected[s.DBID] {
		return p, reject()
	}
	for _, other := range allSites {
		if other.ID == s.ID || !other.Present() {
			continue
		}
		if other.Domain == s.Domain {
			return p, reject()
		}
		for _, id := range append([]string{other.WebID, other.DBID}, other.Writers...) {
			if id != "" && selected[id] {
				return p, reject()
			}
		}
	}
	for _, g := range groups {
		if len(g.Containers) != 2 || g.Network == m.WP.Cfg.Nginx.ProxyNetwork {
			return p, reject()
		}
		n, e := m.WP.Docker.API.NetworkInspect(ctx, g.Network, client.NetworkInspectOptions{})
		if e != nil {
			return p, e
		}
		if n.Network.Labels["com.docker.compose.project"] != "sor-"+g.Ref || n.Network.Driver != "bridge" || n.Network.Ingress {
			return p, reject()
		}
		g.NetworkID = n.Network.ID
		for id := range n.Network.Containers {
			if !selected[id] {
				return p, reject()
			}
		}
		for _, c := range all.Items {
			if !selected[c.ID] && c.NetworkSettings != nil && c.NetworkSettings.Networks[g.Network] != nil {
				return p, reject()
			}
		}
		for _, role := range []string{"wp", "db"} {
			name := "sor_" + g.Ref + "_" + role
			v, e := m.WP.Docker.API.VolumeInspect(ctx, name, client.VolumeInspectOptions{})
			if e != nil {
				return p, e
			}
			if v.Volume.Name != name || v.Volume.Labels["com.docker.compose.project"] != "sor-"+g.Ref || v.Volume.Labels["com.docker.compose.volume"] != role || v.Volume.Driver != "local" || len(v.Volume.Options) > 0 || v.Volume.CreatedAt == "" {
				return p, reject()
			}
			for _, c := range all.Items {
				for _, mount := range c.Mounts {
					if mount.Name == name && !selected[c.ID] {
						return p, reject()
					}
				}
			}
			for _, other := range allSites {
				if other.ID != s.ID && other.Present() && (other.WebVolume == name || other.DBVolume == name || other.Network == g.Network) {
					return p, reject()
				}
			}
			g.Volumes = append(g.Volumes, Volume{Name: name, CreatedAt: v.Volume.CreatedAt})
		}
		roles := map[string]bool{}
		for _, id := range g.Containers {
			c, e := m.WP.Docker.API.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
			if e != nil {
				return p, e
			}
			if c.Container.Config == nil || m.WP.Docker.IsProtected(id, c.Container.Config.Image, c.Container.Config.Labels, c.Container.Name) {
				return p, reject()
			}
			role := c.Container.Config.Labels["com.docker.compose.service"]
			if roles[role] || (role != "web" && role != "db") {
				return p, reject()
			}
			roles[role] = true
			expectedVolume, target := "sor_"+g.Ref+"_wp", "/var/www/html"
			if role == "db" {
				expectedVolume = "sor_" + g.Ref + "_db"
				target = "/var/lib/mysql"
			}
			mounted := false
			for _, mount := range c.Container.Mounts {
				if mount.Name == expectedVolume && mount.Destination == target {
					mounted = true
				}
			}
			if !mounted {
				return p, reject()
			}
			if (id == s.WebID && role != "web") || (id == s.DBID && role != "db") {
				return p, reject()
			}
			if c.Container.State != nil && c.Container.State.Running {
				p.Running = append(p.Running, id)
			}
			for _, mount := range c.Container.Mounts {
				if string(mount.Type) == "volume" && mount.Name != "sor_"+g.Ref+"_wp" && mount.Name != "sor_"+g.Ref+"_db" {
					return p, reject()
				}
			}
		}
		sort.Strings(g.Containers)
		p.Groups = append(p.Groups, *g)
	}
	sort.Slice(p.Groups, func(i, j int) bool { return p.Groups[i].Ref < p.Groups[j].Ref })
	p.Domains, e = m.WP.Store.Domains(ctx, s.ID)
	if e != nil {
		return p, e
	}
	hosts := []string{s.Domain}
	for _, d := range p.Domains {
		if d.SiteID != s.ID {
			return p, reject()
		}
		hosts = append(hosts, d.Hostname)
		if e = m.WP.Certbot.VerifyDeletion(ctx, d); e != nil {
			return p, e
		}
	}
	for _, other := range allSites {
		if other.ID == s.ID || !other.Present() {
			continue
		}
		ds, e := m.WP.Store.Domains(ctx, other.ID)
		if e != nil {
			return p, e
		}
		for _, a := range p.Domains {
			for _, b := range ds {
				if a.Hostname == b.Hostname || a.Certificate != nil && b.Certificate != nil && a.Certificate.CertName == b.Certificate.CertName {
					return p, reject()
				}
			}
		}
		for _, d := range p.Domains {
			if d.Hostname == other.Domain {
				return p, reject()
			}
		}
	}
	p.Files, e = m.WP.Nginx.DeletionFiles(ctx, hosts)
	return p, e
}
