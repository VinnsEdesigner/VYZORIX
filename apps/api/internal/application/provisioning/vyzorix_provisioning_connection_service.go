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
	// ErrInvalidConnection is returned when a connection's fields are unusable.
	ErrInvalidConnection = errors.New("invalid provisioning connection")
	// ErrConnectionUnreachable is returned when a health check cannot reach the host.
	ErrConnectionUnreachable = errors.New("connection host is unreachable")
)

// ConnectionStore is the persistence the service needs. It is declared here, in
// the consumer, so the service does not depend on the storage package.
type ConnectionStore interface {
	SaveConnection(ctx context.Context, c *domain.Connection) error
	SaveConnectionWithToken(ctx context.Context, c *domain.Connection, token redaction.Secret) error
	GetConnection(ctx context.Context, orgID, id string) (*domain.Connection, error)
	ListConnections(ctx context.Context, orgID string) ([]*domain.Connection, error)
	DeleteConnection(ctx context.Context, orgID, id string) error
	CountRepositoriesUsing(ctx context.Context, orgID, connectionID string) (int, error)
	// ConnectionToken returns the decrypted credential. The Secret type is what keeps it
	// out of a log: only an explicit Reveal reaches a caller that must authenticate.
	ConnectionToken(ctx context.Context, orgID, id string) (redaction.Secret, error)
	// UpdateRepositoriesTypeForConnection re-stamps the provider type on the repositories
	// that use a connection, so a type change does not leave them pointing at the wrong
	// provider API.
	UpdateRepositoriesTypeForConnection(ctx context.Context, connectionID, typ string) error
}

// Checker probes a provider host. It is an interface so the service can be tested
// without a network, and so a provider that needs a different probe can be added
// without touching the service.
type Checker interface {
	// Check reports whether the host answers and the token authenticates.
	Check(ctx context.Context, host string, kind domain.ConnectionType, token redaction.Secret) error
}

// ConnectionInput is the mutable surface of a connection.
type ConnectionInput struct {
	// WriteEnabled declares the token has push scope. A nil means "not supplied", so a
	// rename cannot silently flip the flag either way.
	WriteEnabled *bool
	Actor        domain.Actor
	Name         string
	Type         domain.ConnectionType
	Host         string
	// Token is the credential to store. It is a Secret so it cannot be logged, and its
	// zero value means "not supplied" — which is how a rename avoids blanking it.
	Token redaction.Secret
}

// ConnectionService manages stored git credentials.
type ConnectionService struct {
	store   ConnectionStore
	checker Checker
	auditor *Auditor
	log     *slog.Logger
}

// SetAuditor injects the audit trail. Optional: without it the service still works,
// it just records no events.
func (s *ConnectionService) SetAuditor(a *Auditor) { s.auditor = a }

// NewConnectionService builds a ConnectionService. A nil checker marks every
// connection ready without probing, which keeps an offline test path honest.
func NewConnectionService(store ConnectionStore, checker Checker, log *slog.Logger) *ConnectionService {
	if log == nil {
		log = slog.Default()
	}

	return &ConnectionService{store: store, checker: checker, log: log}
}

// List returns an organisation's connections. No token material is included.
func (s *ConnectionService) List(ctx context.Context, orgID string) ([]*domain.Connection, error) {
	return s.store.ListConnections(ctx, orgID)
}

// Get returns one connection.
func (s *ConnectionService) Get(ctx context.Context, orgID, id string) (*domain.Connection, error) {
	return s.store.GetConnection(ctx, orgID, id)
}

// Create validates, stores, and health-checks a new connection.
//
// The check runs after the store so a failed check leaves a row the user can see
// and fix (a typo'd token becomes a red row with the reason) rather than a silent
// nothing.
func (s *ConnectionService) Create(ctx context.Context, orgID string, in ConnectionInput) (*domain.Connection, error) {
	if err := validateConnectionInput(in, true); err != nil {
		return nil, err
	}

	now := time.Now()
	c := &domain.Connection{
		ID:        uuid.New().String(),
		OrgID:     orgID,
		Name:      strings.TrimSpace(in.Name),
		Type:      in.Type,
		Host:      normaliseHost(in.Host),
		Status:    domain.ConnectionPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if in.WriteEnabled != nil {
		c.WriteEnabled = *in.WriteEnabled
	}

	if err := s.store.SaveConnectionWithToken(ctx, c, in.Token); err != nil {
		return nil, duplicateConnectionNameAsInvalid(err, c.Name)
	}

	// The token itself is never recorded: only that a credential was supplied.
	s.audit(ctx, in.Actor, domain.EventConnectionCreated, c, "connection created")

	return s.check(ctx, c)
}

// Update changes the name, host, or token. An empty token keeps the stored one,
// so a rename cannot silently blank the credential.
func (s *ConnectionService) Update(ctx context.Context, orgID, id string, in ConnectionInput) (*domain.Connection, error) {
	c, err := s.store.GetConnection(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	if err := validateConnectionInput(in, false); err != nil {
		return nil, err
	}
	typeChanged := in.Type != "" && in.Type != c.Type
	if in.Type != "" {
		c.Type = in.Type
	}
	if name := strings.TrimSpace(in.Name); name != "" {
		c.Name = name
	}
	if in.Host != "" {
		c.Host = normaliseHost(in.Host)
	}
	if in.WriteEnabled != nil {
		c.WriteEnabled = *in.WriteEnabled
	}
	c.UpdatedAt = time.Now()

	if err := s.store.SaveConnectionWithToken(ctx, c, in.Token); err != nil {
		return nil, duplicateConnectionNameAsInvalid(err, c.Name)
	}

	// A repository stores a copy of its connection's type and uses it to pick the provider
	// API and git username. Re-stamping here keeps every dependent repository in step
	// instead of leaving it on the previous provider.
	if typeChanged {
		if err := s.store.UpdateRepositoriesTypeForConnection(ctx, c.ID, string(c.Type)); err != nil {
			return nil, err
		}
	}

	// Re-check when the credential, host, or provider type changed; a rename alone leaves
	// the last result alone. A type change matters because the probe endpoint and the token
	// format are provider-specific: a status of "ready" earned against GitHub says nothing
	// about whether the same token works at GitLab, so leaving it green would be a stale
	// claim.
	detail := "connection updated"
	if !in.Token.IsZero() {
		detail = "connection updated and token rotated"
	}
	s.audit(ctx, in.Actor, domain.EventConnectionUpdated, c, detail)

	if !in.Token.IsZero() || in.Host != "" || typeChanged {
		return s.check(ctx, c)
	}

	return c, nil
}

// Delete removes a connection, refusing while a repository still uses it.
func (s *ConnectionService) Delete(ctx context.Context, orgID, id string, actor domain.Actor) error {
	if _, err := s.store.GetConnection(ctx, orgID, id); err != nil {
		return err
	}

	n, err := s.store.CountRepositoriesUsing(ctx, orgID, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("%w: %d repository(ies) still use it", domain.ErrConnectionInUse, n)
	}

	if err := s.store.DeleteConnection(ctx, orgID, id); err != nil {
		return err
	}

	s.audit(ctx, actor, domain.EventConnectionDeleted, &domain.Connection{ID: id, OrgID: orgID},
		"connection deleted")

	return nil
}

// CheckNow forces a health check and records the result.
func (s *ConnectionService) CheckNow(ctx context.Context, orgID, id string, actor domain.Actor) (*domain.Connection, error) {
	c, err := s.store.GetConnection(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	checked, err := s.check(ctx, c)
	if err != nil {
		return nil, err
	}

	// A check that found the credential rejected is not a success. Recording it as one
	// made the audit trail say the health check worked when what it proved is that the
	// token is dead — the opposite of what a reader is looking for.
	result := "success"
	if checked.Status == domain.ConnectionError {
		result = "error"
	}

	s.auditResult(ctx, actor, domain.EventConnectionChecked, checked, result,
		"connection health check: "+string(checked.Status))

	return checked, nil
}

// audit records one connection event. A nil auditor is a no-op, so a service built
// without one still works.
func (s *ConnectionService) audit(ctx context.Context, actor domain.Actor, typ domain.EventType,
	c *domain.Connection, detail string) {
	s.auditResult(ctx, actor, typ, c, "success", detail)
}

// auditResult records an event with an explicit result, for the actions where failure is
// an outcome worth recording rather than an error to return.
func (s *ConnectionService) auditResult(ctx context.Context, actor domain.Actor, typ domain.EventType,
	c *domain.Connection, result, detail string) {
	if s.auditor == nil || c == nil {
		return
	}

	s.auditor.Audit(ctx, actor, typ, c.OrgID, "connection", c.ID, result, detail)
}

// check probes the host and stamps the result on the connection.
func (s *ConnectionService) check(ctx context.Context, c *domain.Connection) (*domain.Connection, error) {
	now := time.Now()
	c.CheckedAt = &now
	c.UpdatedAt = now

	if s.checker == nil {
		c.Status = domain.ConnectionReady
		c.LastError = ""

		return c, s.store.SaveConnection(ctx, c)
	}

	token, err := s.store.ConnectionToken(ctx, c.OrgID, c.ID)
	if err != nil {
		return nil, err
	}

	if err := s.checker.Check(ctx, c.Host, c.Type, token); err != nil {
		c.Status = domain.ConnectionError
		c.LastError = err.Error()
		if saveErr := s.store.SaveConnection(ctx, c); saveErr != nil {
			return nil, saveErr
		}

		return c, nil
	}

	c.Status = domain.ConnectionReady
	c.LastError = ""

	return c, s.store.SaveConnection(ctx, c)
}

// validateConnectionInput checks the fields a caller supplied.
//
// On create every field is required. On update the caller sends only what
// changed, so a token rotation with no name is valid — requiring a name there
// would force every caller to resend fields it is not touching.
func validateConnectionInput(in ConnectionInput, create bool) error {
	if create {
		if strings.TrimSpace(in.Name) == "" {
			return fmt.Errorf("%w: name is required", ErrInvalidConnection)
		}
		if in.Type == "" {
			return fmt.Errorf("%w: provider type is required", ErrInvalidConnection)
		}
		if in.Token.IsZero() {
			return fmt.Errorf("%w: token is required", ErrInvalidConnection)
		}
	} else if strings.TrimSpace(in.Name) == "" && in.Type == "" && in.Host == "" &&
		in.Token.IsZero() && in.WriteEnabled == nil {
		return fmt.Errorf("%w: nothing to update", ErrInvalidConnection)
	}

	if in.Type != "" && !in.Type.Valid() {
		return fmt.Errorf("%w: unknown provider %q", ErrInvalidConnection, in.Type)
	}
	if in.Host != "" && !validHost(in.Host) {
		return fmt.Errorf("%w: host must be a hostname or URL", ErrInvalidConnection)
	}

	return nil
}

// duplicateConnectionNameAsInvalid turns the schema's UNIQUE(org_id, name) violation into
// a validation error. Left raw it reached the handler's default branch and became a 500,
// telling the user the server had broken rather than that the name was taken.
func duplicateConnectionNameAsInvalid(err error, name string) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed") {
		return fmt.Errorf("%w: a connection named %q already exists", ErrInvalidConnection, name)
	}

	return err
}

// normaliseHost reduces a host to a probeable form, so "https://github.com/" and
// "github.com" are one connection rather than two.
//
// An explicit http:// scheme is KEPT. https is the default the health checker assumes, so
// stripping "https://" loses nothing — but stripping "http://" made a plain-HTTP git host
// impossible to store at all: the checker forced https://, so an internal server (Gitea or
// GitLab on a LAN, a loopback server in development) could only ever read "unreachable".
func normaliseHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		if u.Scheme == "http" {
			return "http://" + u.Host
		}

		return u.Host
	}

	return strings.TrimSuffix(raw, "/")
}

// validHost reports whether a host is usable: non-empty, carrying no path and no spaces.
// The "//" that a scheme introduces is not a path, so it is removed before the path check.
func validHost(raw string) bool {
	host := normaliseHost(raw)
	if host == "" {
		return false
	}

	remainder := strings.TrimPrefix(strings.TrimPrefix(host, "http://"), "https://")

	return remainder != "" && !strings.ContainsAny(remainder, " /")
}
