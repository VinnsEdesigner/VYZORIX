package provisioning

import (
	"context"
	"time"
)

// ActorKind is who caused a provisioning action.
//
// The distinction is load-bearing: a sync that a scheduler started and one a person
// pressed must not read the same, because the first is unattended and the second
// needs a name against it. A service-account token is a third case: the action is
// machine-initiated but attributable to a stored credential rather than to a person.
type ActorKind string

const (
	// ActorUser is a signed-in person acting through the console.
	ActorUser ActorKind = "user"
	// ActorServiceAccount is an API token acting on behalf of an organisation.
	ActorServiceAccount ActorKind = "service_account"
	// ActorScheduler is the interval worker: nobody pressed anything.
	ActorScheduler ActorKind = "scheduler"
	// ActorSystem is the server itself at boot (interrupted-job cleanup, boot seed).
	ActorSystem ActorKind = "system"
	// ActorProvider is a git provider's push webhook. It is distinct from the scheduler:
	// nobody pressed anything in either case, but a provider push is triggered by an
	// external event, and an operator reading the audit trail needs to tell "the interval
	// fired" from "someone pushed".
	ActorProvider ActorKind = "provider"
)

// Valid reports whether k is a known actor kind.
func (k ActorKind) Valid() bool {
	switch k {
	case ActorUser, ActorServiceAccount, ActorScheduler, ActorSystem, ActorProvider:
		return true
	}

	return false
}

// Actor is who performed an action, and which credential proved it.
//
// It is a value, not a pointer, so every event carries a fully-formed identity: an
// empty Actor would mean "unknown", which the table forbids.
type Actor struct {
	Kind ActorKind
	// ID is the user id, service-account token id, or a fixed name for the
	// scheduler/system. It is the stable handle to join on.
	ID string
	// Label is the human-readable name: an email, a token name, or "scheduler".
	// Stored alongside the id because a deleted user leaves an id that resolves to
	// nothing, and an audit row must still read sensibly.
	Label string
}

// SystemActor is the server acting on its own behalf at boot.
func SystemActor() Actor {
	return Actor{Kind: ActorSystem, ID: "server", Label: "server"}
}

// SchedulerActor is the interval worker.
func SchedulerActor() Actor {
	return Actor{Kind: ActorScheduler, ID: "scheduler", Label: "scheduler"}
}

// EventType is one auditable provisioning action.
type EventType string

const (
	// EventConnectionCreated, EventConnectionUpdated, EventConnectionDeleted and
	// EventConnectionChecked cover the credential lifecycle. The token itself is
	// never recorded, only the fact that one was supplied.
	EventConnectionCreated EventType = "connection.created"
	EventConnectionUpdated EventType = "connection.updated"
	EventConnectionDeleted EventType = "connection.deleted"
	EventConnectionChecked EventType = "connection.checked"

	EventRepositoryCreated EventType = "repository.created"
	EventRepositoryUpdated EventType = "repository.updated"
	EventRepositoryDeleted EventType = "repository.deleted"

	// EventSyncTriggered records who asked for a sync, which is the whole point of
	// the actor column: manual, scheduled, or token-driven.
	EventSyncTriggered EventType = "sync.triggered"
	// EventSyncApplied records the outcome of a real (non-dry) sync, attributed to
	// the same actor that triggered it.
	EventSyncApplied EventType = "sync.applied"
	// EventDocumentPushed records a manifest edited in the console and committed back to
	// its repository. It is the only event that writes to a repository, so it carries the
	// branch it landed on.
	EventDocumentPushed EventType = "document.pushed"
)

// ProvisioningEvent is one audited action.
//
// It is a dedicated table rather than a reuse of audit_logs because the questions
// asked of it are different: audit_logs answers "what did this user do", while this
// answers "who caused this change to this repository, and was it a person or a
// machine". It also carries org_id, which audit_logs does not.
type ProvisioningEvent struct {
	CreatedAt    time.Time
	Actor        Actor
	ID           string
	OrgID        string
	ResourceType string
	ResourceID   string
	Type         EventType
	// Detail is a short human-readable summary. It must never contain a secret.
	Detail string
	// Result is "success" or "error".
	Result string
}

// EventRepository persists the provisioning audit trail.
type EventRepository interface {
	// Record appends one event.
	Record(ctx context.Context, event *ProvisioningEvent) error
	// ListForOrg returns an organisation's events, newest first.
	ListForOrg(ctx context.Context, orgID string, limit int) ([]*ProvisioningEvent, error)
	// ListForResource returns events about one resource, newest first.
	ListForResource(ctx context.Context, orgID, resourceType, resourceID string, limit int) ([]*ProvisioningEvent, error)
}

// AuditEscalator raises an alert when the audit trail itself is broken.
//
// A failed audit write is a silent integrity hole: the change happened and nothing
// recorded who made it. Logging it is not enough, because nobody reads a log until
// after the incident. The escalator turns it into an out-of-band alert, so the
// operator learns while it is still happening.
//
// It is deliberately not an alert.Rule: a rule is evaluated on a schedule against a
// metric, and this must fire on the event, for the organisation that just failed,
// with no rule to configure first.
type AuditEscalator interface {
	// EscalateAuditFailure reports that an audit write failed for an organisation.
	EscalateAuditFailure(ctx context.Context, orgID string, cause error) error
}
