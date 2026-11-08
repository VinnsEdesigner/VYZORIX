package middleware

import "testing"

// TestIsDeployPath pins the scope of the deploy credential. The other
// authenticators defer to the credential only on these paths, so a bug here
// re-opens the hole where an unknown X-Vyzorix-Deploy-Credential value suppresses
// the session and API-key checks on every tenant route.
func TestIsDeployPath(t *testing.T) {
	t.Parallel()

	deployPaths := []string{
		"/v1/deploy",
		"/v1/deploy/",
		"/v1/deploy/d_01H",
		"/v1/deploy/d_01H/document",
		"/v1/deploy/d_01H/credentials",
		"/v1/deploy/schema/3",
	}
	for _, path := range deployPaths {
		if !IsDeployPath(path) {
			t.Errorf("%q is a deploy path and must be recognised as one", path)
		}
	}

	otherPaths := []string{
		"",
		"/",
		"/v1",
		"/v1/deployfoo",
		"/v1/devices",
		"/v1/devices/d_01H",
		"/v1/organizations/org_01H",
		"/v1/auth/sessions",
	}
	for _, path := range otherPaths {
		if IsDeployPath(path) {
			t.Errorf("%q is not a deploy path; accepting it would let the deploy "+
				"credential header suppress auth elsewhere", path)
		}
	}
}
