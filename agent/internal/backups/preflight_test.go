package backups

import (
	"context"
	"errors"
	"sor.hosting/agent/internal/nginx"
	"sor.hosting/agent/internal/wordpress"
	"strings"
	"testing"
)

func TestRestoreRejectsMissingEdgeBeforeMaintenance(t *testing.T) {
	m := Manager{WP: &wordpress.Manager{Nginx: &nginx.Manager{Check: func(context.Context) error { return errors.New("missing mount alias") }}}}
	_, e := m.Restore(context.Background(), "backup01", "site0001", "new.example", "restore1")
	if e == nil || !strings.Contains(e.Error(), "missing mount alias") {
		t.Fatal(e)
	}
}
