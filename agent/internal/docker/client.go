package docker

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"
	"io"
	"regexp"
	"sor.hosting/agent/internal/executil"
	"sor.hosting/agent/internal/inventory"
	"sor.hosting/agent/internal/journal"
	"sor.hosting/shared/agentapi"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Manager struct {
	API            *client.Client
	Store          *journal.Store
	ProtectedIDs   []string
	ProtectedNames []string
	Secrets        func(string) []string
}

func New(s *journal.Store, protected []string) (*Manager, error) {
	c, e := client.New(client.WithHost("unix:///var/run/docker.sock"))
	if e != nil {
		return nil, e
	}
	return &Manager{API: c, Store: s, ProtectedIDs: protected}, nil
}
func (m *Manager) protected(id, image string, labels map[string]string, names ...string) bool {
	for _, ref := range m.ProtectedNames {
		if ref == id {
			return true
		}
		for _, name := range names {
			if strings.TrimPrefix(name, "/") == strings.TrimPrefix(ref, "/") {
				return true
			}
		}
	}
	for _, v := range m.ProtectedIDs {
		if id == v {
			return true
		}
	}
	return labels["sor.hosting/infra"] == "true" || strings.HasPrefix(image, "nginx") || strings.Contains(image, "/nginx") || strings.Contains(image, "prometheus") || strings.Contains(image, "node-exporter") || strings.Contains(image, "node_exporter") || strings.Contains(image, "grafana")
}
func (m *Manager) Authorized(ctx context.Context, id string) (agentapi.SiteResources, error) {
	if !agentapi.OpaqueID.MatchString(id) {
		return agentapi.SiteResources{}, errors.New("invalid container ID")
	}
	v, e := m.API.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if e != nil {
		return agentapi.SiteResources{}, e
	}
	if v.Container.ID != id || v.Container.Config == nil {
		return agentapi.SiteResources{}, errors.New("exact ID required")
	}
	protected := m.protected(id, v.Container.Config.Image, v.Container.Config.Labels, v.Container.Name)
	sites, e := m.Store.Sites(ctx)
	if e != nil {
		return agentapi.SiteResources{}, e
	}
	for _, s := range sites {
		if inventory.Authorized(s, id, protected) {
			return s, nil
		}
	}
	return agentapi.SiteResources{}, errors.New("container not authorized")
}
func (m *Manager) List(ctx context.Context) ([]agentapi.Container, error) {
	v, e := m.API.ContainerList(ctx, client.ContainerListOptions{All: true})
	if e != nil {
		return nil, e
	}
	sites, e := m.Store.Sites(ctx)
	if e != nil {
		return nil, e
	}
	out := []agentapi.Container{}
	for _, c := range v.Items {
		name := c.ID[:12]
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		p := m.protected(c.ID, c.Image, c.Labels, c.Names...)
		a := agentapi.Container{ID: c.ID, Name: name, Image: c.Image, State: string(c.State), Status: c.Status, Protected: p}
		for _, s := range sites {
			if inventory.Authorized(s, c.ID, p) {
				a.Managed = true
				a.CanManage = true
				a.CanReadLogs = true
				a.SiteID = s.ID
			}
		}
		out = append(out, a)
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	for i := range out {
		if !out[i].Managed || out[i].State != "running" {
			continue
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			mem, cpu := m.stats(cctx, out[i].ID)
			out[i].MemoryBytes = mem
			out[i].CPUPct = cpu
		}(i)
	}
	wg.Wait()
	return out, nil
}
func (m *Manager) stats(ctx context.Context, id string) (*uint64, *float64) {
	r, e := m.API.ContainerStats(ctx, id, client.ContainerStatsOptions{Stream: false, IncludePreviousSample: true})
	if e != nil {
		return nil, nil
	}
	defer r.Body.Close()
	type cpu struct {
		Usage struct {
			Total uint64 `json:"total_usage"`
		} `json:"cpu_usage"`
		System uint64 `json:"system_cpu_usage"`
		Online uint32 `json:"online_cpus"`
	}
	var v struct {
		CPU    cpu `json:"cpu_stats"`
		Pre    cpu `json:"precpu_stats"`
		Memory struct {
			Usage uint64 `json:"usage"`
		} `json:"memory_stats"`
	}
	if json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&v) != nil {
		return nil, nil
	}
	value := 0.0
	if v.CPU.System > v.Pre.System && v.CPU.Usage.Total >= v.Pre.Usage.Total {
		value = float64(v.CPU.Usage.Total-v.Pre.Usage.Total) / float64(v.CPU.System-v.Pre.System) * float64(v.CPU.Online) * 100
	}
	return &v.Memory.Usage, &value
}
func (m *Manager) Action(ctx context.Context, id, action string) error {
	if _, e := m.Authorized(ctx, id); e != nil {
		return e
	}
	return m.RawAction(ctx, id, action)
}
func (m *Manager) RawAction(ctx context.Context, id, action string) error {
	switch action {
	case "start":
		_, e := m.API.ContainerStart(ctx, id, client.ContainerStartOptions{})
		return e
	case "stop":
		timeout := 30
		_, e := m.API.ContainerStop(ctx, id, client.ContainerStopOptions{Timeout: &timeout})
		return e
	case "restart":
		_, e := m.API.ContainerRestart(ctx, id, client.ContainerRestartOptions{})
		return e
	}
	return errors.New("action not allowed")
}

var secretPattern = regexp.MustCompile(`(?i)(password|token|secret|authorization)(\s*[:=]\s*)[^\s]+`)

func (m *Manager) Logs(ctx context.Context, id string, tail int) (agentapi.LogsResult, error) {
	s, e := m.Authorized(ctx, id)
	if e != nil {
		return agentapi.LogsResult{}, e
	}
	if tail < 1 || tail > 1000 {
		return agentapi.LogsResult{}, errors.New("invalid tail")
	}
	v, e := m.API.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if e != nil {
		return agentapi.LogsResult{}, e
	}
	r, e := m.API.ContainerLogs(ctx, id, client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Timestamps: true, Tail: strconv.Itoa(tail), Follow: false})
	if e != nil {
		return agentapi.LogsResult{}, e
	}
	defer r.Close()
	buf := &executil.Limited{Max: 256 << 10}
	if v.Container.Config.Tty {
		_, e = io.Copy(buf, r)
	} else {
		_, e = stdcopy.StdCopy(buf, buf, r)
	}
	if e != nil && !errors.Is(e, executil.ErrLimit) {
		return agentapi.LogsResult{}, e
	}
	text := buf.Buffer.String()
	if m.Secrets != nil {
		for _, secret := range m.Secrets(s.SecretRef) {
			if secret != "" {
				text = strings.ReplaceAll(text, secret, "[REDACTADO]")
			}
		}
	}
	text = secretPattern.ReplaceAllString(text, "$1$2[REDACTADO]")
	return agentapi.LogsResult{Lines: strings.Split(strings.TrimSuffix(text, "\n"), "\n"), Truncated: buf.Buffer.Len() >= buf.Max}, nil
}

func (m *Manager) IsProtected(id, image string, labels map[string]string, names ...string) bool {
	return m.protected(id, image, labels, names...)
}
