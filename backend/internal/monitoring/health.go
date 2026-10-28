package monitoring

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sor.hosting/shared/domainname"
	"sor.hosting/shared/netguard"
	"strings"
	"time"
)

func Probe(ctx context.Context, host string) bool {
	host, e := domainname.Normalize(host)
	if e != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ips, e := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if e != nil || len(ips) == 0 {
		return false
	}
	for _, ip := range ips {
		if !netguard.Public(ip.String()) {
			return false
		}
	}
	target := net.JoinHostPort(ips[0].Unmap().String(), "443")
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 3 * time.Second}).DialContext(ctx, network, target)
	}}
	defer transport.CloseIdleConnections()
	client := http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 3 || r.URL.Hostname() != host || r.URL.Scheme != "https" || r.URL.Port() != "" && r.URL.Port() != "443" {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	req, e := http.NewRequestWithContext(ctx, "GET", "https://"+host+"/", nil)
	if e != nil {
		return false
	}
	req.Header.Set("User-Agent", "SOR-Hosting-Health/1")
	res, e := client.Do(req)
	if e != nil {
		return false
	}
	defer res.Body.Close()
	return res.StatusCode >= 200 && res.StatusCode < 300 && !strings.Contains(res.Header.Get("Location"), "169.254.")
}
