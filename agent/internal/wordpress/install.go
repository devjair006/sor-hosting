package wordpress

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/moby/moby/client"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sor.hosting/agent/internal/certbot"
	"sor.hosting/agent/internal/config"
	"sor.hosting/agent/internal/docker"
	"sor.hosting/agent/internal/executil"
	"sor.hosting/agent/internal/journal"
	"sor.hosting/agent/internal/nginx"
	"sor.hosting/shared/agentapi"
	"sor.hosting/shared/domainname"
	"strconv"
	"strings"
	"time"
)

type Manager struct {
	Cfg     config.Config
	Docker  *docker.Manager
	Store   *journal.Store
	Nginx   *nginx.Manager
	Certbot *certbot.Client
	Runner  executil.Runner
}

var imageDigest = regexp.MustCompile(`^[a-zA-Z0-9./:_-]+@sha256:[0-9a-f]{64}$`)

func (m *Manager) dir(ref string) string { return filepath.Join(m.Cfg.StateDir, "sites", ref) }
func (m *Manager) Secrets(ref string) []string {
	if !agentapi.OpaqueID.MatchString(ref) {
		return nil
	}
	var out []string
	for _, name := range []string{"db-password", "db-root-password"} {
		b, e := os.ReadFile(filepath.Join(m.dir(ref), name))
		if e == nil {
			out = append(out, string(b))
		}
	}
	return out
}
func randomSecret() (string, error) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}
func (m *Manager) Compose(ctx context.Context, ref string, args []string, in io.Reader, out io.Writer) error {
	if !agentapi.OpaqueID.MatchString(ref) {
		return errors.New("invalid project reference")
	}
	base := []string{"compose", "--project-name", "sor-" + ref, "--file", filepath.Join(m.dir(ref), "compose.json")}
	return m.Runner.Run(ctx, "docker", append(base, args...), in, out)
}
func (m *Manager) Prepare(ctx context.Context, siteID, ref, secretRef string, in agentapi.SiteInput) (agentapi.SiteResources, error) {
	if e := m.CheckProxy(ctx); e != nil {
		return agentapi.SiteResources{}, e
	}
	s := agentapi.SiteResources{Budget: in.Resources, ID: siteID, Name: in.Name, Domain: in.Domain, SecretRef: secretRef, State: "provisioning", Network: "sor_" + ref + "_network", WebVolume: "sor_" + ref + "_wp", DBVolume: "sor_" + ref + "_db"}
	if !agentapi.OpaqueID.MatchString(siteID) || !agentapi.OpaqueID.MatchString(ref) || !agentapi.OpaqueID.MatchString(secretRef) {
		return s, errors.New("invalid site ID")
	}
	if _, e := domainname.Normalize(in.Domain); e != nil {
		return s, e
	}
	if in.Resources.MemoryMB < 256 || in.Resources.MemoryMB > 16384 || in.Resources.CPULimit < .25 || in.Resources.CPULimit > 16 {
		return s, errors.New("invalid resources")
	}
	for _, image := range []string{m.Cfg.WordPressImage, m.Cfg.MariaDBImage, m.Cfg.CLIImage} {
		if !imageDigest.MatchString(image) {
			return s, errors.New("configure pinned image digests in agent config")
		}
	}
	dir := m.dir(ref)
	if _, e := os.Stat(dir); e == nil {
		return s, errors.New("project already exists; review previous operation")
	}
	if ref == siteID {
		if e := m.Store.SaveSite(ctx, s); e != nil {
			return s, e
		}
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		return s, e
	}
	if ref == secretRef {
		password, e := randomSecret()
		if e != nil {
			return s, e
		}
		root, e := randomSecret()
		if e != nil {
			return s, e
		}
		for name, value := range map[string]string{"db-password": password, "db-root-password": root, "db-client.cnf": "[client]\nuser=root\npassword=" + root + "\n"} {
			mode := os.FileMode(0400)
			if name == "db-password" {
				mode = 0444
			}
			if e = writeSecret(filepath.Join(dir, name), value, mode); e != nil {
				return s, e
			}
		}
	}
	labels := map[string]string{"sor.hosting/site": siteID, "sor.hosting/project": ref}
	mounts := []any{map[string]any{"type": "volume", "source": "wp", "target": "/var/www/html"}}
	webEnv := map[string]string{"WORDPRESS_DB_HOST": "db:3306", "WORDPRESS_DB_NAME": "wordpress", "WORDPRESS_DB_USER": "wordpress", "WORDPRESS_DB_PASSWORD_FILE": "/run/secrets/db_password"}
	secrets := map[string]any{"db_password": map[string]string{"file": filepath.Join(m.dir(secretRef), "db-password")}, "db_root_password": map[string]string{"file": filepath.Join(m.dir(secretRef), "db-root-password")}, "db_client": map[string]string{"file": filepath.Join(m.dir(secretRef), "db-client.cnf")}}
	compose := map[string]any{"services": map[string]any{"web": map[string]any{"image": m.Cfg.WordPressImage, "restart": "unless-stopped", "labels": labels, "volumes": mounts, "environment": webEnv, "secrets": []string{"db_password"}, "ports": []any{map[string]any{"target": 80, "published": "0", "host_ip": "127.0.0.1", "protocol": "tcp"}}, "mem_limit": in.Resources.MemoryMB * 1024 * 1024, "cpus": in.Resources.CPULimit, "networks": []string{"site"}}, "db": map[string]any{"image": m.Cfg.MariaDBImage, "restart": "unless-stopped", "labels": labels, "volumes": []string{"db:/var/lib/mysql"}, "environment": map[string]string{"MARIADB_DATABASE": "wordpress", "MARIADB_USER": "wordpress", "MARIADB_PASSWORD_FILE": "/run/secrets/db_password", "MARIADB_ROOT_PASSWORD_FILE": "/run/secrets/db_root_password"}, "secrets": []string{"db_password", "db_root_password", "db_client"}, "networks": []string{"site"}}, "cli": map[string]any{"image": m.Cfg.CLIImage, "profiles": []string{"tools"}, "user": "33:33", "labels": labels, "volumes": mounts, "environment": webEnv, "secrets": []string{"db_password"}, "networks": []string{"site"}}}, "volumes": map[string]any{"wp": map[string]string{"name": s.WebVolume}, "db": map[string]string{"name": s.DBVolume}}, "networks": map[string]any{"site": map[string]string{"name": s.Network}}, "secrets": secrets}
	if m.Cfg.Nginx.Mode == "docker" {
		network := m.Cfg.Nginx.ProxyNetwork
		if network == "" {
			return s, errors.New("explicit proxy network required")
		}
		compose["networks"].(map[string]any)["proxy"] = map[string]any{"name": network, "external": true}
		compose["services"].(map[string]any)["web"].(map[string]any)["networks"] = map[string]any{"site": map[string]any{}, "proxy": map[string]any{"aliases": []string{"sor-" + ref}}}
	}
	b, e := json.MarshalIndent(compose, "", "  ")
	if e != nil {
		return s, e
	}
	if e = os.WriteFile(filepath.Join(dir, "compose.json"), b, 0600); e != nil {
		return s, e
	}
	if e = m.Compose(ctx, ref, []string{"create", "web", "db"}, nil, nil); e != nil {
		return s, e
	}
	for name, target := range map[string]*string{"web": &s.WebID, "db": &s.DBID} {
		buf := &executil.Limited{Max: 1024}
		if e = m.Compose(ctx, ref, []string{"ps", "--all", "--quiet", name}, nil, buf); e != nil {
			return s, e
		}
		*target = strings.TrimSpace(buf.Buffer.String())
		if ref == siteID {
			if e = m.Store.SaveSite(ctx, s); e != nil {
				return s, e
			}
		}
		if !agentapi.OpaqueID.MatchString(*target) {
			return s, errors.New("Docker returned invalid ID")
		}
	}
	return s, nil
}
func (m *Manager) RefreshPort(ctx context.Context, s *agentapi.SiteResources) error {
	v, e := m.Docker.API.ContainerInspect(ctx, s.WebID, client.ContainerInspectOptions{})
	if e != nil {
		return e
	}
	if v.Container.NetworkSettings == nil {
		return errors.New("missing network settings")
	}
	for p, bindings := range v.Container.NetworkSettings.Ports {
		if p.String() != "80/tcp" {
			continue
		}
		for _, b := range bindings {
			if b.HostIP.String() != "127.0.0.1" {
				return errors.New("unsafe host binding")
			}
			port, e := strconv.ParseUint(b.HostPort, 10, 16)
			if e == nil && port > 0 {
				s.Port = uint16(port)
				return nil
			}
		}
	}
	return errors.New("loopback port not assigned")
}
func (m *Manager) WaitDatabase(ctx context.Context, id string) error {
	for i := 0; i < 90; i++ {
		if m.Runner.Run(ctx, "docker", []string{"exec", id, "healthcheck.sh", "--connect", "--innodb_initialized"}, nil, nil) == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return errors.New("database readiness timeout")
}
func (m *Manager) WaitWeb(ctx context.Context, s agentapi.SiteResources) error {
	c := http.Client{Timeout: 3 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	for i := 0; i < 45; i++ {
		req, e := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://127.0.0.1:%d/", s.Port), nil)
		if e != nil {
			return e
		}
		req.Host = s.Domain
		res, e := c.Do(req)
		if e == nil {
			res.Body.Close()
			if res.StatusCode >= 200 && res.StatusCode < 400 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return errors.New("WordPress readiness timeout")
}
func (m *Manager) Install(ctx context.Context, r agentapi.OperationRequest, checkpoint func(string) error) (s agentapi.SiteResources, err error) {
	var in agentapi.SiteInput
	if e := agentapi.Decode(r.Payload, &in); e != nil {
		return s, e
	}
	host, e := domainname.Normalize(in.Domain)
	if e != nil {
		return s, e
	}
	in.Domain = host
	if e = m.CheckProxy(ctx); e != nil {
		return s, e
	}
	all, e := m.Store.Sites(ctx)
	if e != nil {
		return s, e
	}
	for _, v := range all {
		if !v.Present() {
			continue
		}
		if v.Domain == host || v.ID == r.ResourceID {
			return s, errors.New("domain or resource ID already managed")
		}
	}
	if m.Cfg.Nginx.Mode == "docker" && !m.Nginx.Available(ctx) {
		return s, errors.New("Docker Nginx mounts/network are not ready")
	}
	if e = checkpoint("Creando recursos aislados"); e != nil {
		return s, e
	}
	s, e = m.Prepare(ctx, r.ResourceID, r.ResourceID, r.ResourceID, in)
	if s.ID != "" {
		_ = m.Store.SaveSite(ctx, s)
	}
	defer func() {
		if err != nil && s.ID != "" {
			s.State = "failed"
			_ = m.Store.SaveSite(context.Background(), s)
		}
	}()
	if e != nil {
		return s, e
	}
	if e = checkpoint("Iniciando base de datos"); e != nil {
		return s, e
	}
	if e = m.Docker.RawAction(ctx, s.DBID, "start"); e != nil {
		return s, e
	}
	if e = m.WaitDatabase(ctx, s.DBID); e != nil {
		return s, e
	}
	if e = checkpoint("Instalando WordPress"); e != nil {
		return s, e
	}
	if e = m.Docker.RawAction(ctx, s.WebID, "start"); e != nil {
		return s, e
	}
	if e = m.RefreshPort(ctx, &s); e != nil {
		return s, e
	}
	if e = m.WaitWeb(ctx, s); e != nil {
		return s, e
	}
	if len(in.AdminPassword) < 12 || strings.ContainsAny(in.AdminPassword, "\r\n") {
		return s, errors.New("invalid WordPress password")
	}
	args := []string{"run", "--rm", "--no-deps", "-T", "cli", "wp", "core", "install", "--url=https://" + in.Domain, "--title=" + in.WordPressTitle, "--admin_user=sor_admin", "--admin_email=" + in.AdminEmail, "--skip-email", "--prompt=admin_password"}
	if e = m.Compose(ctx, r.ResourceID, args, strings.NewReader(in.AdminPassword+"\n"), nil); e != nil {
		return s, e
	}
	if e = m.Store.SaveSite(ctx, s); e != nil {
		return s, e
	}
	if e = checkpoint("Configurando dominio y HTTPS"); e != nil {
		return s, e
	}
	d, e := m.ConfigureDomain(ctx, s.ID, s.ID, s.Domain, false)
	if e != nil {
		return s, e
	}
	s.PrimaryDomain = &d
	if d.DNSState != "matched" {
		s.State = "pending_dns"
	} else if d.Certificate == nil {
		s.State = "pending_ssl"
	} else {
		s.State = "active"
	}
	if e = m.Store.SaveSite(ctx, s); e != nil {
		return s, e
	}
	return s, nil
}
func (m *Manager) ConfigureDomain(ctx context.Context, id, siteID, host string, renew bool) (agentapi.DomainResult, error) {
	d := agentapi.DomainResult{ID: id, SiteID: siteID, DNSState: "pending"}
	host, e := domainname.Normalize(host)
	if e != nil {
		return d, e
	}
	d.Hostname = host
	s, e := m.Store.Site(ctx, siteID)
	if e != nil {
		return d, e
	}
	if !s.Mutable() {
		return d, errors.New("read-only site")
	}
	sites, e := m.Store.Sites(ctx)
	if e != nil {
		return d, e
	}
	for _, other := range sites {
		ds, e := m.Store.Domains(ctx, other.ID)
		if e != nil {
			return d, e
		}
		for _, prev := range ds {
			if prev.Hostname == host && prev.ID != id {
				return d, errors.New("domain already registered")
			}
		}
	}
	old, oldErr := m.Store.Domain(ctx, id)
	if renew && (oldErr != nil || old.Certificate == nil) {
		return d, errors.New("unknown managed certificate")
	}
	certName := ""
	if oldErr == nil && old.Certificate != nil {
		certName = old.Certificate.CertName
		d.Certificate = old.Certificate
	}
	if e = m.ApplyProxy(ctx, host, s, certName); e != nil {
		return d, e
	}
	if m.Certbot.DNS(ctx, host) {
		d.DNSState = "matched"
		cert, e := m.Certbot.Issue(ctx, id, host, renew)
		if e == nil {
			d.Certificate = cert
			if e = m.ApplyProxy(ctx, host, s, cert.CertName); e != nil {
				return d, e
			}
		} else if renew {
			return d, e
		}
	}
	if e = m.Store.SaveDomain(ctx, d); e != nil {
		return d, e
	}
	if s.Domain == host {
		s.PrimaryDomain = &d
		if d.DNSState != "matched" {
			s.State = "pending_dns"
		} else if d.Certificate == nil {
			s.State = "pending_ssl"
		} else {
			s.State = "active"
		}
		if e = m.Store.SaveSite(ctx, s); e != nil {
			return d, e
		}
	}
	return d, nil
}
func (m *Manager) Adopt(ctx context.Context, r agentapi.OperationRequest) (agentapi.SiteResources, error) {
	var in agentapi.AdoptInput
	if e := agentapi.Decode(r.Payload, &in); e != nil {
		return agentapi.SiteResources{}, e
	}
	s, ok := m.Cfg.Adoptions[in.WebContainerID]
	if !ok || s.WebID != in.WebContainerID || s.DBID != in.DatabaseContainerID || s.Domain != in.Domain || s.SecretRef != in.DatabaseSecretRef {
		return s, errors.New("site must be approved locally")
	}
	if len(in.WriterContainerIDs) != 0 {
		return s, errors.New("writers must be declared in local configuration")
	}
	existing, e := m.Store.Sites(ctx)
	if e != nil {
		return agentapi.SiteResources{}, e
	}
	wanted := append([]string{s.WebID, s.DBID}, s.Writers...)
	for _, other := range existing {
		if !other.Present() {
			continue
		}
		if other.Domain == s.Domain || other.ID == r.ResourceID {
			return agentapi.SiteResources{}, errors.New("domain already adopted")
		}
		for _, oldID := range append([]string{other.WebID, other.DBID}, other.Writers...) {
			for _, newID := range wanted {
				if oldID == newID {
					return agentapi.SiteResources{}, errors.New("container already associated")
				}
			}
		}
	}
	s.ID = r.ResourceID
	s.Name = in.Name
	s.Adopted = true
	s.State = "active"
	for _, id := range append([]string{s.WebID, s.DBID}, s.Writers...) {
		v, e := m.Docker.API.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
		if e != nil || v.Container.ID != id || v.Container.Config == nil {
			return agentapi.SiteResources{}, errors.New("container ID mismatch")
		}
		if m.Docker.IsProtected(id, v.Container.Config.Image, v.Container.Config.Labels, v.Container.Name) {
			return agentapi.SiteResources{}, errors.New("protected container")
		}
		for _, protected := range m.Cfg.ProtectedIDs {
			if protected == id {
				return s, errors.New("protected container")
			}
		}
	}
	if e := m.RefreshPort(ctx, &s); e != nil {
		return s, e
	}
	return s, m.Store.SaveSite(ctx, s)
}
func (m *Manager) Dump(ctx context.Context, s agentapi.SiteResources, out io.Writer) error {
	return m.Runner.Run(ctx, "docker", []string{"exec", s.DBID, "mariadb-dump", "--defaults-extra-file=/run/secrets/db_client", "--single-transaction", "--routines", "--events", "--triggers", "--databases", "wordpress"}, nil, out)
}
func (m *Manager) Import(ctx context.Context, s agentapi.SiteResources, in io.Reader) error {
	return m.Runner.Run(ctx, "docker", []string{"exec", "-i", s.DBID, "mariadb", "--defaults-extra-file=/run/secrets/db_client"}, in, nil)
}
func (m *Manager) CopyOut(ctx context.Context, id string, out io.Writer) error {
	r, e := m.Docker.API.CopyFromContainer(ctx, id, client.CopyFromContainerOptions{SourcePath: "/var/www/html"})
	if e != nil {
		return e
	}
	defer r.Content.Close()
	_, e = io.Copy(out, r.Content)
	return e
}
func (m *Manager) CopyIn(ctx context.Context, id string, in io.Reader) error {
	_, e := m.Docker.API.CopyToContainer(ctx, id, client.CopyToContainerOptions{DestinationPath: "/var/www", Content: in, AllowOverwriteDirWithFile: false, CopyUIDGID: true})
	return e
}
func (m *Manager) PortOpen(s agentapi.SiteResources) bool {
	conn, e := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", s.Port), time.Second)
	if e == nil {
		conn.Close()
	}
	return e == nil
}

func writeSecret(path, value string, mode os.FileMode) error {
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return e
	}
	if _, e = f.WriteString(value); e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	return os.Chmod(path, mode)
}

func (m *Manager) TemplateReady() bool {
	for _, image := range []string{m.Cfg.WordPressImage, m.Cfg.MariaDBImage, m.Cfg.CLIImage} {
		if !imageDigest.MatchString(image) {
			return false
		}
	}
	return true
}
