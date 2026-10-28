package certbot

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
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
