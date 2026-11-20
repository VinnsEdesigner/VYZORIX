package provisioning

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// HTTPChecker probes a git provider over HTTPS.
//
// It answers two questions at once: is the host reachable, and does the token
// authenticate? A 401 means the host is fine and the credential is not, which is
// the distinction the user needs — "unreachable" and "wrong token" call for
// different fixes.
type HTTPChecker struct {
	client *http.Client
}

// NewHTTPChecker builds a checker with a bounded timeout. An unbounded client
// would let a black-holed host hold a sync open indefinitely.
func NewHTTPChecker() *HTTPChecker {
	return &HTTPChecker{client: &http.Client{Timeout: 10 * time.Second}}
}

// Check probes the provider's API with the token.
func (c *HTTPChecker) Check(ctx context.Context, host string, kind domain.ConnectionType, token redaction.Secret) error {
	endpoint, header := probeTarget(host, kind)
	if endpoint == "" {
		return fmt.Errorf("%w: no host configured", ErrConnectionUnreachable)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionUnreachable, err)
	}
	if header != "" && !token.IsZero() {
		req.Header.Set(header, authValue(token.Reveal()))
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "vyzorix-provisioning")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("%w: the provider rejected the token (HTTP %d)", ErrConnectionUnreachable, resp.StatusCode)
	case resp.StatusCode >= 400:
		return fmt.Errorf("%w: the provider answered HTTP %d", ErrConnectionUnreachable, resp.StatusCode)
	}

	return nil
}

// probeTarget returns the URL to probe and the auth header to send.
//
// The URL is the provider's API, not its website. Probing github.com/user answers 200
// for any caller — the website serves a page to an anonymous request — so a dead token
// would read as healthy. The API endpoint answers 401 for the same token, which is the
// distinction the health check exists to draw.
func probeTarget(host string, kind domain.ConnectionType) (endpoint, header string) {
	base := host
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "https://" + base
	}
	base = strings.TrimSuffix(base, "/")

	switch kind {
	case domain.ConnectionGitHub:
		// api.github.com is the only host that authenticates a token. A GitHub
		// Enterprise host answers on /api/v3/user instead of the public API, so the
		// suffix follows the host.
		if base == "https://github.com" || base == "http://github.com" {
			return "https://api.github.com/user", "Authorization"
		}

		return base + "/api/v3/user", "Authorization"
	case domain.ConnectionGitLab:
		return base + "/api/v4/user", "Authorization"
	default:
		// A generic git host has no portable identity endpoint. Reachability is
		// the only thing that can be checked without cloning.
		return base, ""
	}
}

// authValue renders the token in the header form the provider expects.
//
// Both GitHub and GitLab accept the token in an Authorization: Bearer header. A bare
// token in Authorization is not a scheme either provider documents, so sending one
// unqualified would make a valid credential read as rejected.
func authValue(token string) string {
	return "Bearer " + token
}
