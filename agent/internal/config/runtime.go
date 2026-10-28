package config

import (
	"errors"
	"path/filepath"
	"regexp"
	"sor.hosting/shared/domainname"
	"strings"
)

type NginxConfig struct {
	Mode            string `json:"mode"`
	Container       string `json:"container"`
	IncludeDir      string `json:"include_dir"`
	Webroot         string `json:"webroot"`
	CertificatesDir string `json:"certificates_dir"`
	ProxyNetwork    string `json:"proxy_network"`
}
type CertbotConfig struct {
	Mode          string `json:"mode"`
	Image         string `json:"image"`
	Directory     string `json:"directory"`
	ConfigSource  string `json:"config_source"`
	WebrootSource string `json:"webroot_source"`
	Server        string `json:"server,omitempty"`
}

var imageDigest = regexp.MustCompile(`^[a-zA-Z0-9./:_-]+@sha256:[0-9a-f]{64}$`)
var dockerName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$`)

func cleanPath(p string) bool {
	return filepath.IsAbs(p) && p != "/" && filepath.Clean(p) == p && !strings.ContainsAny(p, "\x00\r\n\"'\\;$`{} \t")
}
func inHome(p string) bool {
	return p == "/home" || strings.HasPrefix(p, "/home/") || p == "/root" || strings.HasPrefix(p, "/root/") || p == "/run/user" || strings.HasPrefix(p, "/run/user/")
}
func (c *Config) NormalizeRuntime() error {
	n, b := &c.Nginx, &c.Certbot
	if n.Mode == "" {
		n.Mode = "native"
	}
	if b.Mode == "" {
		b.Mode = "native"
	}
	if (n.Mode != "native" && n.Mode != "docker") || (b.Mode != "native" && b.Mode != "docker") {
		return errors.New("runtime mode must be native or docker")
	}
	if b.Directory == "" {
		b.Directory = "/etc/letsencrypt"
	}
	if n.IncludeDir == "" {
		n.IncludeDir = c.NginxDir
	}
	if n.Webroot == "" {
		n.Webroot = c.ACMEWebroot
	}
	if n.CertificatesDir == "" {
		n.CertificatesDir = "/etc/letsencrypt"
	}
	for _, p := range []string{c.Socket, c.StateDir, c.BackupDir, c.NginxDir, c.ACMEWebroot, c.KeyFile, b.Directory} {
		if !cleanPath(p) || inHome(p) {
			return errors.New("agent paths require clean absolute paths outside home; use explicit systemd bind aliases")
		}
	}
	for _, p := range []string{n.IncludeDir, n.Webroot, n.CertificatesDir} {
		if !cleanPath(p) {
			return errors.New("invalid Nginx container path")
		}
	}
	if n.Mode == "docker" {
		if !dockerName.MatchString(n.Container) || !dockerName.MatchString(n.ProxyNetwork) {
			return errors.New("Docker Nginx requires container and explicit proxy_network")
		}
		if b.ConfigSource == "" || b.WebrootSource == "" {
			return errors.New("Docker Nginx requires explicit certificate and webroot host sources")
		}
	}
	if b.Mode == "docker" && !imageDigest.MatchString(b.Image) {
		return errors.New("Certbot Docker image must be pinned by sha256 digest")
	}
	if n.Mode == "docker" || b.Mode == "docker" {
		for _, p := range []string{b.ConfigSource, b.WebrootSource} {
			if !cleanPath(p) {
				return errors.New("explicit Docker bind sources required")
			}
		}
	}
	for i, host := range c.ProtectedDomains {
		v, e := domainname.Normalize(host)
		if e != nil {
			return errors.New("invalid protected domain")
		}
		c.ProtectedDomains[i] = v
	}
	return nil
}
