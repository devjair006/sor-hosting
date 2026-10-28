package wordpress

import (
	"context"
	"errors"
	"github.com/moby/moby/client"
	"sor.hosting/shared/agentapi"
	"sor.hosting/shared/domainname"
	"strings"
)

// RegisterExternal records references only. Shared databases are intentionally allowed.
func (m *Manager) RegisterExternal(ctx context.Context, r agentapi.OperationRequest) (agentapi.SiteResources, error) {
	var in agentapi.ExternalInput
	if e := agentapi.Decode(r.Payload, &in); e != nil {
		return agentapi.SiteResources{}, e
	}
	host, e := domainname.Normalize(in.Domain)
	if e != nil || len(strings.TrimSpace(in.Name)) < 1 || len(in.Name) > 80 {
		return agentapi.SiteResources{}, errors.New("invalid external site")
	}
	all, e := m.Store.Sites(ctx)
	if e != nil {
		return agentapi.SiteResources{}, e
	}
	for _, s := range all {
		if s.Present() && (s.ID == r.ResourceID || s.Domain == host) {
			return agentapi.SiteResources{}, errors.New("site already registered")
		}
	}
	for _, id := range []string{in.WebContainerID, in.DatabaseContainerID} {
		if id == "" {
			continue
		}
		if !agentapi.ContainerID.MatchString(id) {
			return agentapi.SiteResources{}, errors.New("exact container ID required")
		}
		v, e := m.Docker.API.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
		if e != nil || v.Container.ID != id {
			return agentapi.SiteResources{}, errors.New("external container not found")
		}
	}
	s := agentapi.SiteResources{ID: r.ResourceID, Name: strings.TrimSpace(in.Name), Domain: host, State: "active", Management: "external", Lifecycle: "present", WebID: in.WebContainerID, DBID: in.DatabaseContainerID}
	return s, m.Store.SaveSite(ctx, s)
}
