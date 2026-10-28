package edge

import (
	"context"
	"encoding/json"
	"os"
	"sor.hosting/agent/internal/config"
	"sor.hosting/agent/internal/wordpress"
	"sor.hosting/shared/agentapi"
	"testing"
)

func TestCertificateAliasRequiresExplicitReadonlyMount(t *testing.T) {
	source := "/home/azureuser/wordpress-server/certbot/conf"
	target := "/var/lib/sor/certificates"
	good := "23 12 8:1 /home/azureuser/wordpress-server/certbot/conf /var/lib/sor/certificates ro,relatime - ext4 /dev/test rw\n"
	if err := checkAliasMount(source, target, good, "1 0 8:1 / / rw - ext4 /dev/test rw\n"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "23 12 8:1 /home/wrong /var/lib/sor/certificates rw,relatime - ext4 /dev/test rw\n", "23 12 8:1 /home/wrong /var/lib/sor/certificates ro,relatime - ext4 /dev/test rw\n"} {
		if checkAliasMount(source, target, bad, "1 0 8:1 / / rw - ext4 /dev/test rw\n") == nil {
			t.Fatal("unmounted or writable alias accepted")
		}
	}
}

func TestWebrootAliasMustMatchSourceBeforeProxyChanges(t *testing.T) {
	source := "/home/azureuser/wordpress-server/certbot/www"
	target := "/var/lib/sor/acme"
	host := "1 0 8:1 / / rw - ext4 /dev/test rw\n"
	if checkAliasMountMode(source, target, "", host, false) == nil {
		t.Fatal("plain directory accepted")
	}
	good := "23 12 8:1 /home/azureuser/wordpress-server/certbot/www /var/lib/sor/acme rw - ext4 /dev/test rw\n"
	if e := checkAliasMountMode(source, target, good, host, false); e != nil {
		t.Fatal(e)
	}
}

func TestRuntimeMissingAliasesRejectsActualInstallEntry(t *testing.T) {
	root := t.TempDir()
	cfg := config.Config{StateDir: root, NginxDir: root, ACMEWebroot: root, Nginx: config.NginxConfig{Mode: "docker"}, Certbot: config.CertbotConfig{Mode: "docker", Directory: root, ConfigSource: "/home/azureuser/certbot/conf", WebrootSource: "/home/azureuser/certbot/www"}}
	proxy, certs := New(cfg, nil, nil)
	if proxy.Available(context.Background()) || certs.Available(context.Background()) {
		t.Fatal("missing aliases reported ready")
	}
	wp := wordpress.Manager{Cfg: cfg, Nginx: proxy}
	_, e := wp.Install(context.Background(), agentapi.OperationRequest{ResourceID: "resource01", Payload: json.RawMessage(`{"domain":"new.example"}`)}, func(string) error { t.Fatal("checkpoint reached"); return nil })
	if e == nil {
		t.Fatal("missing alias accepted")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("wrote before preflight")
	}
}
