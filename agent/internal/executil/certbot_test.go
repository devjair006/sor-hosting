package executil

import "testing"

func TestCertbotPolicyClearsInheritedHooksAndAllowsHookInValues(t *testing.T) {
	a, e := CertbotArgs([]string{"certonly", "--domain", "webhooks.example.com", "--email", "hooks@example.com"})
	if e != nil {
		t.Fatal(e)
	}
	for _, flag := range []string{"--pre-hook", "--post-hook", "--deploy-hook"} {
		found := false
		for i, v := range a {
			if v == flag && i+1 < len(a) && a[i+1] == "" {
				found = true
			}
		}
		if !found {
			t.Fatal("inherited hook not cleared", flag)
		}
	}
	if _, e := CertbotArgs([]string{"certonly", "--pre-hook", "touch /etc/letsencrypt/foreign"}); e == nil {
		t.Fatal("hook command accepted")
	}
}
