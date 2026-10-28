package docker

import "testing"

func TestConfiguredNginxNameIsProtectedAcrossRecreation(t *testing.T) {
	m := &Manager{ProtectedNames: []string{"wordpress-server-nginx-1"}}
	for _, id := range []string{"old-id", "new-id"} {
		if !m.IsProtected(id, "custom/proxy", nil, "/wordpress-server-nginx-1") {
			t.Fatal("configured Nginx became manageable")
		}
	}
}
