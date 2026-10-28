//go:build integration

package backups_test

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"sor.hosting/agent/internal/backups"
	"sor.hosting/agent/internal/certbot"
	"sor.hosting/agent/internal/config"
	"sor.hosting/agent/internal/docker"
	"sor.hosting/agent/internal/edge"
	"sor.hosting/agent/internal/executil"
	"sor.hosting/agent/internal/journal"
	"sor.hosting/agent/internal/nginx"
	"sor.hosting/agent/internal/wordpress"
	"sor.hosting/shared/agentapi"
)

// Only proxy execution is replaced: no host Nginx or Certbot configuration is touched.
type proxyFixture struct{ dir string }

func (p proxyFixture) Run(_ context.Context, name string, args []string, _ io.Reader, out io.Writer) error {
	if name != "nginx" {
		return fmt.Errorf("unexpected executable: %s", name)
	}
	if len(args) == 1 && args[0] == "-T" {
		_, err := fmt.Fprintf(out, "include %s/*.conf;\n", p.dir)
		return err
	}
	return nil
}

func integrationID(t *testing.T) string {
	t.Helper()
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func TestDockerWordPressBackupRestore(t *testing.T) {
	if os.Getenv("SOR_DOCKER_INTEGRATION") != "1" {
		t.Skip("set SOR_DOCKER_INTEGRATION=1 and pinned SOR_TEST_*_IMAGE digests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	root := t.TempDir()
	cfg := config.Config{
		StateDir: filepath.Join(root, "state"), BackupDir: filepath.Join(root, "backups"),
		NginxDir: filepath.Join(root, "nginx"), ACMEWebroot: filepath.Join(root, "acme"),
		DockerDataDir: root, WordPressImage: os.Getenv("SOR_TEST_WP_IMAGE"),
		MariaDBImage: os.Getenv("SOR_TEST_DB_IMAGE"), CLIImage: os.Getenv("SOR_TEST_CLI_IMAGE"),
	}
	for _, dir := range []string{cfg.BackupDir, cfg.NginxDir, cfg.ACMEWebroot} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	store, err := journal.Open(ctx, cfg.StateDir, bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.DB.Close() })
	dm, err := docker.New(store, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dm.API.Close() })
	var verifyEdge func(*wordpress.Manager, agentapi.SiteResources)
	if os.Getenv("SOR_EDGE_INTEGRATION") == "1" {
		verifyEdge = setupDockerEdge(t, &cfg, dm)
	}
	wp := &wordpress.Manager{Cfg: cfg, Store: store, Docker: dm,
		Nginx:   &nginx.Manager{Dir: cfg.NginxDir, Webroot: cfg.ACMEWebroot, Runner: proxyFixture{cfg.NginxDir}},
		Certbot: &certbot.Client{}, Runner: executil.Runner{}}
	if verifyEdge != nil {
		wp.Nginx, wp.Certbot = edge.New(cfg, dm.API, store)
		wp.Nginx.Runner = &edgeTrace{real: wp.Nginx.Runner, t: t}
	}
	dm.Secrets = wp.Secrets
	if !wp.TemplateReady() {
		t.Fatal("provide three image@sha256 digests in SOR_TEST_WP_IMAGE, SOR_TEST_DB_IMAGE, SOR_TEST_CLI_IMAGE")
	}
	siteID, restoreID := integrationID(t), integrationID(t)
	// Exact project references only; never prune or stop existing host resources.
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Minute)
		defer stop()
		for _, ref := range []string{restoreID, siteID} {
			if _, err := os.Stat(filepath.Join(cfg.StateDir, "sites", ref, "compose.json")); err == nil {
				if err := wp.Compose(cleanup, ref, []string{"down", "--volumes", "--remove-orphans"}, nil, nil); err != nil {
					t.Errorf("cleanup project %s: %v", ref, err)
				}
			}
		}
	})
	in := agentapi.SiteInput{Name: "Integration", Domain: "sor-" + siteID + ".invalid",
		WordPressTitle: "SOR integration", AdminEmail: "admin@example.invalid", AdminPassword: integrationID(t),
		Resources: agentapi.Resources{MemoryMB: 512, CPULimit: 1}}
	payload, _ := json.Marshal(in)
	site, err := wp.Install(ctx, agentapi.OperationRequest{ID: integrationID(t), Kind: "site.install", ResourceID: siteID, Payload: payload}, func(phase string) error {
		t.Log(phase)
		return nil
	})
	if err != nil {
		t.Fatal("install:", err)
	}
	if site.State != "pending_dns" || site.Port == 0 || site.PrimaryDomain == nil {
		t.Fatalf("unexpected installation state: %s port=%d", site.State, site.Port)
	}
	db, err := dm.API.ContainerInspect(ctx, site.DBID, client.ContainerInspectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, bindings := range db.Container.NetworkSettings.Ports {
		if len(bindings) != 0 {
			t.Fatal("database published a host port")
		}
	}
	if _, err := dm.Authorized(ctx, site.WebID[:12]); err == nil {
		t.Fatal("short Docker ID was authorized")
	}
	dm.ProtectedIDs = []string{site.WebID}
	if err := dm.Action(ctx, site.WebID, "stop"); err == nil {
		t.Fatal("protected container action accepted")
	}
	dm.ProtectedIDs = nil
	for _, action := range []string{"stop", "start", "restart"} {
		if err := dm.Action(ctx, site.WebID, action); err != nil {
			t.Fatal("container action:", action, err)
		}
	}
	if _, err := dm.Logs(ctx, site.WebID, 50); err != nil {
		t.Fatal("container logs:", err)
	}
	cli := func(ref string, args ...string) string {
		t.Helper()
		out := &executil.Limited{Max: 16384}
		base := []string{"run", "--rm", "--no-deps", "-T", "cli", "wp"}
		if err := wp.Compose(ctx, ref, append(base, args...), nil, out); err != nil {
			t.Fatal("WP CLI:", err)
		}
		return strings.TrimSpace(out.Buffer.String())
	}
	cli(siteID, "option", "update", "blogdescription", "before-backup")
	writeMarker := func(value string) {
		t.Helper()
		var buf bytes.Buffer
		tw := tar.NewWriter(&buf)
		if err := tw.WriteHeader(&tar.Header{Name: "html/sor-integration.txt", Mode: 0644, Size: int64(len(value)), Uid: 33, Gid: 33}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(value)); err != nil {
			t.Fatal(err)
		}
		if err := tw.Close(); err != nil {
			t.Fatal(err)
		}
		if err := wp.CopyIn(ctx, site.WebID, &buf); err != nil {
			t.Fatal(err)
		}
	}
	writeMarker("before-backup")
	bm := &backups.Manager{Cfg: cfg, Store: store, WP: wp}
	backup, err := bm.Create(ctx, site.ID, integrationID(t))
	if err != nil {
		t.Fatal("backup:", err)
	}
	cli(siteID, "option", "update", "blogdescription", "after-backup")
	writeMarker("after-backup")
	restored, err := bm.Restore(ctx, backup.ID, site.ID, site.Domain, restoreID)
	if err != nil {
		t.Fatal("restore:", err)
	}
	if restored.WebID == site.WebID || restored.DBID == site.DBID || restored.SafetyBackup == nil || restored.Budget != site.Budget {
		t.Fatal("restore did not retain the safety backup, new resources, and budget")
	}
	if got := cli(restoreID, "option", "get", "blogdescription"); got != "before-backup" {
		t.Fatalf("database restore: %q", got)
	}
	archive, err := dm.API.CopyFromContainer(ctx, restored.WebID, client.CopyFromContainerOptions{SourcePath: "/var/www/html/sor-integration.txt"})
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Content.Close()
	tr := tar.NewReader(archive.Content)
	if _, err := tr.Next(); err != nil {
		t.Fatal(err)
	}
	marker, err := io.ReadAll(io.LimitReader(tr, 100))
	if err != nil || string(marker) != "before-backup" {
		t.Fatal("file contents were not restored")
	}
	for _, id := range []string{site.WebID, restored.WebID, restored.DBID} {
		v, err := dm.API.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
		if err != nil || v.Container.State == nil || v.Container.State.Running != (id != site.WebID) {
			t.Fatalf("unexpected state after cutover for %s", id)
		}
	}
	rows, err := store.Q.Recoveries(ctx)
	if err != nil || len(rows) != 0 {
		t.Fatal("maintenance recovery journal was not cleared")
	}
	if verifyEdge != nil {
		verifyEdge(wp, restored)
	}
	if os.Getenv("SOR_LIFECYCLE_INTEGRATION") == "1" {
		testSiteDeletion(t, wp, restored, bm)
	}
	t.Log("real Docker install, authorized actions/logs, file+database backup, safety backup and staged restoration passed")
}
