//go:build integration

package backups_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"sor.hosting/agent/internal/config"
	"sor.hosting/agent/internal/docker"
	"sor.hosting/agent/internal/dockerexec"
	"sor.hosting/agent/internal/edge"
	"sor.hosting/agent/internal/wordpress"
	"sor.hosting/shared/agentapi"
)

func fixtureDocker(t *testing.T, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	b, e := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	if e != nil {
		t.Fatalf("fixture Docker %s: %v: %s", args[0], e, b)
	}
	return strings.TrimSpace(string(b))
}
func setupDockerEdge(t *testing.T, cfg *config.Config, dm *docker.Manager) func(*wordpress.Manager, agentapi.SiteResources) {
	t.Helper()
	name := "sor-edge-" + integrationID(t)
	network := name + "-proxy"
	root := filepath.Dir(cfg.StateDir)
	certs := filepath.Join(root, "certificates")
	os.MkdirAll(certs, 0755)
	t.Cleanup(func() {
		fixtureDocker(t, "run", "--rm", "--mount", "type=bind,source="+certs+",target=/cleanup", "--entrypoint", "/bin/chown", os.Getenv("SOR_TEST_NGINX_IMAGE"), "-R", strconv.Itoa(os.Getuid())+":"+strconv.Itoa(os.Getgid()), "/cleanup")
	})
	os.Chmod(cfg.ACMEWebroot, 0755)
	challengeDir := filepath.Join(cfg.ACMEWebroot, ".well-known", "acme-challenge")
	os.MkdirAll(challengeDir, 0755)
	os.WriteFile(filepath.Join(challengeDir, "sor-fixture"), []byte("acme-fixture"), 0644)
	legacyCert := filepath.Join(certs, "existing-certificate.pem")
	os.WriteFile(legacyCert, []byte("legacy-certificate-unchanged"), 0644)
	legacy := filepath.Join(root, "default.conf")
	old := []byte("server { listen 80; server_name projectkarli.dev pagina1.projectkarli.dev pagina2.projectkarli.dev sorpanel.projectkarli.dev; location / { return 200 'legacy'; } }\n")
	os.WriteFile(legacy, old, 0644)
	include := filepath.Join(root, "sor-include.conf")
	os.WriteFile(include, []byte("server_names_hash_bucket_size 128;\ninclude /etc/nginx/sor-hosting.d/*.conf;\n"), 0644)
	fixtureDocker(t, "network", "create", network)
	t.Cleanup(func() { fixtureDocker(t, "network", "rm", network) })
	fixtureDocker(t, "run", "-d", "--name", name, "--network", network, "--label", "sor.hosting/infra=true", "-p", "127.0.0.1::80",
		"--mount", "type=bind,source="+cfg.NginxDir+",target=/etc/nginx/sor-hosting.d,readonly",
		"--mount", "type=bind,source="+cfg.ACMEWebroot+",target=/var/www/certbot,readonly",
		"--mount", "type=bind,source="+certs+",target=/etc/letsencrypt,readonly",
		"--mount", "type=bind,source="+legacy+",target=/etc/nginx/conf.d/default.conf,readonly",
		"--mount", "type=bind,source="+include+",target=/etc/nginx/conf.d/sor-include.conf,readonly",
		"--entrypoint", "nginx", os.Getenv("SOR_TEST_NGINX_IMAGE"), "-g", "daemon off;")
	t.Cleanup(func() { fixtureDocker(t, "rm", "-f", name) })
	cfg.Nginx = config.NginxConfig{Mode: "docker", Container: name, ProxyNetwork: network, IncludeDir: "/etc/nginx/sor-hosting.d", Webroot: "/var/www/certbot", CertificatesDir: "/etc/letsencrypt"}
	cfg.Certbot = config.CertbotConfig{Mode: "docker", Image: os.Getenv("SOR_TEST_CERTBOT_IMAGE"), Directory: certs, ConfigSource: certs, WebrootSource: cfg.ACMEWebroot}
	cfg.ProtectedDomains = []string{"projectkarli.dev", "pagina1.projectkarli.dev", "pagina2.projectkarli.dev", "sorpanel.projectkarli.dev"}
	dm.ProtectedNames = append(dm.ProtectedNames, name)
	return func(wp *wordpress.Manager, site agentapi.SiteResources) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		proxy, cb := wp.Nginx, wp.Certbot
		if !proxy.Available(ctx) || !cb.Available(ctx) {
			t.Fatal("Docker runtime unavailable")
		}
		port := strings.Split(fixtureDocker(t, "port", name, "80/tcp"), ":")
		address := "http://127.0.0.1:" + port[len(port)-1] + "/"
		request := func(host string) string {
			t.Helper()
			r, _ := http.NewRequestWithContext(ctx, "GET", address, nil)
			r.Host = host
			c := http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			res, e := c.Do(r)
			if e != nil {
				t.Fatal(e)
			}
			defer res.Body.Close()
			b, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
			if res.StatusCode < 200 || res.StatusCode >= 400 {
				t.Fatalf("proxy HTTP %d", res.StatusCode)
			}
			return string(b)
		}
		if request("projectkarli.dev") != "legacy" {
			t.Fatal("legacy vhost changed")
		}
		request(site.Domain)
		oldAddress := address
		address += ".well-known/acme-challenge/sor-fixture"
		if request(site.Domain) != "acme-fixture" {
			t.Fatal("webroot does not serve the shared ACME challenge")
		}
		address = oldAddress
		alias, e := wp.ProxyAlias(ctx, site)
		if e != nil {
			t.Fatal(e)
		}
		if e = proxy.ApplyDNS(ctx, "rollback.example", alias, ""); e != nil {
			t.Fatal(e)
		}
		path := filepath.Join(cfg.NginxDir, "rollback.example.conf")
		before, _ := os.ReadFile(path)
		real := proxy.Runner
		for _, phase := range []string{"validation", "reload"} {
			proxy.Runner = &edgeFailure{real: real, path: path, phase: phase}
			if proxy.ApplyDNS(ctx, "rollback.example", alias, "") == nil {
				t.Fatal("injected " + phase + " failure accepted")
			}
			after, _ := os.ReadFile(path)
			if string(after) != string(before) {
				t.Fatal("rollback did not restore config")
			}
			proxy.Runner = real
			if e := real.Run(ctx, "nginx", []string{"-t"}, nil, nil); e != nil {
				t.Fatal(e)
			}
		}
		markers, _ := filepath.Glob(filepath.Join(cfg.NginxDir, ".sor-*.rollback"))
		if len(markers) != 0 {
			t.Fatal("completed rollback kept a marker")
		}
		for _, host := range cfg.ProtectedDomains {
			if proxy.ApplyDNS(ctx, host, alias, "") == nil {
				t.Fatal("protected vhost accepted")
			}
		}
		broken := *cfg
		broken.NginxDir = t.TempDir()
		bad, _ := edge.New(broken, dm.API, dm.Store)
		if bad.Available(ctx) || bad.ApplyDNS(ctx, "mount.example", alias, "") == nil {
			t.Fatal("wrong bind source accepted")
		}
		files, _ := os.ReadDir(broken.NginxDir)
		if len(files) != 0 {
			t.Fatal("wrote config before mount verification")
		}
		runner := cb.Runner.(*dockerexec.Certbot)
		if e := runner.Run(ctx, "certbot", []string{"--version"}, nil, nil); e != nil {
			t.Fatal("real Certbot version:", e)
		}
		args := []string{"certonly", "--webroot", "--webroot-path", "/var/www/certbot", "--cert-name", "sor-failure.example", "--domain", "failure.example", "--email", "admin@example.com", "--agree-tos", "--non-interactive", "--no-directory-hooks", "--server", "http://127.0.0.1:9/directory"}
		if runner.Run(ctx, "certbot", args, nil, nil) == nil {
			t.Fatal("unreachable ACME server succeeded")
		}
		if b, e := os.ReadFile(legacyCert); e != nil || string(b) != "legacy-certificate-unchanged" {
			t.Fatal("existing certificate modified")
		}
		fixtureDocker(t, "stop", name)
		if proxy.Available(ctx) {
			t.Fatal("stopped Nginx reported healthy")
		}
		fixtureDocker(t, "start", name)
		after, _ := os.ReadFile(legacy)
		if string(after) != string(old) {
			t.Fatal("default.conf was modified")
		}
		if leftover := fixtureDocker(t, "ps", "-aq", "--filter", "label=sor.hosting/certbot=true"); leftover != "" {
			t.Fatal("temporary Certbot container leaked")
		}
		t.Log("Docker Nginx status/exec, DNS proxy, mount checks, native-vhost protection, validation/reload rollback and real Certbot temporary containers passed")
	}
}

type edgeFailure struct {
	real interface {
		Run(context.Context, string, []string, io.Reader, io.Writer) error
	}
	path, phase string
	failed      bool
}

func (f *edgeFailure) Run(ctx context.Context, name string, args []string, in io.Reader, out io.Writer) error {
	if !f.failed && len(args) > 0 {
		if f.phase == "validation" && args[0] == "-t" {
			f.failed = true
			file, e := os.OpenFile(f.path, os.O_APPEND|os.O_WRONLY, 0600)
			if e != nil {
				return e
			}
			file.WriteString("\ninvalid_sor_directive;\n")
			file.Close()
		}
		if f.phase == "reload" && args[0] == "-s" {
			f.failed = true
			return errors.New("injected reload failure")
		}
	}
	return f.real.Run(ctx, name, args, in, out)
}

// Fixture configs contain no secrets; retain diagnostics only for local integration failures.
type edgeTrace struct {
	real interface {
		Run(context.Context, string, []string, io.Reader, io.Writer) error
	}
	t *testing.T
}

func (f *edgeTrace) Run(ctx context.Context, n string, a []string, in io.Reader, out io.Writer) error {
	var b bytes.Buffer
	var w io.Writer = &b
	if out != nil {
		w = io.MultiWriter(out, &b)
	}
	e := f.real.Run(ctx, n, a, in, w)
	if e != nil {
		f.t.Logf("fixture Nginx %v: %s", a, b.String())
	}
	return e
}
