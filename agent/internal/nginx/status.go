package nginx

import (
	"context"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func (m *Manager) Available(ctx context.Context) bool {
	if m.Health != nil {
		return m.Health(ctx)
	}
	b, e := os.ReadFile("/run/nginx.pid")
	if e != nil {
		return false
	}
	pid, e := strconv.Atoi(strings.TrimSpace(string(b)))
	return e == nil && pid > 1 && syscall.Kill(pid, 0) == nil
}
