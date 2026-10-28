package inventory

import (
	"sor.hosting/shared/agentapi"
	"testing"
)

func TestForeignContainerRejected(t *testing.T) {
	site := agentapi.SiteResources{ID: "site-123456", WebID: "web-123456", DBID: "db-123456"}
	if Authorized(site, "web-123456", false) != true || Authorized(site, "foreign-id", false) || Authorized(site, "web-123456", true) {
		t.Fatal("ownership or protected resource checks failed")
	}
}
