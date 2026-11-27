// Package provisioning is GitOps for device configs: a repository is pulled on
// an interval, its manifests are applied through the deploy service, and each
// applied config is owned by exactly one repository so drift is explainable.
package provisioning

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrConnectionNotFound is returned when no connection matches.
	ErrConnectionNotFound = errors.New("provisioning connection not found")
	// ErrRepositoryNotFound is returned when no repository matches.
	ErrRepositoryNotFound = errors.New("provisioning repository not found")
	// ErrResourceNotFound is returned when no resource matches.
	ErrResourceNotFound = errors.New("provisioning resource not found")
	// ErrJobNotFound is returned when no job matches.
	ErrJobNotFound = errors.New("provisioning job not found")
	// ErrConnectionInUse is returned when deleting a connection a repository still
	// references. Removing it would leave the repository unable to reach its remote.
	ErrConnectionInUse = errors.New("connection is used by a repository")
	// ErrDeviceOwned is returned when a repository tries to apply to a device
	// another repository already owns. Two owners would make drift unexplainable:
	// nobody could say which repository the device should match.
	ErrDeviceOwned = errors.New("device is owned by another repository")
	// ErrSyncInProgress is returned when a manual sync is requested while one runs.
	ErrSyncInProgress = errors.New("a sync is already running for this repository")
	// ErrUnknownActor is returned when an event would be recorded without an actor.
	// An unattributed provisioning change is what the event table exists to prevent.
	ErrUnknownActor = errors.New("provisioning event has no valid actor")
)

// DeviceOwnedError reports which repository already owns a device, so the
// conflict message can name it instead of leaving the user to guess. It matches
// ErrDeviceOwned under errors.Is.
type DeviceOwnedError struct {
	DeviceID string
	Owner    string
}

// Error implements error.
func (e *DeviceOwnedError) Error() string {
	if e.Owner == "" {
		return "device " + e.DeviceID + " is owned by another repository"
	}

	return "device " + e.DeviceID + " is owned by repository " + e.Owner
}

// Is lets errors.Is match the sentinel.
func (e *DeviceOwnedError) Is(target error) bool { return target == ErrDeviceOwned }

// ConnectionType is the git provider a connection speaks.
type ConnectionType string

const (
	ConnectionGitHub  ConnectionType = "github"
	ConnectionGitLab  ConnectionType = "gitlab"
	ConnectionGeneric ConnectionType = "generic"
)

// Valid reports whether t is a known provider.
func (t ConnectionType) Valid() bool {
	switch t {
	case ConnectionGitHub, ConnectionGitLab, ConnectionGeneric:
		return true
	}

	return false
}

// ConnectionStatus is the result of the last health check.
type ConnectionStatus string

const (
	// ConnectionPending means the credential has not been checked yet.
	ConnectionPending ConnectionStatus = "pending"
	// ConnectionReady means the last check authenticated.
	ConnectionReady ConnectionStatus = "ready"
	// ConnectionError means the last check failed.
	ConnectionError ConnectionStatus = "error"
)

// Connection is a stored git credential, reusable by an organisation's
// repositories. The token is write-only: it is encrypted at rest and no read
// returns it.
type Connection struct {
	CheckedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
	ID        string
	OrgID     string
	Name      string
	Type      ConnectionType
	Host      string
	LastError string
	Status    ConnectionStatus
	// WriteEnabled declares that the stored token has push scope. Pulling only needs
	// read access, so this is off by default: a read-only token is the safe default and
	// the common case, and a push attempted with one fails at the provider with a
	// confusing 403 rather than a clear refusal here.
	WriteEnabled bool
}

// Repository is one branch of one remote, synced on an interval.
type Repository struct {
	UpdatedAt    time.Time
	CreatedAt    time.Time
	LastSyncAt   *time.Time
	Description  string
	URL          string
	ID           string
	OrgID        string
	ConnectionID string
	Name         string
	Title        string
	SyncTarget   SyncTarget
	Type         ConnectionType
	SyncState    SyncState
	Branch       string
	Path         string
	LastError    string
	// Include and Exclude are glob patterns matched against a resource path relative to
	// Path. Exclude wins over Include, and an empty Include means everything under Path.
	// They are the repository's declared scope, which is why a file that drops out of it
	// becomes "ignored" rather than "missing".
	Exclude         []string
	Include         []string
	IntervalSeconds int
	Enabled         bool
}

// SyncTarget is what a repository synchronizes to.
type SyncTarget string

const (
	// SyncTargetDevice provisions individual devices, one manifest per device.
	SyncTargetDevice SyncTarget = "device"
)

// Valid reports whether t is a known target.
func (t SyncTarget) Valid() bool {
	switch t {
	case SyncTargetDevice:
		return true
	}

	return false
}

// SyncState is the outcome of the last sync.
type SyncState string

const (
	SyncPending  SyncState = "pending"
	SyncSuccess  SyncState = "success"
	SyncWarning  SyncState = "warning"
	SyncWorking  SyncState = "working"
	SyncError    SyncState = "error"
	SyncDisabled SyncState = "disabled"
)

// ResourceKind is what a provisioned file declares, taken from the directory it lives in.
//
// It lives in the domain rather than the application package because the resource row
// carries it: the storage layer must not import application code, and the uniqueness rule
// depends on it.
type ResourceKind string

const (
	// KindDevice is a per-device deploy manifest under devices/<device-id>/.
	KindDevice ResourceKind = "device"
	// KindAlerts is an organisation's alert rules, orgs/<org>/alerts.toml.
	KindAlerts ResourceKind = "alerts"
	// KindContactPoints is an organisation's notification targets.
	KindContactPoints ResourceKind = "contact_points"
	// KindAnnotations is an organisation's annotations.
	KindAnnotations ResourceKind = "annotations"
	// KindGroups is an organisation's device groups and their membership.
	KindGroups ResourceKind = "groups"
)

// ResourceState is the drift status of one applied manifest.
type ResourceState string

// ResourceStats is the per-state count of a repository's resources.
//
// It is derived on read rather than stored: the counts are a projection of the
// resource rows, and a stored copy would drift from them the moment a state
// changed without a sync. The contract lists stats[] under a repository's status,
// and a list view needs them without loading every resource.
type ResourceStats struct {
	Applied int `json:"applied"`
	Drifted int `json:"drifted"`
	Missing int `json:"missing"`
	Error   int `json:"error"`
	Ignored int `json:"ignored"`
	// Pending counts resources whose device has not confirmed the config it holds.
	Pending int `json:"pending"`
	// Total is the sum, so a client does not have to add the six up.
	Total int `json:"total"`
}

const (
	// ResourceApplied means the device runs the config this repository applied.
	ResourceApplied ResourceState = "applied"
	// ResourceDrifted means the device's config changed after this repository
	// applied it.
	ResourceDrifted ResourceState = "drifted"
	// ResourceMissing means the manifest was deleted from the repository.
	ResourceMissing ResourceState = "missing"
	// ResourceError means the last apply failed.
	ResourceError ResourceState = "error"
	// ResourceIgnored means the file is now outside the repository's include/exclude
	// scope. It is deliberately not "missing": missing is "the file was deleted", and
	// reverting that is a deliberate act. Ignored is "this repository no longer claims
	// it", so the resource is left alone and only its state changes.
	ResourceIgnored ResourceState = "ignored"
	// ResourcePending means the config was signed and stored, but the device has not
	// confirmed it is running it. A deploy writing a row is not proof of anything: the
	// device may not be registered, may be offline, or may not have fetched yet. Reporting
	// "applied" until the device says so would claim a fact the server cannot see.
	ResourcePending ResourceState = "pending"
)

// Resource is one manifest, bound to one device.
//
// Fields are ordered by alignment so the struct packs without padding: the two
// time.Time values and the pointers first, then the strings, then the scalars.
type Resource struct {
	AppliedAt time.Time
	// ReleasedAt is set when a repository gives up ownership without reverting the
	// device. A released device can be adopted by another repository; an owned one
	// cannot.
	ReleasedAt    *time.Time
	LastAppliedAt *time.Time
	ID            string
	RepoID        string
	// Kind is what the resource declares: a device manifest or an org file. It selects the
	// uniqueness rule, because a device resource is owned per device and an org resource per
	// (kind, path).
	Kind ResourceKind
	// DeviceID is set for a device resource and empty for an org resource.
	DeviceID      string
	Path          string
	ContentHash   string
	LastError     string
	State         ResourceState
	ConfigVersion int
}

// JobResultOutcome is one manifest's fate within a sync job.
type JobResultOutcome string

const (
	ResultApplied   JobResultOutcome = "applied"
	ResultUpdated   JobResultOutcome = "updated"
	ResultUnchanged JobResultOutcome = "unchanged"
	ResultMissing   JobResultOutcome = "missing"
	ResultIgnored   JobResultOutcome = "ignored"
	// ResultSkipped marks an item a single-resource sync deliberately left alone. It is
	// distinct from unchanged: the item was not compared-and-equal, it was out of scope
	// for this run.
	ResultSkipped JobResultOutcome = "skipped"
	ResultFailed  JobResultOutcome = "failed"
)

// JobResult is one manifest's outcome in one job. Job-level counters say how
// many failed; these say which, and why.
type JobResult struct {
	ID         string
	JobID      string
	ResourceID string
	Path       string
	Outcome    JobResultOutcome
	Message    string
}

// Job is one sync run.
type Job struct {
	FinishedAt *time.Time
	StartedAt  time.Time
	Actor      Actor
	ID         string
	RepoID     string
	State      SyncState
	Error      string
	Applied    int
	Skipped    int
	Failed     int
	// DurationMs is how long the run took, recorded when it finished so the
	// health-over-time view can aggregate without recomputing it per row.
	DurationMs int64
	DryRun     bool
}

// HealthPoint is one completed sync run in a repository's health timeline.
//
// It is a projection of a job, not the job itself: a trend only needs when a run
// happened, how it ended, and how long it took, so the timeline stays small even
// for a repository with a long history.
type HealthPoint struct {
	FinishedAt time.Time `json:"finished_at"`
	State      SyncState `json:"state"`
	DurationMs int64     `json:"duration_ms"`
	Failed     int       `json:"failed"`
}

// RepositoryHealth is a repository's sync health over time.
//
// The three figures answer different questions and are not derivable from each
// other: AverageDurationMs says how slow syncing is now, FailureStreak says whether
// the repository is currently broken, and Timeline shows whether it has been
// flapping.
type RepositoryHealth struct {
	// Timeline is the recent runs, oldest first, so a client draws left to right.
	Timeline []HealthPoint `json:"timeline"`
	// AverageDurationMs is the mean duration of the recent completed runs. Zero when
	// there is no history, which a client renders as "no data" rather than "instant".
	AverageDurationMs int64 `json:"average_duration_ms"`
	// LastDurationMs is the most recent completed run's duration.
	LastDurationMs int64 `json:"last_duration_ms"`
	// FailureStreak is how many consecutive most-recent runs failed. Zero means the
	// latest run succeeded.
	FailureStreak int `json:"failure_streak"`
	// Successes and Failures count the runs in the window the timeline covers.
	Successes int `json:"successes"`
	Failures  int `json:"failures"`
}

// ResourceRepository persists the manifests a repository owns.
type ResourceRepository interface {
	// Upsert records an applied manifest, taking ownership of the device.
	// Returns ErrDeviceOwned when another repository owns it.
	Upsert(ctx context.Context, r *Resource) error
	GetByDevice(ctx context.Context, deviceID string) (*Resource, error)
	// GetByID returns one resource scoped to its repository, so a resource id from
	// another repository cannot be read through the wrong route.
	GetByID(ctx context.Context, repoID, id string) (*Resource, error)
	ListByRepo(ctx context.Context, repoID string) ([]*Resource, error)
	// StatsByRepo returns the per-state counts of a repository's owned resources.
	StatsByRepo(ctx context.Context, repoID string) (*ResourceStats, error)
	SetState(ctx context.Context, id string, state ResourceState, lastError string) error
	// UpdatePath re-points a resource at a new manifest path. A rename inside a repository
	// keeps the same device and content, but the row records the path, so the row must move
	// with the file.
	UpdatePath(ctx context.Context, id, path string) error
}

// JobRepository persists sync runs and their per-manifest outcomes.
type JobRepository interface {
	// StartJob records a job in the working state and returns its id.
	StartJob(ctx context.Context, job *Job) error
	// FinishJob writes the aggregate counters and the terminal state.
	FinishJob(ctx context.Context, job *Job) error
	GetJob(ctx context.Context, repoID, id string) (*Job, error)
	ListJobs(ctx context.Context, repoID string, limit int) ([]*Job, error)
	// SaveResult appends one manifest's outcome to a job.
	SaveResult(ctx context.Context, result *JobResult) error
	ListResults(ctx context.Context, jobID string) ([]*JobResult, error)
	// Health returns the repository's sync health over its most recent completed runs.
	Health(ctx context.Context, repoID string, limit int) (*RepositoryHealth, error)
	// FailInterrupted marks jobs left working by a restart as interrupted. A job
	// cannot survive the process that was running it, so a stale working row is a
	// lie the next boot must correct.
	FailInterrupted(ctx context.Context, olderThan time.Time) (int, error)
}
