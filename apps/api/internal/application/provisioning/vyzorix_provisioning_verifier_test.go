package provisioning

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// TestVerifierAcceptsGenericRootServedRepo covers a real defect: the verifier parsed
// "owner/repository" out of the URL before it ever looked at the provider kind, so a generic
// git host serving a repository from its root — "http://host/device-config.git", the shape a
// self-hosted server or a LAN box produces — was rejected as unreachable. It is reachable;
// only a provider with a branch API needs the owner segment.
func TestVerifierAcceptsGenericRootServedRepo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// checkReachable issues a HEAD.
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	verifier := NewHTTPRemoteVerifier()

	err := verifier.Verify(
		context.Background(),
		server.URL+"/device-config.git",
		"main",
		domain.ConnectionGeneric,
		redaction.NewSecret(""),
	)
	if err != nil {
		t.Fatalf("a root-served generic repository was refused as unreachable: %v", err)
	}
}

// A provider WITH a branch API still needs an owner/repository, so a malformed URL is still a
// refusal — the fix must not turn every bad GitHub URL into a silent success.
func TestVerifierStillRefusesUnparseableProviderURL(t *testing.T) {
	verifier := NewHTTPRemoteVerifier()

	err := verifier.Verify(
		context.Background(),
		"https://github.com/only-a-name",
		"main",
		domain.ConnectionGitHub,
		redaction.NewSecret(""),
	)
	if err == nil {
		t.Fatal("a GitHub URL with no repository was accepted; it should be refused")
	}
}

// The generic check is genuinely a reachability check: an unreachable host must still fail,
// or a typo'd hostname would be stored and never sync.
func TestVerifierRefusesUnreachableGenericHost(t *testing.T) {
	verifier := NewHTTPRemoteVerifier()

	// A server that is started and immediately closed gives a URL nothing answers on.
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := server.URL + "/device-config.git"
	server.Close()

	err := verifier.Verify(context.Background(), url, "main", domain.ConnectionGeneric, redaction.NewSecret(""))
	if err == nil {
		t.Fatal("an unreachable generic host was accepted")
	}
}
