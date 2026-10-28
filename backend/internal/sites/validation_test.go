package sites

import (
	"sor.hosting/shared/agentapi"
	"testing"
)

func TestInstallRejectsUnsafeDomainAndResources(t *testing.T) {
	v := agentapi.SiteInput{Name: "Example", Domain: "a.example; include /etc/passwd", AdminEmail: "a@example.com", AdminPassword: "valid-password", WordPressTitle: "Example", Resources: agentapi.Resources{MemoryMB: 512, CPULimit: 1}}
	if ValidateInstall(&v) == nil {
		t.Fatal("unsafe domain accepted")
	}
	v.Domain = "a.example"
	v.Resources.MemoryMB = 1
	if ValidateInstall(&v) == nil {
		t.Fatal("invalid resource budget accepted")
	}
}
