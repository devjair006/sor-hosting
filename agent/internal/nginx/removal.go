package nginx

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sor.hosting/shared/domainname"
	"strings"
)

func (m *Manager) DeletionFiles(ctx context.Context, hosts []string) (map[string][]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string][]byte{}
	if m.Check != nil {
		if e := m.Check(ctx); e != nil {
			return nil, e
		}
	}
	if e := PrivatePath(m.Dir); e != nil {
		return nil, e
	}
	for _, host := range hosts {
		normalized, e := domainname.Normalize(host)
		if e != nil || normalized != host {
			return nil, errors.New("invalid deletion domain")
		}
		for _, protected := range m.ProtectedDomains {
			if protected == host {
				return nil, errors.New("protected domain")
			}
		}
		if e = m.checkInclude(ctx, host); e != nil {
			return nil, e
		}
		p := filepath.Join(m.Dir, host+".conf")
		st, e := os.Lstat(p)
		if os.IsNotExist(e) {
			continue
		}
		if e != nil || !st.Mode().IsRegular() {
			return nil, errors.New("invalid managed config")
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return nil, e
		}
		if !strings.HasPrefix(string(b), "# Managed by SOR Hosting\n") {
			return nil, errors.New("unmanaged config cannot be deleted")
		}
		out[host] = b
	}
	return out, nil
}

// SetDeletionFiles changes only the captured files, rejecting concurrent replacements.
func (m *Manager) SetDeletionFiles(ctx context.Context, files map[string][]byte, restore bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Check != nil {
		if e := m.Check(ctx); e != nil {
			return e
		}
	}
	if e := PrivatePath(m.Dir); e != nil {
		return e
	}
	for host, b := range files {
		h, e := domainname.Normalize(host)
		if e != nil || h != host {
			return errors.New("invalid recovery host")
		}
		for _, protected := range m.ProtectedDomains {
			if protected == host {
				return errors.New("protected domain")
			}
		}
		p := filepath.Join(m.Dir, host+".conf")
		st, e := os.Lstat(p)
		if e == nil && !st.Mode().IsRegular() {
			return errors.New("invalid config target")
		}
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if e == nil {
			current, e := os.ReadFile(p)
			if e != nil {
				return e
			}
			if !bytes.Equal(current, b) {
				return errors.New("managed config changed during deletion")
			}
		}
		if restore {
			if e = os.WriteFile(p, b, 0644); e != nil {
				return e
			}
		} else if e == nil {
			if e = os.Remove(p); e != nil {
				return e
			}
		}
	}
	if e := m.run(ctx, []string{"-t"}); e != nil {
		return e
	}
	return m.run(ctx, []string{"-s", "reload"})
}
