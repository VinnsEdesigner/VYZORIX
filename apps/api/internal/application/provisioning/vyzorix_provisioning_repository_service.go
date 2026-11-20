package provisioning

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
	"github.com/google/uuid"
)

var (
	// ErrInvalidRepository is returned when a repository's fields are unusable.
	ErrInvalidRepository = errors.New("invalid provisioning repository")
	// ErrRemoteUnreachable is returned when the repository's remote cannot be
	// reached with its connection's credential.
	ErrRemoteUnreachable = errors.New("repository remote is not reachable")
)

// MinSyncIntervalSeconds bounds the interval. A shorter one would hammer the
// provider's API for no benefit, since a device re-fetches on reconnect anyway.
const MinSyncIntervalSeconds = 60

// DefaultSyncIntervalSeconds is the interval a repository gets when none is given.
const DefaultSyncIntervalSeconds = 300

// RepositoryStore is the persistence the repository service needs.
type RepositoryStore interface {
	SaveRepository(ctx context.Context, r *domain.Repository) error
	GetRepository(ctx context.Context, orgID, id string) (*domain.Repository, error)
	ListRepositories(ctx context.Context, orgID string) ([]*domain.Repository, error)
	DeleteRepository(ctx context.Context, orgID, id string, keepResources bool) error
	// SetRepositoryWebhookSecret stores the webhook secret, encrypted. It is separate
	// from SaveRepository so a rename cannot blank a secret the caller did not send.
	SetRepositoryWebhookSecret(ctx context.Context, repoID string, secret redaction.Secret) error
}

// RemoteVerifier checks that a repository URL and branch exist and are readable
// with the connection's credential. It is an interface so the service can be
// tested without a network.
type RemoteVerifier interface {
	Verify(ctx context.Context, repoURL, branch string, kind domain.ConnectionType, token redaction.Secret) error
}

// RepositoryInput is the mutable surface of a repository.
//
// Fields are ordered widest-alignment first (slices, then strings, then the pointer, bool
// and int) so the struct packs without padding, which the fieldalignment check enforces.
type RepositoryInput struct {
	Enabled *bool
	Actor   domain.Actor
	// WebhookSecret sets the HMAC secret a provider signs pushes with. Its zero value
	// means "leave the stored secret alone", so a PATCH that only renames a repository
	// cannot silently disable its webhook. Clearing needs the explicit ClearWebhook flag.
	WebhookSecret redaction.Secret
	Title         string
	Name          string
	Description   string
	URL           string
	Branch        string
	Path          string
	ConnectionID  string
	SyncTarget    domain.SyncTarget
	// Include and Exclude are the repository's scope patterns. A nil slice means "not
	// supplied" on an update, so a PATCH that only renames cannot silently clear them;
	// an explicit empty slice clears the filter.
	Include         []string
	Exclude         []string
	IntervalSeconds int
	// ClearWebhook removes the stored webhook secret. It is separate from sending an empty
	// WebhookSecret because empty already means "unchanged": without a distinct signal a
	// user could never turn a webhook off, only replace it.
	ClearWebhook bool
}

// RepositoryService manages repositories.
type RepositoryService struct {
	store       RepositoryStore
	connections ConnectionStore
	verifier    RemoteVerifier
	auditor     *Auditor
	log         *slog.Logger
}

// SetAuditor injects the audit trail. Optional: without it the service still works,
// it just records no events.
func (s *RepositoryService) SetAuditor(a *Auditor) { s.auditor = a }

// audit records one repository event.
func (s *RepositoryService) audit(ctx context.Context, actor domain.Actor, typ domain.EventType,
	repo *domain.Repository, detail string) {
	if s.auditor == nil || repo == nil {
		return
	}

	s.auditor.Audit(ctx, actor, typ, repo.OrgID, "repository", repo.ID, "success", detail)
}

// NewRepositoryService builds a RepositoryService.
func NewRepositoryService(
	store RepositoryStore,
	connections ConnectionStore,
	verifier RemoteVerifier,
	log *slog.Logger,
) *RepositoryService {
	if log == nil {
		log = slog.Default()
	}

	return &RepositoryService{store: store, connections: connections, verifier: verifier, log: log}
}

// List returns an organisation's repositories.
func (s *RepositoryService) List(ctx context.Context, orgID string) ([]*domain.Repository, error) {
	return s.store.ListRepositories(ctx, orgID)
}

// Get returns one repository.
func (s *RepositoryService) Get(ctx context.Context, orgID, id string) (*domain.Repository, error) {
	return s.store.GetRepository(ctx, orgID, id)
}

// Create validates, verifies the remote, and stores a repository.
//
// The remote is verified BEFORE the row is written. A stored repository that
// cannot be reached is a permanently red row whose only fix is delete-and-retry;
// checking first turns a typo into an error message on the form instead.
func (s *RepositoryService) Create(ctx context.Context, orgID string, in RepositoryInput) (*domain.Repository, error) {
	if err := validateRepositoryInput(in, true); err != nil {
		return nil, err
	}

	connection, err := s.connections.GetConnection(ctx, orgID, in.ConnectionID)
	if err != nil {
		if errors.Is(err, domain.ErrConnectionNotFound) {
			return nil, fmt.Errorf("%w: connection not found", ErrInvalidRepository)
		}
		return nil, err
	}

	token, err := s.connections.ConnectionToken(ctx, orgID, connection.ID)
	if err != nil {
		return nil, err
	}

	if err := s.verify(ctx, in.URL, in.Branch, connection.Type, token); err != nil {
		return nil, err
	}

	now := time.Now()
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}

	repo := &domain.Repository{
		ID:              uuid.New().String(),
		OrgID:           orgID,
		ConnectionID:    connection.ID,
		Name:            strings.TrimSpace(in.Name),
		Title:           strings.TrimSpace(in.Title),
		Description:     strings.TrimSpace(in.Description),
		Type:            connection.Type,
		URL:             strings.TrimSpace(in.URL),
		Branch:          branchOrDefault(in.Branch),
		Path:            pathOrDefault(in.Path),
		Include:         in.Include,
		Exclude:         in.Exclude,
		SyncTarget:      syncTargetOrDefault(in.SyncTarget),
		IntervalSeconds: intervalOrDefault(in.IntervalSeconds),
		Enabled:         enabled,
		SyncState:       domain.SyncPending,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.store.SaveRepository(ctx, repo); err != nil {
		return nil, duplicateNameAsInvalid(err, repo.Name)
	}

	if err := s.applyWebhookSecret(ctx, repo.ID, in.WebhookSecret, in.ClearWebhook); err != nil {
		return nil, err
	}

	s.audit(ctx, in.Actor, domain.EventRepositoryCreated, repo, webhookDetail("repository created", in.WebhookSecret))

	return repo, nil
}

// Update changes the mutable fields. A URL, branch, or connection change is
// re-verified, because that is exactly the change that can break a repository.
func (s *RepositoryService) Update(ctx context.Context, orgID, id string, in RepositoryInput) (*domain.Repository, error) {
	repo, err := s.store.GetRepository(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	if err := validateRepositoryInput(in, false); err != nil {
		return nil, err
	}

	remoteChanged := (in.URL != "" && in.URL != repo.URL) ||
		(in.Branch != "" && branchOrDefault(in.Branch) != repo.Branch) ||
		(in.ConnectionID != "" && in.ConnectionID != repo.ConnectionID)

	if in.ConnectionID != "" && in.ConnectionID != repo.ConnectionID {
		connection, err := s.connections.GetConnection(ctx, orgID, in.ConnectionID)
		if err != nil {
			if errors.Is(err, domain.ErrConnectionNotFound) {
				return nil, fmt.Errorf("%w: connection not found", ErrInvalidRepository)
			}

			return nil, err
		}
		repo.ConnectionID = connection.ID
		repo.Type = connection.Type
	}

	applyRepositoryInput(repo, in)

	if remoteChanged {
		connection, err := s.connections.GetConnection(ctx, orgID, repo.ConnectionID)
		if err != nil {
			return nil, err
		}
		token, err := s.connections.ConnectionToken(ctx, orgID, connection.ID)
		if err != nil {
			return nil, err
		}
		if err := s.verify(ctx, repo.URL, repo.Branch, connection.Type, token); err != nil {
			return nil, err
		}
	}

	repo.UpdatedAt = time.Now()
	if err := s.store.SaveRepository(ctx, repo); err != nil {
		return nil, duplicateNameAsInvalid(err, repo.Name)
	}

	if err := s.applyWebhookSecret(ctx, repo.ID, in.WebhookSecret, in.ClearWebhook); err != nil {
		return nil, err
	}

	s.audit(ctx, in.Actor, domain.EventRepositoryUpdated, repo, webhookDetail("repository updated", in.WebhookSecret))

	return repo, nil
}

// applyRepositoryInput copies the non-empty fields of an input onto a repository.
//
// An empty field means "not supplied", which is why each assignment is guarded:
// PATCH semantics let a caller change one field without resending the rest.
func applyRepositoryInput(repo *domain.Repository, in RepositoryInput) {
	if in.Name != "" {
		repo.Name = strings.TrimSpace(in.Name)
	}
	if in.Title != "" {
		repo.Title = strings.TrimSpace(in.Title)
	}
	if in.Description != "" {
		repo.Description = strings.TrimSpace(in.Description)
	}
	if in.URL != "" {
		repo.URL = strings.TrimSpace(in.URL)
	}
	if in.Branch != "" {
		repo.Branch = branchOrDefault(in.Branch)
	}
	if in.Path != "" {
		repo.Path = pathOrDefault(in.Path)
	}
	if in.Include != nil {
		repo.Include = in.Include
	}
	if in.Exclude != nil {
		repo.Exclude = in.Exclude
	}
	if in.SyncTarget != "" {
		repo.SyncTarget = in.SyncTarget
	}
	if in.IntervalSeconds != 0 {
		repo.IntervalSeconds = intervalOrDefault(in.IntervalSeconds)
	}
	if in.Enabled != nil {
		applyEnabled(repo, *in.Enabled)
	}
}

// applyEnabled sets the sync switch and moves the state to match.
func applyEnabled(repo *domain.Repository, enabled bool) {
	repo.Enabled = enabled

	switch {
	case !enabled:
		repo.SyncState = domain.SyncDisabled
	case repo.SyncState == domain.SyncDisabled:
		repo.SyncState = domain.SyncPending
	}
}

// Delete removes a repository. keepResources releases the devices it owned instead
// of deleting their ownership rows; either way the devices keep whatever config
// they already run.
func (s *RepositoryService) Delete(ctx context.Context, orgID, id string, keepResources bool, actor domain.Actor) error {
	repo, err := s.store.GetRepository(ctx, orgID, id)
	if err != nil {
		return err
	}

	if err := s.store.DeleteRepository(ctx, orgID, id, keepResources); err != nil {
		return err
	}

	detail := "repository deleted"
	if keepResources {
		detail = "repository deleted, device ownership released"
	}
	s.audit(ctx, actor, domain.EventRepositoryDeleted, repo, detail)

	return nil
}

// applyWebhookSecret stores or clears the webhook secret.
//
// A zero Secret with ClearWebhook false means "not supplied" and leaves the stored one
// alone, so a PATCH that only renames a repository cannot silently disable its webhook.
func (s *RepositoryService) applyWebhookSecret(ctx context.Context, repoID string, secret redaction.Secret, shouldClear bool) error {
	if secret.IsZero() && !shouldClear {
		return nil
	}

	if shouldClear {
		secret = ""
	}

	return s.store.SetRepositoryWebhookSecret(ctx, repoID, secret)
}

// webhookDetail records whether a webhook was configured, never the secret itself.
func webhookDetail(base string, secret redaction.Secret) string {
	if secret.IsZero() {
		return base
	}

	return base + ", webhook secret set"
}

// verify probes the remote. A nil verifier accepts without checking, which keeps
// an offline test path honest rather than silently passing a real check.
func (s *RepositoryService) verify(ctx context.Context, repoURL, branch string, kind domain.ConnectionType, token redaction.Secret) error {
	if s.verifier == nil {
		return nil
	}

	return s.verifier.Verify(ctx, repoURL, branch, kind, token)
}

func validateRepositoryInput(in RepositoryInput, create bool) error {
	if create {
		if strings.TrimSpace(in.Name) == "" {
			return fmt.Errorf("%w: name is required", ErrInvalidRepository)
		}
		if strings.TrimSpace(in.URL) == "" {
			return fmt.Errorf("%w: repository URL is required", ErrInvalidRepository)
		}
		if strings.TrimSpace(in.ConnectionID) == "" {
			return fmt.Errorf("%w: connection is required", ErrInvalidRepository)
		}
	}

	if in.URL != "" && !validRepositoryURL(in.URL) {
		return fmt.Errorf("%w: URL must be an https or ssh git URL", ErrInvalidRepository)
	}
	if in.SyncTarget != "" && !in.SyncTarget.Valid() {
		return fmt.Errorf("%w: unknown sync target %q", ErrInvalidRepository, in.SyncTarget)
	}
	if in.IntervalSeconds != 0 && in.IntervalSeconds < MinSyncIntervalSeconds {
		return fmt.Errorf("%w: interval must be at least %d seconds", ErrInvalidRepository, MinSyncIntervalSeconds)
	}
	if err := ValidateScope(in.Include, in.Exclude); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRepository, err)
	}

	return nil
}

// validRepositoryURL accepts an https URL or an scp-style ssh address. A plain
// path is refused: a local directory is not a remote a server can pull from.
//
// A URL carrying userinfo (https://user:token@host/...) is refused. The credential
// belongs in the connection, where it is encrypted and never returned; embedded in the
// URL it would be stored in the clear, echoed back by every repository read, and
// repeated in git's error output. The fetcher goes out of its way to keep the token out
// of the URL, so accepting one here would silently undo that.
func validRepositoryURL(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}

	if strings.HasPrefix(raw, "git@") {
		return strings.Contains(raw, ":")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return false
	}

	if u.Scheme == "ssh" {
		// ssh://git@host/path is the conventional form: the username names the account,
		// and auth is by key. A password in the URL is a credential in a place that gets
		// logged, so it is refused.
		if pw, hasPassword := passwordOf(u); hasPassword && pw != "" {
			return false
		}

		return u.Host != "" && strings.Trim(u.Path, "/") != ""
	}

	// On http(s) any userinfo is a credential: the token belongs in the connection.
	if u.User != nil {
		return false
	}

	return (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

// passwordOf reports the userinfo password, if the URL carries userinfo at all.
func passwordOf(u *url.URL) (string, bool) {
	if u.User == nil {
		return "", false
	}

	return u.User.Password()
}

// duplicateNameAsInvalid turns the schema's UNIQUE(org_id, name) violation into a
// validation error. Left raw it reached the handler's default branch and became a 500,
// telling the user the server had broken rather than that the name was taken.
func duplicateNameAsInvalid(err error, name string) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return fmt.Errorf("%w: a repository named %q already exists", ErrInvalidRepository, name)
	}

	return err
}

func branchOrDefault(branch string) string {
	if b := strings.TrimSpace(branch); b != "" {
		return b
	}

	return "main"
}

// pathOrDefault normalises a subdirectory to a leading-slash form, so "/" and ""
// and "apps" and "/apps" cannot become four different values.
func pathOrDefault(path string) string {
	p := strings.TrimSpace(path)
	if p == "" || p == "/" {
		return "/"
	}

	return "/" + strings.Trim(p, "/")
}

func syncTargetOrDefault(target domain.SyncTarget) domain.SyncTarget {
	if target == "" {
		return domain.SyncTargetDevice
	}

	return target
}

func intervalOrDefault(seconds int) int {
	if seconds <= 0 {
		return DefaultSyncIntervalSeconds
	}

	return seconds
}
