package backups

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/moby/moby/client"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
	"sor.hosting/agent/internal/config"
	"sor.hosting/agent/internal/journal"
	jsql "sor.hosting/agent/internal/journal/sqlc"
	"sor.hosting/agent/internal/wordpress"
	"sor.hosting/shared/agentapi"
	"time"
)

type Manager struct {
	Cfg   config.Config
	Store *journal.Store
	WP    *wordpress.Manager
}
type Manifest struct {
	Version      int                    `json:"version"`
	Site         agentapi.SiteResources `json:"site"`
	CreatedAt    string                 `json:"created_at"`
	FilesHash    string                 `json:"files_hash"`
	DatabaseHash string                 `json:"database_hash"`
	MariaDBImage string                 `json:"mariadb_image"`
}
type Recovery struct {
	Kind    string                  `json:"kind"`
	Site    agentapi.SiteResources  `json:"site"`
	Running []string                `json:"running"`
	Staging *agentapi.SiteResources `json:"staging,omitempty"`
}

func id() string {
	b := make([]byte, 24)
	if _, e := rand.Read(b); e != nil {
		panic("secure random unavailable")
	}
	return hex.EncodeToString(b)
}
func (m *Manager) record(ctx context.Context, id string, recovery Recovery) error {
	b, e := json.Marshal(recovery)
	if e != nil {
		return e
	}
	return m.Store.Q.SaveRecovery(ctx, jsql.SaveRecoveryParams{ID: id, SiteID: recovery.Site.ID, Metadata: string(b)})
}
func (m *Manager) running(ctx context.Context, s agentapi.SiteResources) ([]string, error) {
	out := []string{}
	db, e := m.WP.Docker.API.ContainerInspect(ctx, s.DBID, client.ContainerInspectOptions{})
	if e != nil || db.Container.State == nil || !db.Container.State.Running {
		return nil, errors.New("database must be running")
	}
	for _, id := range append([]string{s.WebID}, s.Writers...) {
		if _, e = m.WP.Docker.Authorized(ctx, id); e != nil {
			return nil, e
		}
		v, e := m.WP.Docker.API.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
		if e != nil {
			return nil, e
		}
		if v.Container.State != nil && v.Container.State.Running {
			out = append(out, id)
		}
	}
	return out, nil
}
func (m *Manager) resume(ctx context.Context, r Recovery) error {
	for _, id := range r.Running {
		if e := m.WP.Docker.RawAction(ctx, id, "start"); e != nil {
			return e
		}
	}
	return nil
}
func (m *Manager) Recover(ctx context.Context) error {
	rows, e := m.Store.Q.Recoveries(ctx)
	if e != nil {
		return e
	}
	for _, row := range rows {
		var r Recovery
		if e = json.Unmarshal([]byte(row.Metadata), &r); e != nil {
			return e
		}
		if r.Kind == "site.delete" {
			continue
		}
		if r.Kind == "restore" {
			if e = m.rollback(ctx, r); e != nil {
				return e
			}
		} else if e = m.resume(ctx, r); e != nil {
			return e
		}
		if e = m.Store.Q.DeleteRecovery(ctx, row.ID); e != nil {
			return e
		}
	}
	return nil
}
func freeSpace(path string, required uint64) error {
	var st unix.Statfs_t
	if e := unix.Statfs(path, &st); e != nil {
		return e
	}
	if st.Bavail*uint64(st.Bsize) < required {
		return errors.New("insufficient disk space")
	}
	return nil
}
func (m *Manager) Create(ctx context.Context, siteID, backupID string) (backup agentapi.Backup, err error) {
	if !agentapi.OpaqueID.MatchString(backupID) {
		return backup, errors.New("invalid ID")
	}
	s, e := m.Store.Site(ctx, siteID)
	if e != nil {
		return backup, e
	}
	if !s.Mutable() {
		return backup, errors.New("read-only site")
	}
	if s.SecretRef == "" || s.DBID == "" {
		return backup, errors.New("site missing backup configuration")
	}
	if e = freeSpace(m.Cfg.BackupDir, 256<<20); e != nil {
		return backup, e
	}
	running, e := m.running(ctx, s)
	if e != nil {
		return backup, e
	}
	recovery := Recovery{Kind: "backup", Site: s, Running: running}
	if e = m.record(ctx, backupID, recovery); e != nil {
		return backup, e
	}
	defer func() {
		recoveryCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if e := m.resume(recoveryCtx, recovery); e != nil {
			err = errors.New("RECOVERY_REQUIRED")
			return
		}
		if e := m.Store.Q.DeleteRecovery(recoveryCtx, backupID); e != nil {
			err = e
		}
	}()
	for _, id := range running {
		if e = m.WP.Docker.RawAction(ctx, id, "stop"); e != nil {
			return backup, e
		}
	}
	dir, e := os.MkdirTemp(m.Cfg.BackupDir, ".creating-")
	if e != nil {
		return backup, e
	}
	defer os.RemoveAll(dir)
	for name, write := range map[string]func(io.Writer) error{"database.sql": func(w io.Writer) error { return m.WP.Dump(ctx, s, w) }, "files.tar": func(w io.Writer) error { return m.WP.CopyOut(ctx, s.WebID, w) }} {
		f, e := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return backup, e
		}
		e = write(&diskWriter{File: f, Root: dir})
		if x := f.Sync(); e == nil {
			e = x
		}
		if x := f.Close(); e == nil {
			e = x
		}
		if e != nil {
			return backup, e
		}
	}
	f, e := os.Open(filepath.Join(dir, "files.tar"))
	if e != nil {
		return backup, e
	}
	e = ValidateTar(f)
	f.Close()
	if e != nil {
		return backup, e
	}
	filesHash, filesSize, e := checksum(filepath.Join(dir, "files.tar"))
	if e != nil {
		return backup, e
	}
	dbHash, dbSize, e := checksum(filepath.Join(dir, "database.sql"))
	if e != nil {
		return backup, e
	}
	if filesSize+dbSize > maxArchive-(1<<20) {
		return backup, errors.New("backup size limit exceeded")
	}
	manifest := Manifest{Version: 1, Site: s, CreatedAt: time.Now().UTC().Format(time.RFC3339), FilesHash: filesHash, DatabaseHash: dbHash, MariaDBImage: m.Cfg.MariaDBImage}
	b, e := json.Marshal(manifest)
	if e != nil {
		return backup, e
	}
	if e = os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0600); e != nil {
		return backup, e
	}
	archive := filepath.Join(dir, "backup.tar.gz")
	if e = pack(dir, archive); e != nil {
		return backup, e
	}
	sum, size, e := checksum(archive)
	if e != nil {
		return backup, e
	}
	target := filepath.Join(m.Cfg.BackupDir, backupID+".tar.gz")
	if _, e = os.Lstat(target); e == nil {
		return backup, errors.New("backup already exists")
	}
	if e = os.Rename(archive, target); e != nil {
		return backup, e
	}
	backup = agentapi.Backup{ID: backupID, SiteID: siteID, Version: 1, SizeBytes: size, Checksum: sum, CreatedAt: manifest.CreatedAt}
	if e = m.Store.SaveBackup(ctx, backup); e != nil {
		return backup, e
	}
	return backup, nil
}
func (m *Manager) Download(ctx context.Context, backupID string) (*os.File, agentapi.Backup, error) {
	b, e := m.Store.Backup(ctx, backupID)
	if e != nil {
		return nil, b, e
	}
	if !agentapi.OpaqueID.MatchString(b.ID) {
		return nil, b, errors.New("invalid backup ID")
	}
	path := filepath.Join(m.Cfg.BackupDir, b.ID+".tar.gz")
	st, e := os.Lstat(path)
	if e != nil || !st.Mode().IsRegular() {
		return nil, b, errors.New("invalid backup file")
	}
	f, e := os.Open(path)
	return f, b, e
}
