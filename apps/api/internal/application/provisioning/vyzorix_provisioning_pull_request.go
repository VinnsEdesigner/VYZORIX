package provisioning

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// HTTPPullRequester opens a pull request through the provider's REST API.
//
// It uses the API rather than a git command because a pull request is not a git concept:
// only the provider knows how to open one, and the API is the documented way to ask.
type HTTPPullRequester struct {
	client *http.Client
}

// NewHTTPPullRequester builds a requester with a bounded timeout.
func NewHTTPPullRequester() *HTTPPullRequester {
	return &HTTPPullRequester{client: &http.Client{Timeout: 15 * time.Second}}
}

// OpenPullRequest opens a pull request and returns its URL.
//
// A provider that is not GitHub or GitLab is refused explicitly: a generic git host has no
// portable pull-request API, and guessing would produce a silent no-op where the user
// believes a review is pending.
func (r *HTTPPullRequester) OpenPullRequest(
	ctx context.Context, req PullRequest, token redaction.Secret,
) (string, error) {
	owner, repo, ok := parseRepositoryPath(req.Repo.URL)
	if !ok {
		return "", fmt.Errorf("%w: cannot read an owner/repository from %q", ErrPushFailed, req.Repo.URL)
	}

	endpoint, headers, body, ok := pullRequestBody(req.Repo.Type, req.Repo.URL, owner, repo, req, token)
	if !ok {
		return "", fmt.Errorf("%w: %s does not support pull requests", ErrPushFailed, req.Repo.Type)
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "vyzorix-provisioning")

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrPushFailed, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("%w: the provider answered HTTP %d", ErrPushFailed, resp.StatusCode)
	}

	return pullRequestURL(resp, req.Repo.Type), nil
}

// pullRequestBody builds the endpoint, headers, and JSON body for a provider.
//
// The endpoint follows the repository's own host so a GitHub Enterprise or self-hosted
// GitLab instance is addressed on its own API path rather than the public provider's.
func pullRequestBody(
	kind domain.ConnectionType, repoURL, owner, repo string, req PullRequest, token redaction.Secret,
) (string, map[string]string, map[string]string, bool) {
	base := apiBase(repoURL, kind)

	switch kind {
	case domain.ConnectionGitHub:
		endpoint := fmt.Sprintf("%s/repos/%s/%s/pulls", base, owner, repo)
		if base != "https://api.github.com" {
			endpoint = fmt.Sprintf("%s/api/v3/repos/%s/%s/pulls", base, owner, repo)
		}

		return endpoint,
			map[string]string{
				"Authorization":        "Bearer " + token.Reveal(),
				"X-GitHub-Api-Version": "2022-11-28",
			},
			map[string]string{
				"title": req.Title,
				"head":  req.Head,
				"base":  req.Base,
				"body":  req.Body,
			},
			true
	case domain.ConnectionGitLab:
		return fmt.Sprintf("%s/api/v4/projects/%s/merge_requests", base, url.PathEscape(owner+"/"+repo)),
			map[string]string{"PRIVATE-TOKEN": token.Reveal()},
			map[string]string{
				"source_branch": req.Head,
				"target_branch": req.Base,
				"title":         req.Title,
				"description":   req.Body,
			},
			true
	default:
		return "", nil, nil, false
	}
}

// pullRequestURL reads the web URL out of a provider's response.
//
// It returns only a string, because a missing URL is not a failure: the request was
// accepted, the branch is on the remote, and the URL is a convenience. The two providers
// name the field differently.
func pullRequestURL(resp *http.Response, kind domain.ConnectionType) string {
	var decoded map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return ""
	}

	field := "html_url"
	if kind == domain.ConnectionGitLab {
		field = "web_url"
	}

	url, _ := decoded[field].(string)

	return url
}
