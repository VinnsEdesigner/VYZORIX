package provisioning

import "testing"

// TestNormaliseHostKeepsExplicitHTTP covers a real defect: the scheme was always stripped,
// and the health checker forces https:// when no scheme is present, so a plain-HTTP git host
// (an internal Gitea, a LAN server, a loopback server in development) could never be stored
// in a form the checker could probe — it could only ever read "unreachable".
//
// https is the checker's default, so stripping it is harmless and keeps
// "https://github.com/" and "github.com" as one connection. http is not the default, so it
// is preserved.
func TestNormaliseHostKeepsExplicitHTTP(t *testing.T) {
	cases := []struct{ in, want string }{
		{"http://127.0.0.1:8088", "http://127.0.0.1:8088"},
		{"http://127.0.0.1:8088/", "http://127.0.0.1:8088"},
		{"  http://gitea.lan:3000/  ", "http://gitea.lan:3000"},
		// https is the default, so it collapses to the bare host.
		{"https://git.example.com", "git.example.com"},
		{"github.com", "github.com"},
		{"git.example.com/", "git.example.com"},
		{"", ""},
	}

	for _, tc := range cases {
		if got := normaliseHost(tc.in); got != tc.want {
			t.Errorf("normaliseHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// A host carrying a scheme must stay valid: the "//" a scheme introduces is not a path.
func TestValidHostAcceptsScheme(t *testing.T) {
	for _, in := range []string{"http://127.0.0.1:8088", "https://git.example.com", "github.com"} {
		if !validHost(in) {
			t.Errorf("validHost(%q) = false, want true", in)
		}
	}

	for _, in := range []string{"", "   ", "http://two words", "some/path"} {
		if validHost(in) {
			t.Errorf("validHost(%q) = true, want false", in)
		}
	}
}
