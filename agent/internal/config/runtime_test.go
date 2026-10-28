package config

import "testing"

func runtimeConfig() Config {
	return Config{Socket: "/run/sor/agent.sock", StateDir: "/var/lib/sor", BackupDir: "/var/lib/sor/backups", NginxDir: "/etc/nginx/sor-hosting.d", ACMEWebroot: "/var/lib/sor/acme", KeyFile: "/etc/sor/key"}
}
func TestRuntimeDefaultsRemainNative(t *testing.T) {
	c := runtimeConfig()
	if err := c.NormalizeRuntime(); err != nil {
		t.Fatal(err)
	}
	if c.Nginx.Mode != "native" || c.Certbot.Mode != "native" || c.Certbot.Directory != "/etc/letsencrypt" {
		t.Fatal(c)
	}
}
func TestDockerRuntimeRequiresExplicitMountsAndPinnedImage(t *testing.T) {
	c := runtimeConfig()
	c.Nginx = NginxConfig{Mode: "docker", Container: "wordpress-server-nginx-1", ProxyNetwork: "sor-proxy"}
	c.Certbot = CertbotConfig{Mode: "docker"}
	if c.NormalizeRuntime() == nil {
		t.Fatal("missing sources/image accepted")
	}
	c.Certbot.Image = "certbot/certbot@sha256:" + string(make([]byte, 64))
	if c.NormalizeRuntime() == nil {
		t.Fatal("invalid digest accepted")
	}
}
func TestAgentHomePathsNeedMountAlias(t *testing.T) {
	c := runtimeConfig()
	c.ACMEWebroot = "/home/azureuser/certbot/www"
	if c.NormalizeRuntime() == nil {
		t.Fatal("agent home access accepted without mount alias")
	}
}
