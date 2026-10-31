package server

import (
	"reflect"
	"testing"
)

func TestNormalizeSRCScopes(t *testing.T) {
	got := normalizeSRCScopes([]string{
		" https://*.Example.com/path?x=1 ",
		"example.com",
		"10.0.0.0/24",
		"# comment",
		"",
		"API.EXAMPLE.COM",
	})
	want := []string{"example.com", "10.0.0.0/24", "api.example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeSRCScopes = %v, want %v", got, want)
	}
}

func TestInferScopeKind(t *testing.T) {
	cases := map[string]string{
		"example.com":   "root_domain",
		"10.0.0.0/24":   "cidr",
		"192.168.1.1":   "ip",
		"2001:db8::1":   "ip",
		"2001:db8::/32":  "cidr",
		"not a domain?": "root_domain",
	}
	for in, want := range cases {
		if got := inferScopeKind(in); got != want {
			t.Errorf("inferScopeKind(%q) = %q, want %q", in, got, want)
		}
	}
}
