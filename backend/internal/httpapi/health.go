package httpapi

import (
	"context"
	"encoding/json"
	"sor.hosting/backend/internal/database/sqlc"
	"sor.hosting/backend/internal/monitoring"
	"sor.hosting/shared/agentapi"
	"sync"
	"time"
)

type siteView struct {
	sqlc.Site
	Availability *bool `json:"availability"`
	CanDelete    bool  `json:"can_delete"`
	CanBackup    bool  `json:"can_backup"`
}

func (a *App) view(s sqlc.Site) siteView {
	var meta agentapi.SiteResources
	_ = json.Unmarshal([]byte(s.Metadata), &meta)
	if s.Lifecycle == "deleted" && meta.Domain != "" {
		s.Domain = meta.Domain
	}
	return siteView{Site: s, Availability: a.Health.Result(s.ID), CanDelete: s.Management != "external" && s.Lifecycle == "present" && !meta.Adopted && len(meta.Writers) == 0, CanBackup: s.Management != "external" && s.Lifecycle == "present"}
}
func (a *App) MonitorSites(ctx context.Context) {
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			rows, e := a.Q.ListSites(ctx)
			if e != nil {
				continue
			}
			var wg sync.WaitGroup
			sem := make(chan struct{}, 4)
			for _, s := range rows {
				if s.State != "active" || s.Lifecycle != "present" {
					continue
				}
				select {
				case sem <- struct{}{}:
				case <-ctx.Done():
					return
				}
				wg.Add(1)
				go func(s sqlc.Site) {
					defer wg.Done()
					defer func() { <-sem }()
					a.Health.Record(s.ID, monitoring.Probe(ctx, s.Domain))
				}(s)
			}
			wg.Wait()
		}
	}
}
