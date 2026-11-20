package provisioning

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// ErrWebhookUnknownRepository is returned when a webhook names a repository that does
// not exist. The receiver answers 404 without revealing whether the id is real but the
// secret wrong — that distinction is the secret.
var ErrWebhookUnknownRepository = errors.New("webhook names an unknown repository")

// WebhookRepositoryStore is the lookup the receiver needs.
type WebhookRepositoryStore interface {
	// GetRepositoryForWebhook resolves a repository from its id alone, without an org.
	// The webhook URL carries the id, so the receiver cannot require an org header it
	// will never receive.
	GetRepositoryForWebhook(ctx context.Context, id string) (*domain.Repository, error)
}

// WebhookSecretStore supplies and stores a repository's webhook secret.
type WebhookSecretStore interface {
	// RepositoryWebhookSecret returns the decrypted secret, or an empty Secret when the
	// repository has none configured.
	RepositoryWebhookSecret(ctx context.Context, repoID string) (redaction.Secret, error)
	// SetRepositoryWebhookSecret stores the secret, encrypted. An empty Secret clears it.
	SetRepositoryWebhookSecret(ctx context.Context, repoID string, secret redaction.Secret) error
}

// WebhookReceiver authenticates a provider's push and triggers a sync.
//
// It is deliberately thin. Verification lives in SignatureVerifier and the sync itself
// in SyncService, so this type only sequences them: resolve, verify, sync. The order is
// load-bearing — a repository is resolved first so the right secret is used, and the
// sync is triggered only after the signature is known good.
type WebhookReceiver struct {
	repos    WebhookRepositoryStore
	secrets  WebhookSecretStore
	verifier *SignatureVerifier
	sync     *SyncService
}

// NewWebhookReceiver builds a receiver.
func NewWebhookReceiver(
	repos WebhookRepositoryStore,
	secrets WebhookSecretStore,
	verifier *SignatureVerifier,
	syncSvc *SyncService,
) *WebhookReceiver {
	return &WebhookReceiver{repos: repos, secrets: secrets, verifier: verifier, sync: syncSvc}
}

// webhookSyncTimeout bounds a sync started by a webhook. It is generous enough for a
// first clone of a large repository, and finite so a hung remote cannot pin the goroutine
// that the provider's request has already abandoned.
const webhookSyncTimeout = 5 * time.Minute

// WebhookResult reports what a webhook did, so the handler can answer honestly.
type WebhookResult struct {
	// RepositoryID is the repository the push was for.
	RepositoryID string
	// Branch is the branch the provider reported, when it reported one.
	Branch string
	// Triggered is true when a sync ran. A push for a branch this repository does not
	// track is acknowledged but does not sync, which is not an error.
	Triggered bool
}

// Handle verifies a webhook request and triggers a sync for the repository.
//
// rawBody must be the bytes exactly as received: a provider signs what it sent, so a
// re-encoded body would not verify.
func (r *WebhookReceiver) Handle(
	ctx context.Context, repoID string, provider Provider, header http.Header, rawBody []byte,
) (*WebhookResult, error) {
	repo, err := r.repos.GetRepositoryForWebhook(ctx, repoID)
	if err != nil {
		if errors.Is(err, domain.ErrRepositoryNotFound) {
			return nil, ErrWebhookUnknownRepository
		}

		return nil, err
	}

	secret, err := r.secrets.RepositoryWebhookSecret(ctx, repo.ID)
	if err != nil {
		return nil, err
	}

	if err := r.verifier.Verify(header, rawBody, secret); err != nil {
		return nil, err
	}

	payload := ParseWebhookPayload(provider, rawBody)
	result := &WebhookResult{RepositoryID: repo.ID, Branch: payload.Branch}

	// A push to a branch this repository does not track is acknowledged, not synced.
	// Providers send every push in the repository; syncing on an unrelated branch would
	// pull a tree this repository never reads.
	if payload.Branch != "" && payload.Branch != repo.Branch {
		return result, nil
	}

	// The provider is the actor: nobody pressed anything, and the audit trail must say
	// so rather than attributing the change to the repository's owner.
	//
	// The sync does NOT run on the request context. A provider gives up on the webhook
	// after a few seconds, and a sync applies configs one device at a time; inheriting the
	// request's cancellation would abort the run halfway through the fleet, leaving the job
	// and the resources describing a state that no longer exists.
	syncCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), webhookSyncTimeout)
	defer cancel()

	if _, err := r.sync.Sync(syncCtx, repo, SyncOptions{Actor: ProviderActor(provider)}); err != nil {
		// Two outcomes are acknowledgements, not failures, because retrying cannot help:
		// a sync is already running (this push will be covered by it or the next interval),
		// or the repository's sync is switched off (a webhook must not override that).
		// Answering an error would make the provider retry a push nothing will ever act on.
		if errors.Is(err, domain.ErrSyncInProgress) || errors.Is(err, ErrSyncDisabled) {
			return result, nil
		}

		return nil, fmt.Errorf("webhook sync: %w", err)
	}

	result.Triggered = true

	return result, nil
}
