//go:build integration

package dockerexec

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"sor.hosting/agent/internal/executil"
)

func TestRealCertbotDoesNotInheritGlobalOrRenewalHooks(t *testing.T) {
	image := os.Getenv("SOR_TEST_CERTBOT_IMAGE")
	if os.Getenv("SOR_EDGE_INTEGRATION") != "1" || image == "" {
		t.Skip("Docker edge integration disabled")
	}
	// This fixture executes Certbot's real parser, renewal restoration, and hook functions.
	// No ACME server, host mounts, external hook, or public certificate request is needed.
	args, e := executil.CertbotArgs([]string{"renew", "--cert-name", "sor-webhooks.example.com", "--webroot", "--webroot-path", "/var/www/certbot"})
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(args)
	script := `import json,sys
from pathlib import Path
from certbot._internal import cli,renewal,hooks
from certbot._internal.plugins import disco
root=Path('/etc/letsencrypt');root.mkdir(exist_ok=True)
Path('/var/www/certbot').mkdir(parents=True,exist_ok=True)
marker=root/'foreign-hook-ran'
command='touch '+str(marker)
config=root/'cli.ini'
original='pre-hook = '+command+'\npost-hook = '+command+'\ndeploy-hook = '+command+'\n'
config.write_text(original)
protected=root/'existing-certificate.pem';protected.write_text('unchanged')
c=cli.prepare_and_parse_args(disco.PluginsRegistry.find_all(),json.loads(sys.argv[1]))
renewal.restore_required_config_elements(c,{'pre_hook':command,'post_hook':command,'deploy_hook':command})
assert c.pre_hook == c.post_hook == c.deploy_hook == ''
assert c.directory_hooks is False
hooks.pre_hook(c)
hooks.post_hook(c,False)
hooks.deploy_hook(c,[],'/etc/letsencrypt/live/sor-webhooks.example.com')
hooks.run_saved_post_hooks([],[])
assert not marker.exists()
assert config.read_text()==original
assert protected.read_text()=='unchanged'
print('global and selected renewal hooks suppressed; existing files unchanged')
`
	output, e := exec.Command("docker", "run", "--rm", "--entrypoint", "python", image, "-c", script, string(raw)).CombinedOutput()
	if e != nil {
		t.Fatalf("real Certbot hooks: %v: %s", e, output)
	}
	t.Log(string(output))
}
