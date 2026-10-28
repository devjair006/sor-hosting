//go:build integration

package backups_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moby/moby/client"
	"sor.hosting/agent/internal/backups"
	"sor.hosting/agent/internal/deletion"
	"sor.hosting/agent/internal/wordpress"
	"sor.hosting/shared/agentapi"
)

func testSiteDeletion(t *testing.T, wp *wordpress.Manager, site agentapi.SiteResources, bm *backups.Manager) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	dm := wp.Docker
	store := wp.Store
	manager := &deletion.Manager{WP: wp, Backups: bm}
	request := func(domain string, keep bool) agentapi.OperationRequest {
		t.Helper()
		b, _ := json.Marshal(agentapi.DeleteInput{ConfirmDomain: domain, KeepBackups: keep})
		r := agentapi.OperationRequest{ID: integrationID(t), Kind: "site.delete", ResourceID: site.ID, ServerID: "local", ActorID: "integration-admin", Payload: b}
		if _, e := store.Reserve(ctx, r); e != nil {
			t.Fatal(e)
		}
		if e := store.Update(ctx, r.ID, "running", "deleting", nil, ""); e != nil {
			t.Fatal(e)
		}
		return r
	}
	before, _ := os.ReadFile(filepath.Join(wp.Cfg.NginxDir, site.Domain+".conf"))
	if _, e := manager.Delete(ctx, request("wrong.example", true)); e == nil {
		t.Fatal("wrong domain accepted")
	}
	dm.ProtectedIDs = append(dm.ProtectedIDs, site.WebID)
	if _, e := manager.Delete(ctx, request(site.Domain, true)); e == nil || errors.Is(e, deletion.ErrPending) {
		t.Fatal("protected container accepted")
	}
	dm.ProtectedIDs = dm.ProtectedIDs[:len(dm.ProtectedIDs)-1]
	// A stopped foreign container also protects a referenced volume.
	borrower := "sor-borrower-" + integrationID(t)
	fixtureDocker(t, "create", "--name", borrower, "--mount", "type=volume,source="+site.DBVolume+",target=/shared", "--entrypoint", "/bin/true", wp.Cfg.MariaDBImage)
	if _, e := manager.Delete(ctx, request(site.Domain, true)); e == nil || errors.Is(e, deletion.ErrPending) {
		t.Fatal("shared volume accepted")
	}
	fixtureDocker(t, "rm", borrower)
	ext := agentapi.SiteResources{ID: "external-reference", Name: "External", Domain: "external.example", Management: "external", State: "active", DBID: site.DBID}
	if e := store.SaveSite(ctx, ext); e != nil {
		t.Fatal(e)
	}
	if _, e := manager.Delete(ctx, request(site.Domain, true)); e == nil || errors.Is(e, deletion.ErrPending) {
		t.Fatal("database shared with external accepted")
	}
	if _, e := store.DB.ExecContext(ctx, "DELETE FROM sites WHERE id=?", ext.ID); e != nil {
		t.Fatal(e)
	}
	after, _ := os.ReadFile(filepath.Join(wp.Cfg.NginxDir, site.Domain+".conf"))
	if string(after) != string(before) {
		t.Fatal("preflight changed config")
	}
	// Fail the reversible reload; live containers and Nginx must be restored.
	realRunner := wp.Nginx.Runner
	wp.Nginx.Runner = &edgeFailure{real: realRunner, path: filepath.Join(wp.Cfg.NginxDir, site.Domain+".conf"), phase: "reload"}
	rolledRequest := request(site.Domain, true)
	if _, e := manager.Delete(ctx, rolledRequest); e == nil || errors.Is(e, deletion.ErrPending) {
		t.Fatal("precommit failure did not roll back", e)
	}
	wp.Nginx.Runner = realRunner
	restored, _ := store.Site(ctx, site.ID)
	if !restored.Present() {
		t.Fatal("rollback left site blocked")
	}
	after, _ = os.ReadFile(filepath.Join(wp.Cfg.NginxDir, site.Domain+".conf"))
	if string(after) != string(before) {
		t.Fatal("rollback lost Nginx config")
	}
	v, e := dm.API.ContainerInspect(ctx, site.WebID, client.ContainerInspectOptions{})
	if e != nil || !v.Container.State.Running {
		t.Fatal("rollback failed to resume web", e)
	}
	op, e := store.Operation(ctx, rolledRequest.ID)
	if e != nil || op.State != "failed" {
		t.Fatal("rollback not atomic with operation result", op, e)
	}
	// A private self-signed lineage exercises real Certbot delete without any CA request.
	if wp.Cfg.Certbot.Mode == "docker" {
		seedOwnedCertificate(t, wp, &site)
	}
	original := dm.API
	target, _ := url.Parse("http://docker")
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{DialContext: func(c context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(c, "unix", "/var/run/docker.sock")
	}}
	var fail atomic.Bool
	fail.Store(true)
	var mutations atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			mutations.Add(1)
		}
		if r.Method == "DELETE" && strings.Contains(r.URL.Path, "/networks/") && fail.Swap(false) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(500)
			io.WriteString(w, `{"message":"injected network deletion failure"}`)
			return
		}
		proxy.ServeHTTP(w, r)
	}))
	defer srv.Close()
	api, e := client.New(client.WithHost(srv.URL), client.WithAPIVersion("1.48"))
	if e != nil {
		t.Fatal(e)
	}
	defer api.Close()
	dm.API = api
	defer func() { dm.API = original }()
	keep := os.Getenv("SOR_LIFECYCLE_KEEP_BACKUPS") != "0"
	r := request(site.Domain, keep)
	if _, e = manager.Delete(ctx, r); !errors.Is(e, deletion.ErrPending) {
		t.Fatal("partial deletion must stay resumable", e)
	}
	partial, _ := store.Site(ctx, site.ID)
	if partial.Lifecycle != "deleting" {
		t.Fatal("partial deletion lost lifecycle")
	}
	// Recovery uses the persisted plan, not an in-memory request.
	manager = &deletion.Manager{WP: wp, Backups: bm}
	if e = manager.Recover(ctx); e != nil {
		t.Fatal("recovery:", e)
	}
	deleted, _ := store.Site(ctx, site.ID)
	if deleted.Lifecycle != "deleted" {
		t.Fatal(deleted)
	}
	op, e = store.Operation(ctx, r.ID)
	if e != nil || op.State != "succeeded" {
		t.Fatal(op, e)
	}
	var audits int
	store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events WHERE operation_id=? AND actor_id='integration-admin' AND outcome='succeeded'", r.ID).Scan(&audits)
	if audits != 1 {
		t.Fatal("missing durable agent audit", audits)
	}
	count := mutations.Load()
	if _, e = manager.Delete(ctx, r); e != nil || mutations.Load() != count {
		t.Fatal("idempotent retry changed resources", e)
	}
	for _, id := range []string{site.WebID, site.DBID} {
		if _, e = original.ContainerInspect(ctx, id, client.ContainerInspectOptions{}); e == nil {
			t.Fatal("container survived deletion")
		}
	}
	if _, e = os.Stat(filepath.Join(wp.Cfg.NginxDir, site.Domain+".conf")); !os.IsNotExist(e) {
		t.Fatal("managed config survived deletion", e)
	}
	var backups int
	store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM backups WHERE site_id=?", site.ID).Scan(&backups)
	if keep && backups == 0 || !keep && backups != 0 {
		t.Fatal("backup retention mismatch", keep, backups)
	}
	if wp.Cfg.Nginx.Mode == "docker" {
		if _, e = original.NetworkInspect(ctx, wp.Cfg.Nginx.ProxyNetwork, client.NetworkInspectOptions{}); e != nil {
			t.Fatal("shared proxy network was deleted", e)
		}
		if _, e = os.Stat(filepath.Join(wp.Cfg.Certbot.Directory, "existing-certificate.pem")); e != nil {
			t.Fatal("foreign certificate changed", e)
		}
	}
	t.Log("safe deletion: protected/shared refusal, rollback, partial recovery, audit, retention and idempotence passed")
}
func seedOwnedCertificate(t *testing.T, wp *wordpress.Manager, site *agentapi.SiteResources) {
	t.Helper()
	ctx := context.Background()
	d := *site.PrimaryDomain
	name := "sor-" + d.Hostname
	root := wp.Cfg.Certbot.Directory
	archive := filepath.Join(root, "archive", name)
	live := filepath.Join(root, "live", name)
	os.MkdirAll(archive, 0700)
	os.MkdirAll(live, 0700)
	os.MkdirAll(filepath.Join(root, "renewal"), 0700)
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(5), Subject: pkix.Name{CommonName: d.Hostname}, DNSNames: []string{d.Hostname}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, e := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	private := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	for _, part := range []string{"cert", "chain", "fullchain", "privkey"} {
		data := cert
		if part == "privkey" {
			data = private
		}
		os.WriteFile(filepath.Join(archive, part+"1.pem"), data, 0600)
		os.Symlink("../../archive/"+name+"/"+part+"1.pem", filepath.Join(live, part+".pem"))
	}
	renewal := "version = 5.8.0\narchive_dir = /etc/letsencrypt/archive/" + name + "\ncert = /etc/letsencrypt/live/" + name + "/cert.pem\nprivkey = /etc/letsencrypt/live/" + name + "/privkey.pem\nchain = /etc/letsencrypt/live/" + name + "/chain.pem\nfullchain = /etc/letsencrypt/live/" + name + "/fullchain.pem\n[renewalparams]\nauthenticator = webroot\nserver = http://127.0.0.1:9/directory\n"
	os.WriteFile(filepath.Join(root, "renewal", name+".conf"), []byte(renewal), 0600)
	d.Certificate, e = wp.Certbot.ReadCertificate(d.ID, d.Hostname)
	if e != nil {
		t.Fatal(e)
	}
	if e = wp.Store.SaveDomain(ctx, d); e != nil {
		t.Fatal(e)
	}
	site.PrimaryDomain = &d
	if e = wp.Store.SaveSite(ctx, *site); e != nil {
		t.Fatal(e)
	}
}
