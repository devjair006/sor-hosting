package domainname

import (
	"errors"
	"golang.org/x/net/idna"
	"net"
	"strings"
)

func Normalize(s string) (string, error) {
	if strings.TrimSpace(s) != s || strings.ContainsAny(s, "/:;{}\\\n\r\t@*#? ") {
		return "", errors.New("dominio inválido")
	}
	s = strings.ToLower(strings.TrimSuffix(s, "."))
	a, e := idna.Lookup.ToASCII(s)
	if e != nil || len(a) > 253 || net.ParseIP(a) != nil || !strings.Contains(a, ".") {
		return "", errors.New("se requiere un FQDN")
	}
	for _, p := range strings.Split(a, ".") {
		if p == "" || len(p) > 63 || p[0] == '-' || p[len(p)-1] == '-' {
			return "", errors.New("etiqueta inválida")
		}
		for _, c := range p {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return "", errors.New("etiqueta inválida")
			}
		}
	}
	return a, nil
}
