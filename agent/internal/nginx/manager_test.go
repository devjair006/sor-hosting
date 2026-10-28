package nginx

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type rejectValidation struct{ Dir string }

func (r rejectValidation) Run(_ context.Context, _ string, args []string, _ io.Reader, out io.Writer) error {
	if len(args) > 0 && args[0] == "-T" {
		_, _ = io.WriteString(out, "include "+r.Dir+"/*.conf;\n")
		return nil
	}
	if len(args) > 0 && args[0] == "-t" {
		return errors.New("bad config")
	}
	return nil
}
func TestValidationFailurePreservesOldConfig(t *testing.T) {
	dir := t.TempDir()
	web := t.TempDir()
	os.Chmod(dir, 0700)
	os.Chmod(web, 0700)
	p := filepath.Join(dir, "a.example.conf")
	os.WriteFile(p, []byte("# Managed by SOR Hosting\nold configuration"), 0600)
	m := Manager{Dir: dir, Webroot: web, Runner: rejectValidation{dir}}
	if m.Apply(context.Background(), "a.example", 8000, "") == nil {
		t.Fatal("invalid config accepted")
	}
	b, _ := os.ReadFile(p)
	if string(b) != "# Managed by SOR Hosting\nold configuration" {
		t.Fatal("previous configuration lost")
	}
}

type dumpRunner struct{ Dump string }

func (r dumpRunner) Run(_ context.Context, _ string, args []string, _ io.Reader, out io.Writer) error {
	if len(args) > 0 && args[0] == "-T" {
		_, _ = io.WriteString(out, r.Dump)
	}
	return nil
}
func TestUnmanagedDomainCannotBeReplaced(t *testing.T) {
	dir := t.TempDir()
	web := t.TempDir()
	os.Chmod(dir, 0700)
	os.Chmod(web, 0700)
	dump := "# configuration file /etc/nginx/nginx.conf:\ninclude " + dir + "/*.conf;\n# configuration file /etc/nginx/sites-enabled/panel:\nserver { server_name panel.example; listen 80; }\n"
	m := Manager{Dir: dir, Webroot: web, Runner: dumpRunner{dump}}
	if m.Apply(context.Background(), "panel.example", 8000, "") == nil {
		t.Fatal("unmanaged panel vhost overwritten")
	}
	if _, e := os.Stat(filepath.Join(dir, "panel.example.conf")); !os.IsNotExist(e) {
		t.Fatal("unexpected config created")
	}
}

func TestContainerPathsAndProtectedDomains(t *testing.T) {
	dir, web := t.TempDir(), t.TempDir()
	m := Manager{Dir: dir, Webroot: web, ContainerDir: "/etc/nginx/sor-hosting.d", ContainerWebroot: "/var/www/certbot", CertificatesDir: "/etc/letsencrypt", ProtectedDomains: []string{"projectkarli.dev"}}
	m.Runner = dumpRunner{Dump: "# configuration file /etc/nginx/nginx.conf:\ninclude /etc/nginx/sor-hosting.d/*.conf;\n"}
	if err := m.Apply(context.Background(), "new.example", 8080, ""); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "new.example.conf"))
	if !strings.Contains(string(b), `root "/var/www/certbot"`) || strings.Contains(string(b), web) {
		t.Fatal("agent path leaked into container config")
	}
	if err := m.Apply(context.Background(), "projectkarli.dev", 8080, ""); err == nil {
		t.Fatal("protected vhost accepted")
	}
}
func TestManualFileInManagedDirectoryIsNotOverwritten(t *testing.T) {
	dir, web := t.TempDir(), t.TempDir()
	p := filepath.Join(dir, "manual.example.conf")
	old := []byte("# manual configuration\n")
	os.WriteFile(p, old, 0600)
	m := Manager{Dir: dir, Webroot: web, Runner: dumpRunner{Dump: "include " + dir + "/*.conf;\n"}}
	if m.Apply(context.Background(), "manual.example", 8080, "") == nil {
		t.Fatal("manual file overwritten")
	}
	b, _ := os.ReadFile(p)
	if string(b) != string(old) {
		t.Fatal("manual file changed")
	}
}
