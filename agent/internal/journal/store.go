package journal

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"net/url"
	"os"
	"path/filepath"
	"sor.hosting/agent/internal/journal/sqlc"
	"sor.hosting/agent/migrations"
	"sor.hosting/shared/agentapi"
	"sor.hosting/shared/secretbox"
	"time"
)

type Store struct {
	DB  *sql.DB
	Q   *sqlc.Queries
	Key []byte
}

func Open(ctx context.Context, dir string, key []byte) (*Store, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	u := url.URL{Scheme: "file", Path: filepath.Join(dir, "journal.db"), RawQuery: "_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"}
	db, e := sql.Open("sqlite", u.String())
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	b, e := migrations.Files.ReadFile("000001_agent.sql")
	if e != nil {
		db.Close()
		return nil, e
	}
	tx, e := db.BeginTx(ctx, nil)
	if e != nil {
		db.Close()
		return nil, e
	}
	defer tx.Rollback()
	var v int
	e = tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v)
	if e == nil && v > 2 {
		e = errors.New("unsupported journal version")
	}
	if e == nil && v == 0 {
		_, e = tx.ExecContext(ctx, string(b))
		if e == nil {
			_, e = tx.ExecContext(ctx, "PRAGMA user_version=1")
		}
	}
	if e == nil && v < 2 {
		var audit []byte
		audit, e = migrations.Files.ReadFile("000002_audit.sql")
		if e == nil {
			_, e = tx.ExecContext(ctx, string(audit))
		}
		if e == nil {
			_, e = tx.ExecContext(ctx, "PRAGMA user_version=2")
		}
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		db.Close()
		return nil, e
	}
	return &Store{db, sqlc.New(db), key}, nil
}
func public(v sqlc.Operation) agentapi.Operation {
	t, _ := time.Parse(time.RFC3339, v.UpdatedAt)
	return agentapi.Operation{ID: v.ID, Kind: v.Kind, State: v.State, Phase: v.Phase, ErrorCode: v.ErrorCode, Result: json.RawMessage(v.Result), UpdatedAt: t}
}
func (s *Store) Reserve(ctx context.Context, r agentapi.OperationRequest) (agentapi.Operation, error) {
	if e := r.Validate(); e != nil {
		return agentapi.Operation{}, e
	}
	b, _ := json.Marshal(r)
	h := hmac.New(sha256.New, s.Key)
	h.Write(b)
	digest := hex.EncodeToString(h.Sum(nil))
	encrypted, e := secretbox.Seal(s.Key, b)
	if e != nil {
		return agentapi.Operation{}, e
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return agentapi.Operation{}, e
	}
	defer tx.Rollback()
	q := s.Q.WithTx(tx)
	_, e = q.Reserve(ctx, sqlc.ReserveParams{ID: r.ID, Kind: r.Kind, ResourceID: r.ResourceID, Digest: digest, State: "queued", Phase: "Pendiente", Request: encrypted, Result: "{}", ErrorCode: "", UpdatedAt: time.Now().UTC().Format(time.RFC3339)})
	if e != nil {
		return agentapi.Operation{}, e
	}
	v, e := q.Operation(ctx, r.ID)
	if e != nil {
		return agentapi.Operation{}, e
	}
	if v.Digest != digest {
		return agentapi.Operation{}, fmt.Errorf("idempotency conflict")
	}
	actor := r.ActorID
	if actor == "" {
		actor = "panel"
	}
	if _, e = tx.ExecContext(ctx, "INSERT OR IGNORE INTO audit_events(operation_id,action,resource_id,actor_id,outcome,created_at) VALUES(?,?,?,?,?,?)", r.ID, r.Kind, r.ResourceID, actor, "queued", time.Now().UTC().Format(time.RFC3339)); e != nil {
		return agentapi.Operation{}, e
	}
	if e = tx.Commit(); e != nil {
		return agentapi.Operation{}, e
	}
	return public(v), nil
}
func (s *Store) Operation(ctx context.Context, id string) (agentapi.Operation, error) {
	v, e := s.Q.Operation(ctx, id)
	return public(v), e
}
func (s *Store) Update(ctx context.Context, id, state, phase string, result any, code string) error {
	v, e := s.Q.Operation(ctx, id)
	if e != nil {
		return e
	}
	b, e := json.Marshal(result)
	if e != nil {
		return e
	}
	request := v.Request
	if state == "succeeded" || state == "failed" || state == "interrupted" {
		request = ""
	}
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	e = s.Q.WithTx(tx).UpdateOperation(ctx, sqlc.UpdateOperationParams{State: state, Phase: phase, Request: request, Result: string(b), ErrorCode: code, UpdatedAt: time.Now().UTC().Format(time.RFC3339), ID: id})
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, "INSERT OR IGNORE INTO audit_events(operation_id,action,resource_id,actor_id,outcome,created_at) SELECT ?,?,?,COALESCE((SELECT actor_id FROM audit_events WHERE operation_id=? LIMIT 1),'panel'),?,?", id, v.Kind, v.ResourceID, id, state, time.Now().UTC().Format(time.RFC3339))
	if e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) Queued(ctx context.Context) ([]agentapi.OperationRequest, error) {
	rows, e := s.Q.Pending(ctx)
	if e != nil {
		return nil, e
	}
	out := []agentapi.OperationRequest{}
	for _, r := range rows {
		b, e := secretbox.Open(s.Key, r.Request)
		if e != nil {
			return nil, e
		}
		var v agentapi.OperationRequest
		if e = agentapi.Decode(b, &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (s *Store) Site(ctx context.Context, id string) (v agentapi.SiteResources, e error) {
	r, e := s.Q.Site(ctx, id)
	if e == nil {
		e = json.Unmarshal([]byte(r.Metadata), &v)
	}
	return
}
func (s *Store) Sites(ctx context.Context) ([]agentapi.SiteResources, error) {
	rows, e := s.Q.Sites(ctx)
	if e != nil {
		return nil, e
	}
	out := []agentapi.SiteResources{}
	for _, r := range rows {
		var v agentapi.SiteResources
		if e = json.Unmarshal([]byte(r.Metadata), &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (s *Store) SaveSite(ctx context.Context, v agentapi.SiteResources) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return s.Q.SaveSite(ctx, sqlc.SaveSiteParams{ID: v.ID, Metadata: string(b)})
}
func (s *Store) Domain(ctx context.Context, id string) (v agentapi.DomainResult, e error) {
	r, e := s.Q.Domain(ctx, id)
	if e == nil {
		e = json.Unmarshal([]byte(r.Metadata), &v)
	}
	return
}
func (s *Store) Domains(ctx context.Context, id string) ([]agentapi.DomainResult, error) {
	rows, e := s.Q.Domains(ctx, id)
	if e != nil {
		return nil, e
	}
	out := []agentapi.DomainResult{}
	for _, r := range rows {
		var v agentapi.DomainResult
		if e = json.Unmarshal([]byte(r.Metadata), &v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, nil
}
func (s *Store) SaveDomain(ctx context.Context, v agentapi.DomainResult) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return s.Q.SaveDomain(ctx, sqlc.SaveDomainParams{ID: v.ID, SiteID: v.SiteID, Metadata: string(b)})
}
func (s *Store) Backup(ctx context.Context, id string) (v agentapi.Backup, e error) {
	r, e := s.Q.Backup(ctx, id)
	if e == nil {
		e = json.Unmarshal([]byte(r.Metadata), &v)
	}
	return
}
func (s *Store) SaveBackup(ctx context.Context, v agentapi.Backup) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	return s.Q.SaveBackup(ctx, sqlc.SaveBackupParams{ID: v.ID, SiteID: v.SiteID, Metadata: string(b)})
}
