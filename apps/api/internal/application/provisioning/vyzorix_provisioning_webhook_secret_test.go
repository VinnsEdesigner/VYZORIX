package provisioning

import (
	"context"
	"strings"
	"testing"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

const whProbeSecret = "whsec_probe_secret_value"

// TestCreateStoresTheWebhookSecret proves a repository created with a secret stores it,
// so a provider's push can be verified afterwards.
func TestCreateStoresTheWebhookSecret(t *testing.T) {
	repos := newFakeRepositoryStore()
	conns := newFakeConnectionStore()
	conns.connections["conn-1"] = &domain.Connection{
		ID: "conn-1", OrgID: "org-1", Name: "gh", Type: domain.ConnectionGitHub,
	}
	conns.tokens["conn-1"] = "token"

	svc := NewRepositoryService(repos, conns, nil, nil)

	repo, err := svc.Create(context.Background(), "org-1", RepositoryInput{
		Actor:         domain.SystemActor(),
		Name:          "fleet",
		URL:           "https://github.com/acme/fleet.git",
		ConnectionID:  "conn-1",
		WebhookSecret: redaction.NewSecret(whProbeSecret),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if repos.webhookSecrets[repo.ID] != whProbeSecret {
		t.Fatalf("webhook secret = %q, want it stored", repos.webhookSecrets[repo.ID])
	}
}

// TestRenameKeepsTheWebhookSecret is the regression guard for the Secret-specific hazard:
// a PATCH that omits the secret must not clear it, or a rename silently disables the
// repository's webhook.
func TestRenameKeepsTheWebhookSecret(t *testing.T) {
	repos := newFakeRepositoryStore()
	conns := newFakeConnectionStore()
	conns.connections["conn-1"] = &domain.Connection{
		ID: "conn-1", OrgID: "org-1", Name: "gh", Type: domain.ConnectionGitHub,
	}
	conns.tokens["conn-1"] = "token"

	svc := NewRepositoryService(repos, conns, nil, nil)

	repo, err := svc.Create(context.Background(), "org-1", RepositoryInput{
		Actor:         domain.SystemActor(),
		Name:          "fleet",
		URL:           "https://github.com/acme/fleet.git",
		ConnectionID:  "conn-1",
		WebhookSecret: redaction.NewSecret(whProbeSecret),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := svc.Update(context.Background(), "org-1", repo.ID, RepositoryInput{
		Actor: domain.SystemActor(),
		Title: "Renamed",
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if repos.webhookSecrets[repo.ID] != whProbeSecret {
		t.Fatal("a rename cleared the webhook secret")
	}
}

// TestWebhookSecretNeverAppearsInTheAuditDetail proves the audit row records that a
// secret was configured, not the secret.
func TestWebhookSecretNeverAppearsInTheAuditDetail(t *testing.T) {
	detail := webhookDetail("repository created", redaction.NewSecret(whProbeSecret))

	if strings.Contains(detail, whProbeSecret) {
		t.Fatalf("the audit detail leaked the secret: %q", detail)
	}
	if !strings.Contains(detail, "webhook secret set") {
		t.Fatalf("the detail does not record that a secret was set: %q", detail)
	}

	// No secret supplied: the detail stays plain.
	if got := webhookDetail("repository updated", ""); got != "repository updated" {
		t.Fatalf("detail = %q, want the base message", got)
	}
}
