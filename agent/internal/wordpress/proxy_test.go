package wordpress

import (
	"context"
	"encoding/json"
	"github.com/moby/moby/client"
	"net/http"
	"net/http/httptest"
	"sor.hosting/agent/internal/config"
	"sor.hosting/agent/internal/docker"
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
