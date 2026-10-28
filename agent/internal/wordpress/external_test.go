package wordpress

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/moby/moby/client"
	"net/http"
	"net/http/httptest"
	"path"
	"sor.hosting/agent/internal/docker"
	"sor.hosting/agent/internal/journal"
	"sor.hosting/shared/agentapi"
	"strings"
	"testing"
)

func TestExternalSitesShareDatabaseWithoutDockerWrites(t *testing.T) {
	ctx := context.Background()
	store, e := journal.Open(ctx, t.TempDir(), bytes.Repeat([]byte{1}, 32))
	if e != nil {
		t.Fatal(e)
	}
	defer store.DB.Close()
	writes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			writes++
			t.Errorf("unexpected mutation %s", r.Method)
		}
		id := path.Base(path.Dir(r.URL.Path))
		json.NewEncoder(w).Encode(map[string]any{"Id": id, "Config": map[string]any{"Image": "wordpress", "Labels": map[string]string{}}})
	}))
	defer srv.Close()
	api, e := client.New(client.WithHost(srv.URL), client.WithAPIVersion("1.56"))
	if e != nil {
		t.Fatal(e)
	}
	defer api.Close()
	engine := &docker.Manager{API: api, Store: store}
	m := Manager{Store: store, Docker: engine}
	dbID := strings.Repeat("d", 64)
	for i, web := range []string{strings.Repeat("a", 64), strings.Repeat("b", 64)} {
		input := agentapi.ExternalInput{Name: "External", Domain: []string{"one.example", "two.example"}[i], WebContainerID: web, DatabaseContainerID: dbID}
		payload, _ := json.Marshal(input)
		s, e := m.RegisterExternal(ctx, agentapi.OperationRequest{ID: "external-op", ResourceID: []string{"external01", "external02"}[i], Payload: payload})
		if e != nil {
			t.Fatal(e)
		}
		if !s.External() || s.Mutable() {
			t.Fatal(s)
		}
	}
	if _, e = engine.Authorized(ctx, dbID); e == nil {
		t.Fatal("shared external database became authorized")
	}
	if writes != 0 {
		t.Fatal("registration changed Docker")
	}
	sites, _ := store.Sites(ctx)
	if len(sites) != 2 {
		t.Fatal(sites)
	}
}
