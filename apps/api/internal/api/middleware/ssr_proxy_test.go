package middleware

import "testing"

// TestIsStaticPathClassifiesApiRoutes is the regression guard for a routing
// defect: the SSR middleware is mounted globally (engine.Use), so it runs before
// route matching. Any API path it does not recognise as "not an app route" is
// treated as an app page and redirected to /login. The GraphQL subscription
// endpoint was missing from the list, so a WebSocket upgrade to
// /:org/graphql/ws got a 307 to /login instead of reaching its handler.
func TestIsStaticPathClassifiesApiRoutes(t *testing.T) {
	apiPaths := []string{
		"/api/anything",
		"/v1/devices",
		"/health",
		"/healthz",
		"/metrics",
		"/bin/app.apk",
		// The GraphQL endpoints, which are API routes even though they are not
		// under a REST prefix.
		"/org-1/graphql",
		"/org-1/graphql/batch",
		"/org-1/graphql/ws",
		"/org-1/playground",
		"/favicon.ico",
	}
	for _, p := range apiPaths {
		if !isStaticPath(p) {
			t.Errorf("isStaticPath(%q) = false, want true (the API serves it)", p)
		}
	}

	appPaths := []string{
		"/",
		"/devices",
		"/admin/default-preferences",
		"/login",
		"/profile",
		// A nested app route that merely contains the word "graphql" must not be
		// mistaken for the API's org-scoped endpoint.
		"/admin/provisioning/graphql",
		"/org-1/graphql/ws/extra",
	}
	for _, p := range appPaths {
		if isStaticPath(p) {
			t.Errorf("isStaticPath(%q) = true, want false (the web app serves it)", p)
		}
	}
}

func TestIsGraphQLPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/org-1/graphql", true},
		{"/org-1/graphql/batch", true},
		{"/org-1/graphql/ws", true},
		{"/org-1/playground", true},
		// Two segments only: the org, then the endpoint.
		{"/graphql", false},
		{"/a/b/graphql", false},
		{"/org-1/devices/graphql", false},
		{"/org-1/graphql/ws/extra", false},
		{"/", false},
		{"", false},
	}
	for _, tc := range tests {
		if got := isGraphQLPath(tc.path); got != tc.want {
			t.Errorf("isGraphQLPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
