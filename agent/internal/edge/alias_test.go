package edge

import "testing"

func TestCertificateAliasRequiresExplicitReadonlyMount(t *testing.T) {
	source := "/home/azureuser/wordpress-server/certbot/conf"
	target := "/var/lib/sor/certificates"
	good := "23 12 8:1 /home/azureuser/wordpress-server/certbot/conf /var/lib/sor/certificates ro,relatime - ext4 /dev/test rw\n"
	if err := checkAliasMount(source, target, good, "1 0 8:1 / / rw - ext4 /dev/test rw\n"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"", "23 12 8:1 /home/wrong /var/lib/sor/certificates rw,relatime - ext4 /dev/test rw\n", "23 12 8:1 /home/wrong /var/lib/sor/certificates ro,relatime - ext4 /dev/test rw\n"} {
		if checkAliasMount(source, target, bad, "1 0 8:1 / / rw - ext4 /dev/test rw\n") == nil {
			t.Fatal("unmounted or writable alias accepted")
		}
	}
}
