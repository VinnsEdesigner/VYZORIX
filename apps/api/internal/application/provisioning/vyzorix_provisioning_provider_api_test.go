package provisioning

import (
	"testing"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// TestProbeTargetUsesProviderAPI is the guard for the health-check false positive.
//
// The probe used to build https://github.com/user, which answers 200 to anyone — the
// website serves a page to an anonymous request — so a dead token read as healthy. The
// probe must hit the API, where the same token is 401.
func TestProbeTargetUsesProviderAPI(t *testing.T) {
	cases := []struct {
		name     string
		host     string
		kind     domain.ConnectionType
		want     string
		wantAuth bool
	}{
		{"github", "github.com", domain.ConnectionGitHub, "https://api.github.com/user", true},
		{"github enterprise", "ghe.acme.internal", domain.ConnectionGitHub,
			"https://ghe.acme.internal/api/v3/user", true},
		{"gitlab", "gitlab.com", domain.ConnectionGitLab, "https://gitlab.com/api/v4/user", true},
		{"self-hosted gitlab", "git.acme.internal", domain.ConnectionGitLab,
			"https://git.acme.internal/api/v4/user", true},
		{"generic", "git.acme.internal", domain.ConnectionGeneric, "https://git.acme.internal", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint, header := probeTarget(tc.host, tc.kind)
			if endpoint != tc.want {
				t.Fatalf("endpoint = %q, want %q", endpoint, tc.want)
			}
			if (header != "") != tc.wantAuth {
				t.Fatalf("auth header = %q, want present = %v", header, tc.wantAuth)
			}
		})
	}
}

// TestAuthValueAlwaysBearer pins the header form. A bare token in Authorization is not
// a scheme either provider documents, so sending one unqualified made a valid
// credential read as rejected.
func TestAuthValueAlwaysBearer(t *testing.T) {
	if got := authValue("tok"); got != "Bearer tok" {
		t.Fatalf("authValue = %q, want %q", got, "Bearer tok")
	}
}

// TestBranchEndpointFollowsRepositoryHost is the guard for the hardcoded-provider
// defect: a GitHub Enterprise or self-hosted GitLab repository must be verified on its
// own host, not on api.github.com or gitlab.com.
func TestBranchEndpointFollowsRepositoryHost(t *testing.T) {
	cases := []struct {
		name    string
		repoURL string
		kind    domain.ConnectionType
		want    string
	}{
		{"github.com", "https://github.com/acme/fleet.git", domain.ConnectionGitHub,
			"https://api.github.com/repos/acme/fleet/branches/main"},
		{"github enterprise", "https://ghe.acme.internal/acme/fleet.git", domain.ConnectionGitHub,
			"https://ghe.acme.internal/api/v3/repos/acme/fleet/branches/main"},
		{"gitlab.com", "https://gitlab.com/acme/fleet.git", domain.ConnectionGitLab,
			"https://gitlab.com/api/v4/projects/acme%2Ffleet/repository/branches/main"},
		{"self-hosted gitlab", "https://git.acme.internal/acme/fleet.git", domain.ConnectionGitLab,
			"https://git.acme.internal/api/v4/projects/acme%2Ffleet/repository/branches/main"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			owner, repo, ok := parseRepositoryPath(tc.repoURL)
			if !ok {
				t.Fatalf("could not parse %q", tc.repoURL)
			}

			endpoint, header, _ := branchEndpoint(tc.kind, tc.repoURL, owner, repo, "main")
			if endpoint != tc.want {
				t.Fatalf("endpoint = %q, want %q", endpoint, tc.want)
			}
			if header == "" {
				t.Fatal("no auth header for a provider that needs one")
			}
		})
	}
}

// TestBranchEndpointGenericIsUnverifiable pins that a generic host has no branch API,
// which the caller turns into a reachability probe rather than a silent accept.
func TestBranchEndpointGenericIsUnverifiable(t *testing.T) {
	endpoint, header, _ := branchEndpoint(domain.ConnectionGeneric,
		"https://git.acme.internal/acme/fleet.git", "acme", "fleet", "main")
	if endpoint != "" || header != "" {
		t.Fatalf("generic host produced endpoint %q header %q, want both empty", endpoint, header)
	}
}
