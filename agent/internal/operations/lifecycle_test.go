package operations

import (
	"bytes"
	"context"
	"encoding/json"
	"sor.hosting/agent/internal/journal"
	"sor.hosting/shared/agentapi"
	"testing"
)

func TestLifecycleRestartStaysNonterminalUntilReconciled(t *testing.T) {
	ctx := context.Background()
	s, e := journal.Open(ctx, t.TempDir(), bytes.Repeat([]byte{1}, 32))
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	for _, kind := range []string{"site.delete", "site.external"} {
		id := map[string]string{"site.delete": "delete123", "site.external": "extern123"}[kind]
		site := agentapi.SiteResources{ID: id + "site", Name: "Site", Domain: id + ".example", State: "active"}
		var payload any = agentapi.DeleteInput{ConfirmDomain: site.Domain, KeepBackups: true}
		if kind == "site.external" {
			site.Management = "external"
			payload = agentapi.ExternalInput{Name: site.Name, Domain: site.Domain}
		}
		if e = s.SaveSite(ctx, site); e != nil {
			t.Fatal(e)
		}
		b, _ := json.Marshal(payload)
		r := agentapi.OperationRequest{ID: id, Kind: kind, ResourceID: site.ID, ServerID: "local", Payload: b}
		if _, e = s.Reserve(ctx, r); e != nil {
			t.Fatal(e)
		}
		if e = s.Update(ctx, id, "running", "Preparing", nil, ""); e != nil {
			t.Fatal(e)
		}
		if e = s.Q.InterruptRunning(ctx, "Restarting"); e != nil {
			t.Fatal(e)
		}
		op, _ := s.Operation(ctx, id)
		if op.State != "running" {
			t.Fatalf("%s exposed a premature terminal state: %s", kind, op.State)
		}
	}
	d := Dispatcher{Store: s}
	if e = d.reconcile(ctx); e != nil {
		t.Fatal(e)
	}
	deleted, _ := s.Operation(ctx, "delete123")
	external, _ := s.Operation(ctx, "extern123")
	if deleted.State != "failed" || external.State != "succeeded" {
		t.Fatal(deleted, external)
	}
}
