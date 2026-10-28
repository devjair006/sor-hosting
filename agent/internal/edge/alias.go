package edge

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type mountInfo struct {
	device, root, point string
	ro                  bool
}

func mounts(raw string) []mountInfo {
	var result []mountInfo
	for _, line := range strings.Split(raw, "\n") {
		f := strings.Fields(line)
		if len(f) < 7 {
			continue
		}
		result = append(result, mountInfo{f[2], f[3], f[4], strings.Contains(","+f[5]+",", ",ro,")})
	}
	return result
}
func requireAliasMount(source, target string) error {
	if source == target {
		return nil
	}
	self, e := os.ReadFile("/proc/self/mountinfo")
	if e != nil {
		return e
	}
	host, e := os.ReadFile("/proc/1/mountinfo")
	if e != nil {
		return e
	}
	return checkAliasMount(source, target, string(self), string(host))
}
func checkAliasMount(source, target, self, host string) error {
	var parent mountInfo
	for _, m := range mounts(host) {
		if (m.point == "/" || source == m.point || strings.HasPrefix(source, m.point+"/")) && len(m.point) > len(parent.point) {
			parent = m
		}
	}
	if parent.point == "" {
		return errors.New("host source mount not found")
	}
	rel, e := filepath.Rel(parent.point, source)
	if e != nil {
		return e
	}
	want := filepath.Join(parent.root, rel)
	for _, m := range mounts(self) {
		if m.point == target && m.ro && m.device == parent.device && m.root == want {
			return nil
		}
	}
	return errors.New("certificate alias must be an explicit readonly systemd bind of config_source")
}
