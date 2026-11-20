package provisioning

import (
	"encoding/json"
	"net/http"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// Provider is the source of a webhook push, for attribution.
type Provider string

const (
	ProviderGitHub  Provider = "github"
	ProviderGitLab  Provider = "gitlab"
	ProviderGeneric Provider = "generic"
)

// WebhookPayload is what a push webhook tells us.
//
// Only the branch is read. A provider's payload carries the whole push — commits, files,
// author details — and none of it is needed: the sync fetches the branch and reads the
// manifests itself. Parsing less means a payload change upstream cannot break the
// receiver, and a large body is not decoded twice.
type WebhookPayload struct {
	// Branch is the pushed branch, when the provider reports one.
	Branch string
}

// ParseWebhookPayload reads the branch out of a provider's push body.
//
// A body that does not parse, or that carries no ref, is not an error: the branch is
// optional, and an absent branch means "sync regardless", which is the safe direction —
// a push happened, and the sync re-reads the repository anyway.
func ParseWebhookPayload(_ Provider, body []byte) WebhookPayload {
	// Every push payload that matters reports the branch in `ref`: GitHub, GitLab, and
	// the generic form. One shape is enough, so the provider argument is not consulted.
	var push struct {
		Ref string `json:"ref"`
	}

	var ref string
	if err := json.Unmarshal(body, &push); err == nil {
		ref = push.Ref
	}

	return WebhookPayload{Branch: branchFromRef(ref)}
}

// branchFromRef turns "refs/heads/main" into "main". A ref that is not a branch — a tag,
// or an empty ref — yields empty, which the receiver treats as "sync regardless".
func branchFromRef(ref string) string {
	const prefix = "refs/heads/"

	if len(ref) > len(prefix) && ref[:len(prefix)] == prefix {
		return ref[len(prefix):]
	}

	return ""
}

// ProviderActor is a git provider's push webhook. The id names the provider so the audit
// row says which one pushed.
//
// It lives here rather than in the domain because Provider is an application-level
// concept; the domain only knows the ActorKind.
func ProviderActor(p Provider) domain.Actor {
	kind := string(p)
	if kind == "" {
		kind = "provider"
	}

	return domain.Actor{Kind: domain.ActorProvider, ID: kind, Label: kind + " webhook"}
}

// ProviderFromHeader infers the provider from the signature header a request carried.
//
// It is a hint for attribution and payload parsing only. Verification never trusts it:
// the secret is what proves the sender, and an unknown header falls back to the generic
// HMAC check.
func ProviderFromHeader(h http.Header) Provider {
	switch {
	case h.Get("X-Gitlab-Token") != "":
		return ProviderGitLab
	case h.Get("X-Hub-Signature-256") != "", h.Get("X-Hub-Signature") != "":
		return ProviderGitHub
	default:
		return ProviderGeneric
	}
}
