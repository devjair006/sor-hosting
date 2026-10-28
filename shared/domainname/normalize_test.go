package domainname

import "testing"

func TestDomainRejectsConfigInjection(t *testing.T) {
	for _, s := range []string{"a.example;foo", "https://a.example", "127.0.0.1", "a.example\n}", "*.a.example", " a.example", "a..example"} {
		if _, e := Normalize(s); e == nil {
			t.Fatalf("accepted %q", s)
		}
	}
}
func TestIDNNormalization(t *testing.T) {
	s, e := Normalize("BÜCHER.example")
	if e != nil || s != "xn--bcher-kva.example" {
		t.Fatalf("%q %v", s, e)
	}
}
