package deletion

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"sor.hosting/agent/internal/journal"
	"sor.hosting/agent/internal/nginx"
	"sor.hosting/agent/internal/wordpress"
	"sor.hosting/shared/agentapi"
	"testing"
)

type noOpNginx struct{}

func (noOpNginx) Run(context.Context, string, []string, io.Reader, io.Writer) error { return nil }
func TestRollbackKeepsRecoveryUntilSiteAndOperationCommit(t *testing.T) {
	ctx := context.Background()
	store, e := journal.Open(ctx, t.TempDir(), bytes.Repeat([]byte{1}, 32))
	if e != nil {
		t.Fatal(e)
	}
	defer store.DB.Close()
	s := agentapi.SiteResources{ID: "managed123", Name: "Site", Domain: "one.example", Management: "managed", State: "active", Lifecycle: "deleting"}
	store.SaveSite(ctx, s)
	payload, _ := json.Marshal(agentapi.DeleteInput{ConfirmDomain: s.Domain, KeepBackups: true})
	r := agentapi.OperationRequest{ID: "delete123", Kind: "site.delete", ServerID: "local", ResourceID: s.ID, ActorID: "admin123", Payload: payload}
	if _, e = store.Reserve(ctx, r); e != nil {
		t.Fatal(e)
	}
	m := Manager{WP: &wordpress.Manager{Store: store, Nginx: &nginx.Manager{Dir: t.TempDir(), Webroot: t.TempDir(), Runner: noOpNginx{}}}}
	plan := Plan{Suspended: true, Kind: "site.delete", ID: r.ID, Site: s, KeepBackups: true, Files: map[string][]byte{}}
	if e = m.save(ctx, plan); e != nil {
		t.Fatal(e)
	}
	store.DB.ExecContext(ctx, "CREATE TRIGGER fail_operation BEFORE UPDATE ON operations BEGIN SELECT RAISE(FAIL,'injected'); END")
	if e = m.Recover(ctx); e == nil {
		t.Fatal("expected transaction failure")
	}
	site, _ := store.Site(ctx, s.ID)
	if site.Lifecycle != "deleting" {
		t.Fatal("partial DB commit")
	}
	pending, _ := store.Q.Recoveries(ctx)
	if len(pending) != 1 {
		t.Fatal("recovery lost")
	}
	store.DB.ExecContext(ctx, "DROP TRIGGER fail_operation")
	if e = m.Recover(ctx); e != nil {
		t.Fatal(e)
	}
	site, _ = store.Site(ctx, s.ID)
	op, _ := store.Operation(ctx, r.ID)
	pending, _ = store.Q.Recoveries(ctx)
	if site.Lifecycle != "present" || op.State != "failed" || len(pending) != 0 {
		t.Fatal(site, op, pending)
	}
	var count int
	store.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_events WHERE operation_id=? AND outcome='failed' AND actor_id='admin123'", r.ID).Scan(&count)
	if count != 1 {
		t.Fatal("audit mismatch", count)
	}
}
