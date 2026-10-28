package deletion

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/containerd/errdefs"
	"github.com/moby/moby/client"
	"os"
	"path/filepath"
	"sor.hosting/agent/internal/journal/sqlc"
	"sor.hosting/shared/agentapi"
	"time"
)

func (m *Manager) save(ctx context.Context, p Plan) error {
	b, e := json.Marshal(p)
	if e != nil {
		return e
	}
	return m.WP.Store.Q.SaveRecovery(ctx, sqlc.SaveRecoveryParams{ID: p.ID, SiteID: p.Site.ID, Metadata: string(b)})
}
func (m *Manager) Delete(ctx context.Context, r agentapi.OperationRequest) (agentapi.DeleteResult, error) {
	s, e := m.WP.Store.Site(ctx, r.ResourceID)
	if s.Lifecycle == "" {
		s.Lifecycle = "present"
	}
	if s.Management == "" {
		s.Management = "managed"
	}
	result := agentapi.DeleteResult{Site: s}
	if e != nil {
		return result, e
	}
	op, opErr := m.WP.Store.Q.Operation(ctx, r.ID)
	if opErr != nil || op.Kind != "site.delete" || op.ResourceID != s.ID {
		return result, reject()
	}
	var in agentapi.DeleteInput
	if e = agentapi.Decode(r.Payload, &in); e != nil {
		return result, e
	}
	result.KeepBackups = in.KeepBackups
	if in.ConfirmDomain != s.Domain || s.External() {
		return result, reject()
	}
	if s.Lifecycle == "deleted" {
		if s.DeleteKeepBackups != in.KeepBackups {
			return result, reject()
		}
		return result, nil
	}
	p, e := m.preflight(ctx, s)
	if e != nil {
		return result, e
	}
	p.ID = r.ID
	p.KeepBackups = in.KeepBackups
	if e = m.save(ctx, p); e != nil {
		return result, e
	}
	if e = m.prepare(ctx, &p); e != nil {
		recovery, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if x := m.rollback(recovery, p); x != nil {
			result.Site.Lifecycle = "deleting"
			return result, ErrPending
		}
		result.Site = p.Site
		result.Site.Lifecycle = "present"
		return result, e
	}
	result.Site = p.Site
	if e = m.forward(ctx, p); e != nil {
		result.Site.Lifecycle = "deleting"
		return result, ErrPending
	}
	result.Site.Lifecycle = "deleted"
	result.Site.DeleteKeepBackups = p.KeepBackups
	if !p.KeepBackups {
		result.Site.SafetyBackup = nil
	}
	return result, nil
}
func (m *Manager) prepare(ctx context.Context, p *Plan) error {
	safety, e := m.Backups.Create(ctx, p.Site.ID, p.ID+"-safety")
	if e != nil {
		return e
	}
	p.Site.SafetyBackup = &safety
	// Recheck after the safety backup, before suspension or irreversible steps.
	if _, e = m.preflight(ctx, p.Site); e != nil {
		return e
	}
	p.Suspended = true
	if e = m.save(ctx, *p); e != nil {
		return e
	}
	site := p.Site
	site.Lifecycle = "deleting"
	if e = m.WP.Store.SaveSite(ctx, site); e != nil {
		return e
	}
	if e = m.WP.Nginx.SetDeletionFiles(ctx, p.Files, false); e != nil {
		return e
	}
	for _, g := range p.Groups {
		for _, id := range g.Containers {
			if e = m.checkContainer(ctx, p.Site.ID, g.Ref, id); e != nil {
				return e
			}
			if e = m.WP.Docker.RawAction(ctx, id, "stop"); e != nil {
				return e
			}
		}
	}
	p.Committed = true
	return m.save(ctx, *p)
}
func (m *Manager) rollback(ctx context.Context, p Plan) error {
	if !p.Suspended {
		return m.finalize(ctx, p, false)
	}
	if e := m.WP.Nginx.SetDeletionFiles(ctx, p.Files, true); e != nil {
		return e
	}
	for _, id := range p.Running {
		ref := ""
		for _, g := range p.Groups {
			for _, candidate := range g.Containers {
				if candidate == id {
					ref = g.Ref
				}
			}
		}
		if e := m.checkContainer(ctx, p.Site.ID, ref, id); e != nil {
			return e
		}
		if e := m.WP.Docker.RawAction(ctx, id, "start"); e != nil {
			return e
		}
	}
	return m.finalize(ctx, p, false)
}
func (m *Manager) checkContainer(ctx context.Context, site, ref, id string) error {
	v, e := m.WP.Docker.API.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if e != nil {
		return e
	}
	if v.Container.ID != id || v.Container.Config == nil {
		return reject()
	}
	c := v.Container.Config
	if c.Labels["sor.hosting/site"] != site || c.Labels["sor.hosting/project"] != ref || c.Labels["com.docker.compose.project"] != "sor-"+ref || m.WP.Docker.IsProtected(id, c.Image, c.Labels, v.Container.Name) {
		return reject()
	}
	return nil
}
func (m *Manager) forward(ctx context.Context, p Plan) error {
	if !p.Committed {
		return errors.New("deletion has not committed")
	}
	if e := m.WP.CheckProxy(ctx); e != nil {
		return e
	}
	if e := m.WP.Nginx.SetDeletionFiles(ctx, p.Files, false); e != nil {
		return e
	}
	for _, g := range p.Groups {
		for _, id := range g.Containers {
			e := m.checkContainer(ctx, p.Site.ID, g.Ref, id)
			if errdefs.IsNotFound(e) {
				continue
			}
			if e != nil {
				return e
			}
			if e = m.WP.Docker.RawAction(ctx, id, "stop"); e != nil {
				return e
			}
			if _, e = m.WP.Docker.API.ContainerRemove(ctx, id, client.ContainerRemoveOptions{}); e != nil && !errdefs.IsNotFound(e) {
				return e
			}
		}
		for _, expected := range g.Volumes {
			v, e := m.WP.Docker.API.VolumeInspect(ctx, expected.Name, client.VolumeInspectOptions{})
			if errdefs.IsNotFound(e) {
				continue
			}
			if e != nil {
				return e
			}
			if v.Volume.CreatedAt != expected.CreatedAt || v.Volume.Labels["com.docker.compose.project"] != "sor-"+g.Ref || v.Volume.Driver != "local" || len(v.Volume.Options) > 0 {
				return reject()
			}
			// Docker itself also refuses a non-force removal if another container uses it.
			if _, e = m.WP.Docker.API.VolumeRemove(ctx, expected.Name, client.VolumeRemoveOptions{}); e != nil && !errdefs.IsNotFound(e) {
				return e
			}
		}
		n, e := m.WP.Docker.API.NetworkInspect(ctx, g.NetworkID, client.NetworkInspectOptions{})
		if !errdefs.IsNotFound(e) {
			if e != nil {
				return e
			}
			if n.Network.Name != g.Network || n.Network.Labels["com.docker.compose.project"] != "sor-"+g.Ref || g.Network == m.WP.Cfg.Nginx.ProxyNetwork || len(n.Network.Containers) > 0 {
				return reject()
			}
			if _, e = m.WP.Docker.API.NetworkRemove(ctx, g.NetworkID, client.NetworkRemoveOptions{}); e != nil && !errdefs.IsNotFound(e) {
				return e
			}
		}
	}
	for _, d := range p.Domains {
		if e := m.WP.Certbot.DeleteOwned(ctx, d); e != nil {
			return e
		}
	}
	if !p.KeepBackups {
		rows, e := m.WP.Store.DB.QueryContext(ctx, "SELECT id FROM backups WHERE site_id=?", p.Site.ID)
		if e != nil {
			return e
		}
		var ids []string
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return e
			}
			ids = append(ids, id)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		for _, id := range ids {
			if !agentapi.OpaqueID.MatchString(id) {
				return reject()
			}
			path := filepath.Join(m.WP.Cfg.BackupDir, id+".tar.gz")
			st, e := os.Lstat(path)
			if os.IsNotExist(e) {
				continue
			}
			if e != nil || !st.Mode().IsRegular() {
				return reject()
			}
			if e = os.Remove(path); e != nil {
				return e
			}
		}
	}
	return m.finalize(ctx, p, true)
}
func (m *Manager) finalize(ctx context.Context, p Plan, deleted bool) error {
	p.Site.Lifecycle = "present"
	state, phase, code := "failed", "Eliminación revertida antes del borrado", "DELETE_ROLLED_BACK"
	if deleted {
		p.Site.Lifecycle = "deleted"
		state = "succeeded"
		phase = "Sitio eliminado"
		code = ""
	}
	p.Site.DeleteKeepBackups = p.KeepBackups
	if deleted && !p.KeepBackups {
		p.Site.SafetyBackup = nil
	}
	b, _ := json.Marshal(p.Site)
	tx, e := m.WP.Store.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.ExecContext(ctx, "UPDATE sites SET metadata=? WHERE id=?", string(b), p.Site.ID); e != nil {
		return e
	}
	if deleted {
		if _, e = tx.ExecContext(ctx, "DELETE FROM domains WHERE site_id=?", p.Site.ID); e != nil {
			return e
		}
	}
	if deleted && !p.KeepBackups {
		if _, e = tx.ExecContext(ctx, "DELETE FROM backups WHERE site_id=?", p.Site.ID); e != nil {
			return e
		}
	}
	// Recovery is removed only after the terminal operation has its durable result.
	result, _ := json.Marshal(agentapi.DeleteResult{Site: p.Site, KeepBackups: p.KeepBackups})
	now := time.Now().UTC().Format(time.RFC3339)
	if _, e = tx.ExecContext(ctx, "UPDATE operations SET state=?,phase=?,request='',result=?,error_code=?,updated_at=? WHERE id=?", state, phase, string(result), code, now, p.ID); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "INSERT OR IGNORE INTO audit_events SELECT id,kind,resource_id,COALESCE((SELECT actor_id FROM audit_events WHERE operation_id=? LIMIT 1),'panel'),?,? FROM operations WHERE id=?", p.ID, state, now, p.ID); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "DELETE FROM recovery WHERE id=?", p.ID); e != nil {
		return e
	}
	return tx.Commit()
}
func (m *Manager) Recover(ctx context.Context) error {
	rows, e := m.WP.Store.Q.Recoveries(ctx)
	if e != nil {
		return e
	}
	for _, row := range rows {
		var p Plan
		if e = json.Unmarshal([]byte(row.Metadata), &p); e != nil {
			return e
		}
		if p.Kind != "site.delete" {
			continue
		}
		if p.ID != row.ID || p.Site.ID != row.SiteID || !agentapi.OpaqueID.MatchString(p.ID) {
			return reject()
		}
		if p.Committed {
			if e = m.forward(ctx, p); e != nil {
				return e
			}
		} else {
			if e = m.rollback(ctx, p); e != nil {
				return e
			}
			p.Site.Lifecycle = "present"
			if e = m.WP.Store.Update(ctx, p.ID, "failed", "Eliminación revertida antes del borrado", agentapi.DeleteResult{Site: p.Site, KeepBackups: p.KeepBackups}, "DELETE_ROLLED_BACK"); e != nil {
				return e
			}
		}
	}
	return nil
}
