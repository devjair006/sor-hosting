package operations

import (
	"context"
	"github.com/moby/moby/client"
	"sor.hosting/shared/agentapi"
)

// reconcile observes resources after interruption; it never replays an uncertain mutation.
func (d *Dispatcher) reconcile(ctx context.Context) error {
	sites, e := d.Store.Sites(ctx)
	if e != nil {
		return e
	}
	for _, site := range sites {
		if site.State != "provisioning" {
			continue
		}
		containers, e := d.Docker.API.ContainerList(ctx, client.ContainerListOptions{All: true})
		if e != nil {
			return e
		}
		for _, c := range containers.Items {
			if c.Labels["sor.hosting/site"] != site.ID || c.Labels["sor.hosting/project"] != site.ID {
				continue
			}
			switch c.Labels["com.docker.compose.service"] {
			case "web":
				site.WebID = c.ID
			case "db":
				site.DBID = c.ID
			}
		}
		if site.WebID != "" {
			_ = d.WP.RefreshPort(ctx, &site)
		}
		site.State = "failed"
		if e = d.Store.SaveSite(ctx, site); e != nil {
			return e
		}
	}
	rows, e := d.Store.Q.Interrupted(ctx)
	if e != nil {
		return e
	}
	for _, row := range rows {
		if row.Kind == "site.external" {
			site, e := d.Store.Site(ctx, row.ResourceID)
			if e == nil && site.External() {
				if e = d.Store.Update(ctx, row.ID, "succeeded", "Registro externo recuperado", site, ""); e != nil {
					return e
				}
				continue
			}
		}
		if row.Kind == "site.delete" {
			site, e := d.Store.Site(ctx, row.ResourceID)
			if e == nil {
				state, phase, code := "failed", "Eliminación no iniciada o revertida", "DELETE_ROLLED_BACK"
				if site.Lifecycle == "deleted" {
					state = "succeeded"
					phase = "Sitio eliminado"
					code = ""
				} else if site.Lifecycle == "deleting" {
					continue
				} else {
					site.Lifecycle = "present"
				}
				if e = d.Store.Update(ctx, row.ID, state, phase, agentapi.DeleteResult{Site: site, KeepBackups: site.DeleteKeepBackups}, code); e != nil {
					return e
				}
				continue
			}
		}
		var result any = map[string]bool{"reconciled": true}
		siteID := row.ResourceID
		if row.Kind == "backup.restore" {
			backup, e := d.Store.Backup(ctx, row.ResourceID)
			if e != nil {
				continue
			}
			siteID = backup.SiteID
		}
		if row.Kind == "site.install" || row.Kind == "backup.restore" {
			site, e := d.Store.Site(ctx, siteID)
			if e == nil {
				result = site
			}
		}
		if e = d.Store.Update(ctx, row.ID, "interrupted", "Recurso observado; revisa el estado antes de reintentar", result, "RECOVERY_REQUIRED"); e != nil {
			return e
		}
	}
	return nil
}
