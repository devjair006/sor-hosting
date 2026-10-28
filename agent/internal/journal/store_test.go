package journal

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"sor.hosting/agent/internal/journal/sqlc"
	"sor.hosting/shared/agentapi"
)

func TestGeneratedJournalInventoryQueries(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, t.TempDir(), bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	site := agentapi.SiteResources{ID: "site0001", Name: "Sitio de prueba", Domain: "sitio.example", State: "pending_dns"}
	if err := s.SaveSite(ctx, site); err != nil {
		t.Fatal("save site:", err)
	}
	if got, err := s.Site(ctx, site.ID); err != nil || got.Name != site.Name {
		t.Fatal("get site:", got, err)
	}
	if got, err := s.Sites(ctx); err != nil || len(got) != 1 {
		t.Fatal("list sites:", got, err)
	}
	domain := agentapi.DomainResult{ID: "domain01", SiteID: site.ID, Hostname: site.Domain, DNSState: "pending"}
	if err := s.SaveDomain(ctx, domain); err != nil {
		t.Fatal("save domain:", err)
	}
	if got, err := s.Domain(ctx, domain.ID); err != nil || got.Hostname != site.Domain {
		t.Fatal("get domain:", got, err)
	}
	if got, err := s.Domains(ctx, site.ID); err != nil || len(got) != 1 {
		t.Fatal("list domains:", got, err)
	}
	backup := agentapi.Backup{ID: "backup01", SiteID: site.ID, Version: 1, Checksum: "checksum"}
	if err := s.SaveBackup(ctx, backup); err != nil {
		t.Fatal("save backup:", err)
	}
	if got, err := s.Backup(ctx, backup.ID); err != nil || got.Checksum != backup.Checksum {
		t.Fatal("get backup:", got, err)
	}
	if err := s.Q.SaveRecovery(ctx, sqlc.SaveRecoveryParams{ID: "recover1", SiteID: site.ID, Metadata: "{}"}); err != nil {
		t.Fatal("save recovery:", err)
	}
	if got, err := s.Q.Recoveries(ctx); err != nil || len(got) != 1 {
		t.Fatal("list recovery:", got, err)
	}
	if err := s.Q.DeleteRecovery(ctx, "recover1"); err != nil {
		t.Fatal("delete recovery:", err)
	}
	if got, err := s.Q.Interrupted(ctx); err != nil || len(got) != 0 {
		t.Fatal("list interrupted:", got, err)
	}
	r := agentapi.OperationRequest{ID: "operation1", ServerID: "local", Kind: "container.start", ResourceID: "container1", Payload: json.RawMessage(`{"action":"start"}`)}
	if _, err := s.Reserve(ctx, r); err != nil {
		t.Fatal("reserve:", err)
	}
	if got, err := s.Queued(ctx); err != nil || len(got) != 1 {
		t.Fatal("queued:", got, err)
	}
	if err := s.Update(ctx, r.ID, "running", "Preparando operación", nil, ""); err != nil {
		t.Fatal("start:", err)
	}
	if err := s.Q.InterruptRunning(ctx, "Revisión requerida tras reinicio"); err != nil {
		t.Fatal("interrupt:", err)
	}
	if got, err := s.Q.Interrupted(ctx); err != nil || len(got) != 1 || got[0].Request != "" || got[0].Phase != "Revisión requerida tras reinicio" {
		t.Fatal("interrupted operation:", got, err)
	}
	if err := s.Update(ctx, r.ID, "succeeded", "Completada", map[string]string{"id": r.ResourceID}, ""); err != nil {
		t.Fatal("update:", err)
	}
}
