package nginx

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sor.hosting/shared/domainname"
	"strings"
)

type transaction struct {
	Host     string `json:"host"`
	Previous []byte `json:"previous"`
	Exists   bool   `json:"exists"`
}

func (m *Manager) Recover(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := PrivatePath(m.Dir); e != nil {
		return e
	}
	paths, e := filepath.Glob(filepath.Join(m.Dir, ".sor-*.rollback"))
	if e != nil {
		return e
	}
	for _, p := range paths {
		raw, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		var tx transaction
		if e = json.Unmarshal(raw, &tx); e != nil {
			return e
		}
		host, e := domainname.Normalize(tx.Host)
		if e != nil || !strings.HasSuffix(p, host+".rollback") {
			return errors.New("invalid proxy recovery transaction")
		}
		target := filepath.Join(m.Dir, host+".conf")
		if st, e := os.Lstat(target); e == nil && st.Mode()&os.ModeSymlink != 0 {
			return errors.New("symlink recovery target")
		}
		if tx.Exists {
			e = os.WriteFile(target, tx.Previous, 0644)
		} else {
			e = os.Remove(target)
			if os.IsNotExist(e) {
				e = nil
			}
		}
		if e != nil {
			return e
		}
		if e = m.run(ctx, []string{"-t"}); e != nil {
			return e
		}
		if e = m.run(ctx, []string{"-s", "reload"}); e != nil {
			return e
		}
		if e = os.Remove(p); e != nil {
			return e
		}
	}
	return nil
}
