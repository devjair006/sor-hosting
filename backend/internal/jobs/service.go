package jobs

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sor.hosting/backend/internal/auth"
	"sor.hosting/backend/internal/database/sqlc"
	"sor.hosting/shared/agentapi"
	"sor.hosting/shared/secretbox"
	"sync"
	"time"
)

var ErrBusy = errors.New("resource busy")
var ErrConflict = errors.New("idempotency conflict")

type Job struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	State      string `json:"state"`
	Phase      string `json:"phase"`
	ErrorCode  string `json:"error_code"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func Public(j sqlc.Job) Job {
	return Job{j.ID, j.Kind, j.ResourceID, j.State, j.Phase, j.ErrorCode, j.CreatedAt, j.UpdatedAt}
}

type Service struct {
	DB    *sql.DB
	Q     *sqlc.Queries
	Agent agentapi.Client
	Key   []byte
	mu    sync.Mutex
}

func (s *Service) mac(b []byte) string {
	h := hmac.New(sha256.New, s.Key)
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}
func (s *Service) normalized(actor, key string, r agentapi.OperationRequest) (agentapi.OperationRequest, string, error) {
	if !agentapi.OpaqueID.MatchString(key) {
		return r, "", errors.New("invalid idempotency key")
	}
	r.ID = ""
	r.ServerID = "local"
	if r.ResourceID == "" {
		r.ResourceID = s.mac([]byte(actor + ":" + key + ":" + r.Kind))[:48]
	}
	b, e := json.Marshal(r)
	if e != nil {
		return r, "", e
	}
	return r, s.mac(b), nil
}
func (s *Service) Existing(ctx context.Context, actor, key string, r agentapi.OperationRequest) (Job, bool, error) {
	_, digest, e := s.normalized(actor, key, r)
	if e != nil {
		return Job{}, false, e
	}
	old, e := s.Q.JobByKey(ctx, sqlc.JobByKeyParams{UserID: actor, IdemKey: key})
	if errors.Is(e, sql.ErrNoRows) {
		return Job{}, false, nil
	}
	if e != nil {
		return Job{}, false, e
	}
	if old.Digest != digest {
		return Job{}, false, ErrConflict
	}
	return Public(old), true, nil
}
func lockKey(r agentapi.OperationRequest) string {
	var p struct {
		SiteID string `json:"site_id"`
	}
	_ = json.Unmarshal(r.Payload, &p)
	if p.SiteID != "" {
		return p.SiteID
	}
	return r.ResourceID
}
func (s *Service) Enqueue(ctx context.Context, actor, key string, r agentapi.OperationRequest) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, digest, e := s.normalized(actor, key, r)
	if e != nil {
		return Job{}, e
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return Job{}, e
	}
	defer tx.Rollback()
	q := s.Q.WithTx(tx)
	old, e := q.JobByKey(ctx, sqlc.JobByKeyParams{UserID: actor, IdemKey: key})
	if e == nil {
		if old.Digest != digest {
			return Job{}, ErrConflict
		}
		return Public(old), nil
	}
	if !errors.Is(e, sql.ErrNoRows) {
		return Job{}, e
	}
	if r.Kind == "site.install" || r.Kind == "site.adopt" || r.Kind == "site.external" {
		var in struct {
			Domain string `json:"domain"`
		}
		_ = json.Unmarshal(r.Payload, &in)
		sites, e := q.ListSites(ctx)
		if e != nil {
			return Job{}, e
		}
		for _, site := range sites {
			if site.Domain == in.Domain {
				return Job{}, ErrConflict
			}
		}
	}
	if r.Kind == "domain.configure" {
		var in agentapi.DomainInput
		if e = json.Unmarshal(r.Payload, &in); e != nil {
			return Job{}, e
		}
		domains, e := q.ListDomains(ctx)
		if e != nil {
			return Job{}, e
		}
		for _, domain := range domains {
			if domain.Hostname == in.Hostname && domain.ID != r.ResourceID {
				return Job{}, ErrConflict
			}
		}
	}
	if r.Kind == "site.delete" {
		site, e := q.Site(ctx, r.ResourceID)
		if e != nil {
			return Job{}, e
		}
		if site.Management == "external" || site.Lifecycle != "present" {
			return Job{}, ErrConflict
		}
	}
	pending, e := q.PendingJobs(ctx)
	if e != nil {
		return Job{}, e
	}
	if len(pending) >= 128 {
		return Job{}, ErrBusy
	}
	for _, p := range pending {
		b, e := secretbox.Open(s.Key, p.Payload)
		if e != nil {
			return Job{}, e
		}
		var prev agentapi.OperationRequest
		if json.Unmarshal(b, &prev) != nil {
			return Job{}, errors.New("invalid stored job")
		}
		if lockKey(prev) == lockKey(r) {
			return Job{}, ErrBusy
		}
	}
	if r.Kind == "site.delete" {
		if _, e = tx.ExecContext(ctx, "UPDATE sites SET lifecycle='deleting' WHERE id=?", r.ResourceID); e != nil {
			return Job{}, e
		}
	}
	r.ID = auth.RandomID()
	if r.Kind == "site.delete" || r.Kind == "site.external" {
		r.ActorID = actor
	}
	if e = r.Validate(); e != nil {
		return Job{}, e
	}
	raw, e := json.Marshal(r)
	if e != nil {
		return Job{}, e
	}
	cipher, e := secretbox.Seal(s.Key, raw)
	if e != nil {
		return Job{}, e
	}
	now := time.Now().UTC().Format(time.RFC3339)
	e = q.CreateJob(ctx, sqlc.CreateJobParams{ID: r.ID, UserID: actor, Kind: r.Kind, ResourceID: r.ResourceID, State: "queued", Phase: "Pendiente", Payload: cipher, IdemKey: key, Digest: digest, Result: "{}", ErrorCode: "", CreatedAt: now, UpdatedAt: now})
	if e != nil {
		return Job{}, e
	}
	if e = q.InsertAudit(ctx, sqlc.InsertAuditParams{ID: auth.RandomID(), UserID: actor, Action: r.Kind, ResourceID: r.ResourceID, Outcome: "queued", CreatedAt: now}); e != nil {
		return Job{}, e
	}
	if e = tx.Commit(); e != nil {
		return Job{}, e
	}
	return Job{r.ID, r.Kind, r.ResourceID, "queued", "Pendiente", "", now, now}, nil
}
func (s *Service) Run(ctx context.Context) error {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
			s.reconcile(ctx)
		}
	}
}
func (s *Service) reconcile(ctx context.Context) {
	rows, e := s.Q.PendingJobs(ctx)
	if e != nil {
		return
	}
	for _, j := range rows {
		if ctx.Err() != nil {
			return
		}
		raw, e := secretbox.Open(s.Key, j.Payload)
		if e != nil {
			s.finish(ctx, j, agentapi.Operation{State: "interrupted", Phase: "No se pudo recuperar el trabajo", ErrorCode: "RECOVERY_REQUIRED"})
			continue
		}
		var r agentapi.OperationRequest
		if json.Unmarshal(raw, &r) != nil {
			continue
		}
		var op agentapi.Operation
		if j.State == "queued" {
			op, e = s.Agent.Submit(ctx, r)
		} else {
			op, e = s.Agent.Operation(ctx, j.ID)
		}
		if e != nil {
			continue
		}
		s.finish(ctx, j, op)
	}
}
func (s *Service) finish(ctx context.Context, j sqlc.Job, o agentapi.Operation) {
	if o.State != "queued" && o.State != "running" && o.State != "succeeded" && o.State != "failed" && o.State != "interrupted" {
		return
	}
	if o.State == j.State && o.Phase == j.Phase {
		return
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return
	}
	defer tx.Rollback()
	q := s.Q.WithTx(tx)
	payload := j.Payload
	terminal := o.State == "succeeded" || o.State == "failed" || o.State == "interrupted"
	if terminal {
		payload = ""
	}
	result := o.Result
	if len(result) == 0 {
		result = json.RawMessage(`{}`)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	persistResult := o.State == "succeeded"
	if (o.State == "failed" || o.State == "interrupted") && (j.Kind == "site.install" || j.Kind == "backup.restore") {
		var site agentapi.SiteResources
		if json.Unmarshal(result, &site) == nil && agentapi.OpaqueID.MatchString(site.ID) {
			persistResult = true
		}
	}
	if j.Kind == "site.delete" {
		var resultSite agentapi.DeleteResult
		if json.Unmarshal(result, &resultSite) == nil && resultSite.Site.ID == j.ResourceID {
			persistResult = true
		} else if o.State == "failed" {
			if _, e = tx.ExecContext(ctx, "UPDATE sites SET lifecycle='present' WHERE id=? AND lifecycle='deleting'", j.ResourceID); e != nil {
				return
			}
		}
	}
	if persistResult {
		if e = s.persist(ctx, q, j.Kind, result, now); e != nil {
			_ = tx.Rollback()
			s.syncFailure(ctx, j, result, now)
			return
		}
	}
	e = q.UpdateJob(ctx, sqlc.UpdateJobParams{State: o.State, Phase: o.Phase, Payload: payload, Result: string(result), ErrorCode: o.ErrorCode, UpdatedAt: now, ID: j.ID})
	if e != nil {
		return
	}
	if terminal {
		e = q.InsertAudit(ctx, sqlc.InsertAuditParams{ID: auth.RandomID(), UserID: j.UserID, Action: j.Kind, ResourceID: j.ResourceID, Outcome: o.State, CreatedAt: now})
		if e != nil {
			return
		}
	}
	_ = tx.Commit()
}
func (s *Service) persist(ctx context.Context, q *sqlc.Queries, kind string, data []byte, now string) error {
	switch kind {
	case "site.install", "site.adopt", "site.external", "backup.restore":
		var v agentapi.SiteResources
		if e := json.Unmarshal(data, &v); e != nil {
			return e
		}
		if v.ID == "" {
			return errors.New("missing site")
		}
		old, e := q.Site(ctx, v.ID)
		if e == nil {
			e = q.UpdateSite(ctx, sqlc.UpdateSiteParams{State: v.State, Metadata: string(data), UpdatedAt: now, ID: old.ID})
		} else if errors.Is(e, sql.ErrNoRows) {
			e = q.CreateSite(ctx, sqlc.CreateSiteParams{ID: v.ID, ServerID: "local", Name: v.Name, Domain: v.Domain, State: v.State, Metadata: string(data), CreatedAt: now, UpdatedAt: now})
		}
		if e != nil {
			return e
		}
		management := v.Management
		if management == "" {
			management = "managed"
		}
		lifecycle := v.Lifecycle
		if lifecycle == "" {
			lifecycle = "present"
		}
		if e = q.SetSiteManagement(ctx, sqlc.SetSiteManagementParams{Management: management, Lifecycle: lifecycle, ID: v.ID}); e != nil {
			return e
		}
		if e = q.DeleteSiteContainers(ctx, v.ID); e != nil {
			return e
		}
		if v.External() {
			return nil
		}
		for role, ids := range map[string][]string{"web": {v.WebID}, "database": {v.DBID}, "writer": v.Writers} {
			for _, id := range ids {
				if id == "" {
					continue
				}
				if e = q.BindContainer(ctx, sqlc.BindContainerParams{SiteID: v.ID, ContainerID: id, Role: role}); e != nil {
					return e
				}
			}
		}
		if v.PrimaryDomain != nil {
			b, _ := json.Marshal(v.PrimaryDomain)
			if e = s.persist(ctx, q, "domain.configure", b, now); e != nil {
				return e
			}
		}
		if v.SafetyBackup != nil {
			b, _ := json.Marshal(v.SafetyBackup)
			if e = s.persist(ctx, q, "backup.create", b, now); e != nil {
				return e
			}
		}
		return nil
	case "site.delete":
		var v agentapi.DeleteResult
		if e := json.Unmarshal(data, &v); e != nil {
			return e
		}
		if v.Site.SafetyBackup != nil {
			raw, _ := json.Marshal(v.Site.SafetyBackup)
			if e := s.persist(ctx, q, "backup.create", raw, now); e != nil {
				return e
			}
		}
		if v.Site.Lifecycle != "deleted" {
			meta, _ := json.Marshal(v.Site)
			if e := q.UpdateSite(ctx, sqlc.UpdateSiteParams{ID: v.Site.ID, State: v.Site.State, Metadata: string(meta), UpdatedAt: now}); e != nil {
				return e
			}
			return q.SetSiteManagement(ctx, sqlc.SetSiteManagementParams{ID: v.Site.ID, Management: "managed", Lifecycle: v.Site.Lifecycle})
		}
		if e := q.ClearSiteCertificates(ctx, v.Site.ID); e != nil {
			return e
		}
		if e := q.ClearSiteDomains(ctx, v.Site.ID); e != nil {
			return e
		}
		if e := q.DeleteSiteContainers(ctx, v.Site.ID); e != nil {
			return e
		}
		if !v.KeepBackups {
			if e := q.ClearSiteBackups(ctx, v.Site.ID); e != nil {
				return e
			}
		}
		meta, _ := json.Marshal(v.Site)
		return q.CompleteSiteDeletion(ctx, sqlc.CompleteSiteDeletionParams{Metadata: string(meta), Domain: "deleted-" + v.Site.ID + ".invalid", UpdatedAt: now, ID: v.Site.ID})
	case "domain.configure", "certificate.issue", "certificate.renew":
		var v agentapi.DomainResult
		if e := json.Unmarshal(data, &v); e != nil {
			return e
		}
		_, e := q.Domain(ctx, v.ID)
		if errors.Is(e, sql.ErrNoRows) {
			e = q.CreateDomain(ctx, sqlc.CreateDomainParams{ID: v.ID, SiteID: v.SiteID, Hostname: v.Hostname, DnsState: v.DNSState, Metadata: string(data), CreatedAt: now})
		} else if e == nil {
			e = q.UpdateDomain(ctx, sqlc.UpdateDomainParams{DnsState: v.DNSState, Metadata: string(data), ID: v.ID})
		}
		if e != nil {
			return e
		}
		if v.Certificate != nil {
			b, _ := json.Marshal(v.Certificate)
			e = q.SaveCertificate(ctx, sqlc.SaveCertificateParams{ID: v.Certificate.ID, DomainID: v.ID, CertName: v.Certificate.CertName, ExpiresAt: v.Certificate.ExpiresAt, Metadata: string(b)})
			if e != nil {
				return e
			}
		}
		site, e := q.Site(ctx, v.SiteID)
		if e == nil && site.Domain == v.Hostname {
			var meta agentapi.SiteResources
			if json.Unmarshal([]byte(site.Metadata), &meta) != nil {
				return errors.New("invalid site metadata")
			}
			meta.PrimaryDomain = &v
			if v.DNSState != "matched" {
				meta.State = "pending_dns"
			} else if v.Certificate == nil {
				meta.State = "pending_ssl"
			} else {
				meta.State = "active"
			}
			b, _ := json.Marshal(meta)
			return q.UpdateSite(ctx, sqlc.UpdateSiteParams{State: meta.State, Metadata: string(b), UpdatedAt: now, ID: site.ID})
		}
	case "backup.create":
		var v agentapi.Backup
		if e := json.Unmarshal(data, &v); e != nil {
			return e
		}
		return q.SaveBackup(ctx, sqlc.SaveBackupParams{ID: v.ID, SiteID: v.SiteID, State: "ready", SizeBytes: v.SizeBytes, Checksum: v.Checksum, Metadata: string(data), CreatedAt: v.CreatedAt})
	}
	return nil
}

func (s *Service) syncFailure(ctx context.Context, j sqlc.Job, result []byte, now string) {
	state, phase, payload := "interrupted", "El recurso necesita sincronización y revisión", ""
	if j.Kind == "site.delete" {
		state = "running"
		phase = "Sincronizando eliminación con el agente"
		payload = j.Payload
	}
	_ = s.Q.UpdateJob(ctx, sqlc.UpdateJobParams{State: state, Phase: phase, Payload: payload, Result: string(result), ErrorCode: "SYNC_REQUIRED", UpdatedAt: now, ID: j.ID})
}
