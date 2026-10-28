package backups

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const maxArchive int64 = 32 << 30

func ValidateTar(r io.Reader) error {
	tr := tar.NewReader(io.LimitReader(r, maxArchive+1))
	var total int64
	count := 0
	for {
		h, e := tr.Next()
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
		count++
		clean := path.Clean(h.Name)
		if count > 500000 || strings.Contains(h.Name, "\\") || path.IsAbs(h.Name) || clean == ".." || strings.HasPrefix(clean, "../") || clean != "html" && !strings.HasPrefix(clean, "html/") || h.Size < 0 || h.Size > 8<<30 {
			return errors.New("unsafe archive entry")
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA && h.Typeflag != tar.TypeDir {
			return errors.New("links and special files rejected")
		}
		total += h.Size
		if total > maxArchive {
			return errors.New("archive too large")
		}
		if _, e = io.Copy(io.Discard, tr); e != nil {
			return e
		}
	}
}
func checksum(file string) (string, int64, error) {
	f, e := os.Open(file)
	if e != nil {
		return "", 0, e
	}
	defer f.Close()
	h := sha256.New()
	n, e := io.Copy(h, io.LimitReader(f, maxArchive+1))
	if n > maxArchive {
		return "", n, errors.New("backup too large")
	}
	return hex.EncodeToString(h.Sum(nil)), n, e
}
func pack(dir, target string) error {
	f, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	gz := gzip.NewWriter(&diskWriter{File: f, Root: dir})
	tw := tar.NewWriter(gz)
	for _, name := range []string{"manifest.json", "database.sql", "files.tar"} {
		src, e := os.Open(filepath.Join(dir, name))
		if e != nil {
			tw.Close()
			gz.Close()
			f.Close()
			return e
		}
		st, e := src.Stat()
		if e == nil {
			e = tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: st.Size(), Typeflag: tar.TypeReg})
		}
		if e == nil {
			_, e = io.Copy(tw, src)
		}
		src.Close()
		if e != nil {
			tw.Close()
			gz.Close()
			f.Close()
			return e
		}
	}
	e = tw.Close()
	if x := gz.Close(); e == nil {
		e = x
	}
	if x := f.Sync(); e == nil {
		e = x
	}
	if x := f.Close(); e == nil {
		e = x
	}
	return e
}
func unpack(file, dir string) error {
	f, e := os.Open(file)
	if e != nil {
		return e
	}
	defer f.Close()
	gz, e := gzip.NewReader(f)
	if e != nil {
		return e
	}
	defer gz.Close()
	tr := tar.NewReader(io.LimitReader(gz, maxArchive+1))
	seen := map[string]bool{}
	var total int64
	for {
		h, e := tr.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
		if h.Name != "manifest.json" && h.Name != "database.sql" && h.Name != "files.tar" || seen[h.Name] || h.Typeflag != tar.TypeReg || h.Size < 0 {
			return errors.New("invalid backup structure")
		}
		if h.Name == "manifest.json" && h.Size > 1<<20 {
			return errors.New("manifest too large")
		}
		total += h.Size
		if total > maxArchive {
			return errors.New("backup too large")
		}
		seen[h.Name] = true
		if e = freeSpace(dir, uint64(h.Size)+256<<20); e != nil {
			return e
		}
		out, e := os.OpenFile(filepath.Join(dir, h.Name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		_, e = io.Copy(out, tr)
		closeErr := out.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if len(seen) != 3 {
		return fmt.Errorf("incomplete backup")
	}
	return nil
}

type diskWriter struct {
	File    *os.File
	Root    string
	Written int64
}

func (w *diskWriter) Write(p []byte) (int, error) {
	if w.Written+int64(len(p)) > maxArchive {
		return 0, errors.New("backup size limit exceeded")
	}
	if e := freeSpace(w.Root, uint64(len(p))+(256<<20)); e != nil {
		return 0, e
	}
	n, e := w.File.Write(p)
	w.Written += int64(n)
	return n, e
}
