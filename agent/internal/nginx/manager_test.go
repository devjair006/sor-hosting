package nginx

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
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
	os.WriteFile(p, []byte("old configuration"), 0600)
	m := Manager{Dir: dir, Webroot: web, Runner: rejectValidation{dir}}
	if m.Apply(context.Background(), "a.example", 8000, "") == nil {
		t.Fatal("invalid config accepted")
	}
	b, _ := os.ReadFile(p)
	if string(b) != "old configuration" {
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
