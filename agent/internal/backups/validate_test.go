package backups

import (
	"archive/tar"
	"bytes"
	"testing"
)

func TestArchiveRejectsTraversalAndSymlinks(t *testing.T) {
	for _, h := range []*tar.Header{{Name: "../escape", Mode: 0600, Size: 1, Typeflag: tar.TypeReg}, {Name: "/absolute", Mode: 0600, Typeflag: tar.TypeDir}, {Name: "html/link", Typeflag: tar.TypeSymlink, Linkname: "/etc/shadow"}} {
		var b bytes.Buffer
		w := tar.NewWriter(&b)
		w.WriteHeader(h)
		if h.Size > 0 {
			w.Write([]byte("x"))
		}
		w.Close()
		if ValidateTar(bytes.NewReader(b.Bytes())) == nil {
			t.Fatal("unsafe archive accepted")
		}
	}
}
