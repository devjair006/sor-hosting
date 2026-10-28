package certbot

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"sor.hosting/shared/agentapi"
	"testing"
	"time"
)

func TestReadCertificateFromHostAliasAndRejectEscapingSymlink(t *testing.T) {
	dir := t.TempDir()
	name := "sor-new.example"
	live := filepath.Join(dir, "live", name)
	archive := filepath.Join(dir, "archive", name)
	os.MkdirAll(live, 0700)
	os.MkdirAll(archive, 0700)
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "new.example"}, DNSNames: []string{"new.example"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour)}
	der, _ := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	cert := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	p := filepath.Join(archive, "fullchain1.pem")
	os.WriteFile(p, cert, 0600)
	link := filepath.Join(live, "fullchain.pem")
	os.Symlink(p, link)
	c := Client{Directory: dir}
	if _, e := c.ReadCertificate("domain01", "new.example"); e != nil {
		t.Fatal(e)
	}
	outside := filepath.Join(t.TempDir(), "cert.pem")
	os.WriteFile(outside, cert, 0600)
	os.Remove(link)
	os.Symlink(outside, link)
	if _, e := c.ReadCertificate("domain01", "new.example"); e == nil {
		t.Fatal("certificate escaped mounted root")
	}
}

func TestDeletionAllowsExpiredOwnedCertificateButRejectsSharedOrReplaced(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "expired-owned", true: "shared-SAN"}[shared], func(t *testing.T) {
			dir := t.TempDir()
			host := "new.example"
			live := filepath.Join(dir, "live", "sor-"+host)
			if e := os.MkdirAll(live, 0700); e != nil {
				t.Fatal(e)
			}
			key, e := rsa.GenerateKey(rand.Reader, 2048)
			if e != nil {
				t.Fatal(e)
			}
			names := []string{host}
			if shared {
				names = append(names, "other.example")
			}
			tpl := &x509.Certificate{SerialNumber: big.NewInt(2), DNSNames: names, NotBefore: time.Now().Add(-48 * time.Hour), NotAfter: time.Now().Add(-time.Hour)}
			der, e := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(live, "fullchain.pem"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); e != nil {
				t.Fatal(e)
			}
			sum := sha256.Sum256(der)
			domain := agentapi.DomainResult{ID: "domain01", Hostname: host, Certificate: &agentapi.Certificate{CertName: "sor-" + host, Fingerprint: hex.EncodeToString(sum[:])}}
			c := Client{Directory: dir, Managed: func(context.Context, string, string, string) bool { return true }}
			if e = c.VerifyDeletion(context.Background(), domain); (e != nil) != shared {
				t.Fatal("wrong certificate deletion policy", e)
			}
			domain.Certificate.Fingerprint = "different"
			if c.VerifyDeletion(context.Background(), domain) == nil {
				t.Fatal("replaced certificate accepted")
			}
		})
	}
}
func TestExistingUnregisteredLineagesAndProtectedDomainsAreDenied(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "live", "sor-old.example"), 0700)
	c := Client{Directory: dir, ProtectedDomains: []string{"projectkarli.dev"}}
	if c.CheckLineage(context.Background(), "domain01", "old.example", false) == nil {
		t.Fatal("foreign sor- lineage accepted")
	}
	if c.CheckLineage(context.Background(), "domain01", "projectkarli.dev", false) == nil {
		t.Fatal("protected certificate accepted")
	}
	if c.CheckLineage(context.Background(), "domain01", "new.example", true) == nil {
		t.Fatal("unregistered renewal accepted")
	}
	c.Managed = func(context.Context, string, string, string) bool { return true }
	if e := c.CheckLineage(context.Background(), "domain01", "old.example", true); e != nil {
		t.Fatal(e)
	}
}
