package provisioning

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// HTTPRemoteVerifier confirms a repository URL and branch exist, using the
// provider's API rather than cloning. A clone would download the whole history to
// answer a yes/no question.
type HTTPRemoteVerifier struct {
	client *http.Client
}

// NewHTTPRemoteVerifier builds a verifier with a bounded timeout.
func NewHTTPRemoteVerifier() *HTTPRemoteVerifier {
	return &HTTPRemoteVerifier{client: &http.Client{Timeout: 15 * time.Second}}
}

// Verify checks that the repository is readable and the branch exists.
//
// A generic git host has no portable branch API, so reachability is the only check possible
// without cloning — and that check must not require the URL to look like "owner/repository".
// A self-hosted server may serve "http://host/device-config.git" from its root, which has no
// owner segment; demanding one rejected every such host as unreachable.
func (v *HTTPRemoteVerifier) Verify(
	ctx context.Context, repoURL, branch string, kind domain.ConnectionType, token redaction.Secret,
) error {
	if kind == domain.ConnectionGeneric {
		return v.checkReachable(ctx, repoURL)
	}

	owner, name, ok := parseRepositoryPath(repoURL)
	if !ok {
		return fmt.Errorf("%w: cannot read an owner/repository from %q", ErrRemoteUnreachable, repoURL)
	}

	endpoint, header, authValue := branchEndpoint(kind, repoURL, owner, name, branch)
	if endpoint == "" {
		// A host with no portable branch API: accepting anything would let a typo'd
		// hostname become a repository that can never sync, so reachability is the check.
		return v.checkReachable(ctx, repoURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRemoteUnreachable, err)
	}
	if !token.IsZero() {
		req.Header.Set(header, authValue(token.Reveal()))
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "vyzorix-provisioning")

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRemoteUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("%w: repository or branch %q not found", ErrRemoteUnreachable, branch)
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%w: the provider rejected the credential (HTTP %d)", ErrRemoteUnreachable, resp.StatusCode)
	default:
		return fmt.Errorf("%w: the provider answered HTTP %d", ErrRemoteUnreachable, resp.StatusCode)
	}
}

// checkReachable confirms the host resolves and answers, the only verification
// a generic git host allows without cloning.
func (v *HTTPRemoteVerifier) checkReachable(ctx context.Context, repoURL string) error {
	probe, err := http.NewRequestWithContext(ctx, http.MethodHead, repoURL, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRemoteUnreachable, err)
	}
	probe.Header.Set("User-Agent", "vyzorix-provisioning")

	resp, err := v.client.Do(probe)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrRemoteUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()

	// A private repository answers 401/403 to an unauthenticated HEAD, which
	// still proves the host is real. Only a transport failure or 5xx is a
	// reachability problem.
	if resp.StatusCode >= 500 {
		return fmt.Errorf("%w: the host answered HTTP %d", ErrRemoteUnreachable, resp.StatusCode)
	}

	return nil
}

// branchEndpoint returns the URL that answers "does this branch exist" and the
// header carrying the credential. A generic host returns empty, meaning
// "unverifiable without cloning".
func branchEndpoint(kind domain.ConnectionType, repoURL, owner, repo, branch string) (endpoint, header string, auth func(string) string) {
	base := apiBase(repoURL, kind)

	switch kind {
	case domain.ConnectionGitHub:
		if base == "https://api.github.com" {
			return fmt.Sprintf("%s/repos/%s/%s/branches/%s",
					base, url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(branch)),
				"Authorization",
				func(token string) string { return "Bearer " + token }
		}

		// A GitHub Enterprise host serves the same API under /api/v3.
		return fmt.Sprintf("%s/api/v3/repos/%s/%s/branches/%s",
				base, url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(branch)),
			"Authorization",
			func(token string) string { return "Bearer " + token }
	case domain.ConnectionGitLab:
		return fmt.Sprintf("%s/api/v4/projects/%s/repository/branches/%s",
				base, url.PathEscape(owner+"/"+repo), url.PathEscape(branch)),
			"PRIVATE-TOKEN",
			func(token string) string { return token }
	default:
		return "", "", nil
	}
}

// apiBase reduces a repository URL to the API origin for its provider.
//
// The API base follows the repository's own host rather than a hardcoded provider
// domain: a GitHub Enterprise or self-hosted GitLab instance answers on its own /api
// path, and sending the request to api.github.com would report a valid private
// repository on that host as unreachable.
func apiBase(repoURL string, kind domain.ConnectionType) string {
	u, err := url.Parse(repoURL)
	host := ""
	if err == nil {
		host = u.Host
	}
	if host == "" {
		if kind == domain.ConnectionGitHub {
			return "https://api.github.com"
		}

		return "https://gitlab.com"
	}

	if kind == domain.ConnectionGitHub && (host == "github.com" || host == "www.github.com") {
		return "https://api.github.com"
	}

	scheme := "https"
	if u.Scheme == "http" {
		scheme = "http"
	}

	return scheme + "://" + host
}

// parseRepositoryPath extracts owner and repository from an https or ssh URL.
//
// Accepted forms: https://host/owner/repo(.git), git@host:owner/repo(.git),
// ssh://git@host/owner/repo(.git).
func parseRepositoryPath(raw string) (owner, repo string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}

	var path string

	switch {
	case strings.HasPrefix(raw, "git@"):
		idx := strings.Index(raw, ":")
		if idx < 0 {
			return "", "", false
		}
		path = raw[idx+1:]
	case strings.HasPrefix(raw, "ssh://"):
		u, err := url.Parse(raw)
		if err != nil {
			return "", "", false
		}
		path = u.Path
	default:
		u, err := url.Parse(raw)
		if err != nil {
			return "", "", false
		}
		path = u.Path
	}

	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", "", false
	}

	return parts[len(parts)-2], parts[len(parts)-1], true
}
