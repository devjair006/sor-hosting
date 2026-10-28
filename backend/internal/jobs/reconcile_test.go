package jobs

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sor.hosting/backend/internal/database"
	"sor.hosting/backend/internal/database/sqlc"
	"sor.hosting/shared/agentapi"
	"testing"
)

func fixture(t *testing.T, kind string) (*Service, sqlc.Job) {
	t.Helper()
	ctx := context.Background()
	db, e := database.Open(ctx, filepath.Join(t.TempDir(), "panel.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	if e = database.Migrate(ctx, db); e != nil {
		t.Fatal(e)
	}
	q := sqlc.New(db)
	if e = q.CreateUser(ctx, sqlc.CreateUserParams{ID: "user123456", Email: "admin@example.com", PasswordHash: "fixture", Role: "admin", CreatedAt: "now"}); e != nil {
		t.Fatal(e)
	}
	if e = q.CreateJob(ctx, sqlc.CreateJobParams{ID: "job123456", UserID: "user123456", Kind: kind, ResourceID: "site123456", State: "running", Phase: "pending", Payload: "encrypted", IdemKey: "key123456", Digest: "digest", Result: "{}", CreatedAt: "now", UpdatedAt: "now"}); e != nil {
		t.Fatal(e)
	}
	job, e := q.Job(ctx, "job123456")
	if e != nil {
		t.Fatal(e)
	}
	return &Service{DB: db, Q: q}, job
}
func TestFailedAdoptionDoesNotCreateSite(t *testing.T) {
	s, j := fixture(t, "site.adopt")
	data, _ := json.Marshal(agentapi.SiteResources{ID: "site123456", Name: "Rejected", Domain: "a.example", State: "active"})
	s.finish(context.Background(), j, agentapi.Operation{State: "failed", Phase: "Rejected", ErrorCode: "OPERATION_FAILED", Result: data})
	sites, e := s.Q.ListSites(context.Background())
	if e != nil || len(sites) != 0 {
		t.Fatalf("rejected adoption was persisted: %v %v", sites, e)
	}
	got, e := s.Q.Job(context.Background(), j.ID)
	if e != nil || got.State != "failed" {
		t.Fatal("job did not reach failed state")
	}
}
func TestPersistenceConflictDoesNotLeaveJobRunning(t *testing.T) {
	s, j := fixture(t, "site.adopt")
	ctx := context.Background()
	e := s.Q.CreateSite(ctx, sqlc.CreateSiteParams{ID: "existing123", ServerID: "local", Name: "Existing", Domain: "a.example", State: "active", Metadata: "{}", CreatedAt: "now", UpdatedAt: "now"})
	if e != nil {
		t.Fatal(e)
	}
	data, _ := json.Marshal(agentapi.SiteResources{ID: "site123456", Name: "Duplicate", Domain: "a.example", State: "active"})
	s.finish(ctx, j, agentapi.Operation{State: "succeeded", Phase: "done", Result: data})
	got, e := s.Q.Job(ctx, j.ID)
	if e != nil || got.State != "interrupted" || got.ErrorCode != "SYNC_REQUIRED" {
		t.Fatalf("job stuck or misleading: %+v %v", got, e)
	}
}
func TestIdempotentRetrySurvivesResourceCreation(t *testing.T) {
	s, j := fixture(t, "container.start")
	ctx := context.Background()
	s.finish(ctx, j, agentapi.Operation{State: "failed", Phase: "done"})
	s.Key = make([]byte, 32)
	payload, _ := json.Marshal(agentapi.SiteInput{Name: "Example", Domain: "a.example", WordPressTitle: "Example", AdminEmail: "admin@example.com", AdminPassword: "correct password", Resources: agentapi.Resources{MemoryMB: 512, CPULimit: 1}})
	request := agentapi.OperationRequest{Kind: "site.install", Payload: payload}
	first, e := s.Enqueue(ctx, "user123456", "key-new-123456", request)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Q.CreateSite(ctx, sqlc.CreateSiteParams{ID: first.ResourceID, ServerID: "local", Name: "Example", Domain: "a.example", State: "active", Metadata: "{}", CreatedAt: "now", UpdatedAt: "now"}); e != nil {
		t.Fatal(e)
	}
	second, e := s.Enqueue(ctx, "user123456", "key-new-123456", request)
	if e != nil || second.ID != first.ID {
		t.Fatalf("retry did not recover original job: %v %v", second, e)
	}
	_, found, e := s.Existing(ctx, "user123456", "key-new-123456", request)
	if e != nil || !found {
		t.Fatal("existing lookup should not need an agent")
	}
}

func TestDeletedSiteReleasesDomainAndKeepsBackups(t *testing.T) {
	s, j := fixture(t, "site.delete")
	ctx := context.Background()
	if e := s.Q.CreateSite(ctx, sqlc.CreateSiteParams{ID: j.ResourceID, ServerID: "local", Name: "Site", Domain: "reuse.example", State: "active", Metadata: "{}", CreatedAt: "now", UpdatedAt: "now"}); e != nil {
		t.Fatal(e)
	}
	backup := agentapi.Backup{ID: "backup1234", SiteID: j.ResourceID, Checksum: "sum", Version: 1}
	result, _ := json.Marshal(agentapi.DeleteResult{Site: agentapi.SiteResources{ID: j.ResourceID, Name: "Site", Domain: "reuse.example", State: "active", Lifecycle: "deleted", SafetyBackup: &backup}, KeepBackups: true})
	s.finish(ctx, j, agentapi.Operation{State: "succeeded", Phase: "deleted", Result: result})
	got, _ := s.Q.Site(ctx, j.ResourceID)
	if got.Lifecycle != "deleted" || got.Domain == "reuse.example" {
		t.Fatal(got)
	}
	if _, e := s.Q.Backup(ctx, backup.ID); e != nil {
		t.Fatal("retained backup missing", e)
	}
	if e := s.Q.CreateSite(ctx, sqlc.CreateSiteParams{ID: "replacement01", ServerID: "local", Name: "New", Domain: "reuse.example", State: "active", Metadata: "{}", CreatedAt: "now", UpdatedAt: "now"}); e != nil {
		t.Fatal("domain is not reusable", e)
	}
}
func TestFailedDeletionRestoresPanelLifecycle(t *testing.T) {
	s, j := fixture(t, "site.delete")
	ctx := context.Background()
	s.Q.CreateSite(ctx, sqlc.CreateSiteParams{ID: j.ResourceID, ServerID: "local", Name: "Site", Domain: "one.example", State: "active", Metadata: "{}", CreatedAt: "now", UpdatedAt: "now"})
	s.Q.SetSiteManagement(ctx, sqlc.SetSiteManagementParams{ID: j.ResourceID, Management: "managed", Lifecycle: "deleting"})
	data, _ := json.Marshal(agentapi.DeleteResult{Site: agentapi.SiteResources{ID: j.ResourceID, Domain: "one.example", State: "active", Lifecycle: "present"}})
	s.finish(ctx, j, agentapi.Operation{State: "failed", Phase: "rollback", Result: data})
	got, _ := s.Q.Site(ctx, j.ResourceID)
	if got.Lifecycle != "present" {
		t.Fatal(got)
	}
}

func TestExternalPersistenceNeverGrantsContainerBindings(t *testing.T) {
	s, _ := fixture(t, "site.external")
	ctx := context.Background()
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	for _, id := range []string{"external01", "external02"} {
		v := agentapi.SiteResources{ID: id, Name: id, Domain: id + ".example", State: "active", Management: "external", Lifecycle: "present", WebID: id, DBID: "shared-mariadb"}
		raw, _ := json.Marshal(v)
		if e = s.persist(ctx, s.Q.WithTx(tx), "site.external", raw, "now"); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	var bindings int
	s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM site_containers").Scan(&bindings)
	if bindings != 0 {
		t.Fatal("external reference granted control", bindings)
	}
	rows, e := s.Q.ListSites(ctx)
	if e != nil || len(rows) != 2 || rows[0].Management != "external" || rows[1].Management != "external" {
		t.Fatal(rows, e)
	}
}
