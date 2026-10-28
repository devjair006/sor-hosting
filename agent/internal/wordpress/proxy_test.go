package wordpress

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/moby/moby/client"
	"net/http"
	"net/http/httptest"
	"os"
	"sor.hosting/agent/internal/config"
	"sor.hosting/agent/internal/docker"
	"sor.hosting/agent/internal/nginx"
	"sor.hosting/shared/agentapi"
	"strings"
	"testing"
)

func TestProxyAliasRequiresSharedNetworkAndUniqueAlias(t *testing.T) {
	shared := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		network := "shared"
		if !shared && strings.Contains(r.URL.Path, "/edge/") {
			network = "other"
		}
		json.NewEncoder(w).Encode(map[string]any{"Id": "container", "Name": "/web-project", "Config": map[string]any{"Labels": map[string]string{"sor.hosting/project": "project01"}}, "NetworkSettings": map[string]any{"Networks": map[string]any{"proxy": map[string]any{"NetworkID": network, "Aliases": []string{"sor-project01"}}}}})
	}))
	defer srv.Close()
	api, e := client.New(client.WithHost(srv.URL), client.WithAPIVersion("1.56"))
	if e != nil {
		t.Fatal(e)
	}
	defer api.Close()
	m := Manager{Cfg: config.Config{Nginx: config.NginxConfig{Mode: "docker", Container: "edge", ProxyNetwork: "proxy"}}, Docker: &docker.Manager{API: api}}
	if alias, e := m.ProxyAlias(context.Background(), agentapi.SiteResources{WebID: "web-id"}); e != nil || alias != "sor-project01" {
		t.Fatal(alias, e)
	}
	shared = false
	if _, e := m.ProxyAlias(context.Background(), agentapi.SiteResources{WebID: "web-id"}); e == nil {
		t.Fatal("separate networks accepted")
	}
}

func TestMissingEdgeAliasBlocksInstallBeforeResources(t *testing.T) {
	root := t.TempDir()
	m := Manager{Cfg: config.Config{StateDir: root, Nginx: config.NginxConfig{Mode: "docker"}}, Nginx: &nginx.Manager{Health: func(context.Context) bool { return true }, Check: func(context.Context) error { return errors.New("missing mount alias") }}}
	payload := json.RawMessage(`{"name":"Test","domain":"new.example"}`)
	_, e := m.Install(context.Background(), agentapi.OperationRequest{ResourceID: "resource01", Payload: payload}, func(string) error { t.Fatal("checkpoint reached before mount validation"); return nil })
	if e == nil || !strings.Contains(e.Error(), "missing mount alias") {
		t.Fatal("mount error was not returned first", e)
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("resources written before mount validation")
	}
}

func TestMissingEdgeAliasBlocksPrepareBeforeResources(t *testing.T) {
	m := Manager{Cfg: config.Config{StateDir: t.TempDir()}, Nginx: &nginx.Manager{Check: func(context.Context) error { return errors.New("missing mount alias") }}}
	_, e := m.Prepare(context.Background(), "resource01", "resource01", "resource01", agentapi.SiteInput{Domain: "new.example", Resources: agentapi.Resources{MemoryMB: 512, CPULimit: 1}})
	if e == nil || !strings.Contains(e.Error(), "missing mount alias") {
		t.Fatal("prepare did not reject missing alias first", e)
	}
}
