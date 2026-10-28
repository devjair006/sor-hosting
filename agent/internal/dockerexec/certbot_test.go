package dockerexec

import (
	"context"
	"encoding/json"
	"github.com/moby/moby/client"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCertbotTemporaryContainerFailureCleansExactID(t *testing.T) {
	removed := false
	var created map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.Contains(r.URL.Path, "/images/"):
			w.Write([]byte(`{"Id":"sha256:test"}`))
		case strings.HasSuffix(r.URL.Path, "/containers/create"):
			json.NewDecoder(r.Body).Decode(&created)
			w.Write([]byte(`{"Id":"owned-container"}`))
		case strings.HasSuffix(r.URL.Path, "/start"):
			w.WriteHeader(204)
		case strings.HasSuffix(r.URL.Path, "/wait"):
			w.Write([]byte(`{"StatusCode":1}`))
		case r.Method == "DELETE" && strings.HasSuffix(r.URL.Path, "/owned-container"):
			removed = true
			w.WriteHeader(204)
		default:
			t.Errorf("unexpected API request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	api, e := client.New(client.WithHost(srv.URL), client.WithAPIVersion("1.56"))
	if e != nil {
		t.Fatal(e)
	}
	defer api.Close()
	c := &Certbot{API: api, Image: "certbot/certbot@sha256:" + strings.Repeat("a", 64), ConfigSource: "/explicit/certbot/conf", WebrootSource: "/explicit/certbot/www"}
	if e = c.Run(context.Background(), "certbot", []string{"--version"}, nil, nil); e == nil {
		t.Fatal("nonzero status accepted")
	}
	if !removed {
		t.Fatal("temporary container leaked")
	}
	h := created["HostConfig"].(map[string]any)
	if h["ReadonlyRootfs"] != true || h["Privileged"] == true {
		t.Fatal(h)
	}
	mounts := h["Mounts"].([]any)
	if len(mounts) != 2 {
		t.Fatal(mounts)
	}
	if e = c.Run(context.Background(), "sh", nil, nil, nil); e == nil {
		t.Fatal("arbitrary executable accepted")
	}
}
