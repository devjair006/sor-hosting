package certbot

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sor.hosting/agent/internal/executil"
	"sor.hosting/shared/agentapi"
	"sor.hosting/shared/domainname"
	"sor.hosting/shared/netguard"
	"strings"
	"time"
)

type Client struct {
	Check                                                   func() error
	Webroot, Email                                          string
	PublicIPs                                               []string
	Runner                                                  executil.Executor
	Directory, ContainerWebroot, Server, ExecutionDirectory string
	ProtectedDomains                                        []string
	Managed                                                 func(context.Context, string, string, string) bool
}

func (c *Client) DNS(ctx context.Context, host string) bool {
	ips, e := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if e != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		ok := false
		for _, allowed := range c.PublicIPs {
			if ip.Unmap().String() == allowed && netguard.Public(allowed) {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
func (c *Client) Issue(ctx context.Context, id, host string, renew bool) (*agentapi.Certificate, error) {
	host, e := domainname.Normalize(host)
	if e != nil {
		return nil, e
	}
	if e = c.CheckLineage(ctx, id, host, renew); e != nil {
		return nil, e
	}
	if !c.DNS(ctx, host) {
		return nil, errors.New("DNS_PENDING")
	}
	name := "sor-" + host
	webroot := c.ContainerWebroot
	if webroot == "" {
		webroot = c.Webroot
	}
	args := []string{"certonly", "--webroot", "--webroot-path", webroot, "--domain", host, "--cert-name", name, "--email", c.Email, "--agree-tos", "--non-interactive", "--keep-until-expiring", "--no-directory-hooks"}
	if renew {
		args = []string{"renew", "--cert-name", name, "--non-interactive", "--no-directory-hooks", "--webroot", "--webroot-path", webroot}
	}
	if c.ExecutionDirectory != "" {
		args = append(args, "--config-dir", c.ExecutionDirectory)
	}
	if c.Server != "" {
		args = append(args, "--server", c.Server)
	}
	args, e = executil.CertbotArgs(args)
	if e != nil {
		return nil, e
	}
	runner := c.Runner
	if runner == nil {
		runner = executil.Runner{}
	}
	if e = runner.Run(ctx, "certbot", args, nil, nil); e != nil {
		return nil, e
	}
	return c.ReadCertificate(id, host)
}
func (c *Client) root() string {
	if c.Directory != "" {
		return c.Directory
	}
	return "/etc/letsencrypt"
}
func (c *Client) Available(ctx context.Context) bool {
	if c.Check != nil && c.Check() != nil {
		return false
	}
	if r, ok := c.Runner.(interface{ Available(context.Context) bool }); ok {
		return r.Available(ctx)
	}
	_, e := exec.LookPath("certbot")
	return e == nil
}
func (c *Client) CheckLineage(ctx context.Context, id, host string, renew bool) error {
	if c.Check != nil {
		if e := c.Check(); e != nil {
			return e
		}
	}
	host, e := domainname.Normalize(host)
	if e != nil {
		return e
	}
	for _, p := range c.ProtectedDomains {
		if p == host {
			return errors.New("protected certificate domain")
		}
	}
	name := "sor-" + host
	owned := c.Managed != nil && c.Managed(ctx, id, host, name)
	if renew && !owned {
		return errors.New("unknown managed certificate")
	}
	for _, p := range []string{filepath.Join(c.root(), "live", name), filepath.Join(c.root(), "archive", name), filepath.Join(c.root(), "renewal", name+".conf")} {
		_, e := os.Lstat(p)
		if e == nil && !owned {
			return errors.New("existing certificate lineage is not registered to SOR")
		}
		if e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	return nil
}
func (c *Client) certificate(host string) (*x509.Certificate, error) {
	host, e := domainname.Normalize(host)
	if e != nil {
		return nil, e
	}
	name := "sor-" + host
	root, e := filepath.EvalSymlinks(c.root())
	if e != nil {
		return nil, e
	}
	path, e := filepath.EvalSymlinks(filepath.Join(root, "live", name, "fullchain.pem"))
	if e != nil {
		return nil, e
	}
	if !strings.HasPrefix(path, root+string(os.PathSeparator)) {
		return nil, errors.New("certificate symlink escapes configured directory")
	}
	st, e := os.Stat(path)
	if e != nil || !st.Mode().IsRegular() || st.Size() > 1<<20 {
		return nil, errors.New("invalid certificate file")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, errors.New("invalid certificate")
	}
	cert, e := x509.ParseCertificate(block.Bytes)
	if e != nil {
		return nil, e
	}
	if e = cert.VerifyHostname(host); e != nil {
		return nil, e
	}
	return cert, nil
}
func (c *Client) ReadCertificate(id, host string) (*agentapi.Certificate, error) {
	host, e := domainname.Normalize(host)
	if e != nil {
		return nil, e
	}
	cert, e := c.certificate(host)
	if e != nil {
		return nil, e
	}
	now := time.Now()
	if cert.NotAfter.Before(now) || cert.NotBefore.After(now) {
		return nil, errors.New("certificate is not currently valid")
	}
	sum := sha256.Sum256(cert.Raw)
	return &agentapi.Certificate{ID: id, DomainID: id, CertName: "sor-" + host, Hostname: host, ExpiresAt: cert.NotAfter.UTC().Format(time.RFC3339), Fingerprint: hex.EncodeToString(sum[:])}, nil
}

// VerifyDeletion rejects shared SAN certificates and unknown or replaced lineages.
func (c *Client) VerifyDeletion(ctx context.Context, d agentapi.DomainResult) error {
	if d.Certificate == nil {
		return nil
	}
	if e := c.CheckLineage(ctx, d.ID, d.Hostname, true); e != nil {
		return e
	}
	cert, e := c.certificate(d.Hostname)
	if e != nil {
		return e
	}
	sum := sha256.Sum256(cert.Raw)
	if hex.EncodeToString(sum[:]) != d.Certificate.Fingerprint || d.Certificate.CertName != "sor-"+d.Hostname {
		return errors.New("certificate identity changed")
	}
	if len(cert.DNSNames) != 1 || cert.DNSNames[0] != d.Hostname {
		return errors.New("shared certificate is protected")
	}
	return nil
}
func (c *Client) DeleteOwned(ctx context.Context, d agentapi.DomainResult) error {
	if d.Certificate == nil {
		return nil
	}
	name := "sor-" + d.Hostname
	if d.Certificate.CertName != name {
		return errors.New("certificate name mismatch")
	}
	missing := true
	for _, p := range []string{filepath.Join(c.root(), "live", name), filepath.Join(c.root(), "archive", name), filepath.Join(c.root(), "renewal", name+".conf")} {
		_, e := os.Lstat(p)
		if e == nil {
			missing = false
		} else if !os.IsNotExist(e) {
			return e
		}
	}
	if missing {
		return nil
	}
	if e := c.VerifyDeletion(ctx, d); e != nil {
		return e
	}
	args, e := executil.CertbotArgs([]string{"delete", "--cert-name", name, "--non-interactive", "--config-dir", c.ExecutionDirectory})
	if e != nil {
		return e
	}
	runner := c.Runner
	if runner == nil {
		runner = executil.Runner{}
	}
	return runner.Run(ctx, "certbot", args, nil, nil)
}
