package provisioning

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploy"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/deployconfig"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/google/uuid"
)

// ErrSyncDisabled is returned when a sync is requested for a repository whose
// sync is switched off.
var ErrSyncDisabled = errors.New("repository sync is disabled")

// Action is what a sync decided to do with one manifest. It is the diff's output
// and the preview's vocabulary, so the dry run and the real run speak the same
// words.
type Action string

const (
	// ActionCreate applies a manifest for a device this repository does not own yet.
	ActionCreate Action = "create"
	// ActionUpdate applies a changed manifest for a device it already owns.
	ActionUpdate Action = "update"
	// ActionUnchanged leaves a manifest whose hash and version already match.
	ActionUnchanged Action = "unchanged"
	// ActionMissing flags a resource whose file disappeared. It is never applied.
	ActionMissing Action = "missing"
	// ActionConflict refuses a device another repository owns.
	ActionConflict Action = "conflict"
	// ActionError marks a manifest that cannot be applied (malformed, or a device
	// outside the organisation).
	ActionError Action = "error"
	// ActionIgnored marks a resource whose file is now outside the repository's
	// include/exclude scope. It is never applied and never reverted: only the state
	// changes, so the difference from "missing" stays visible.
	ActionIgnored Action = "ignored"
)

// repoScope is the one place a repository's patterns become a Scope, so the two call sites
// that need it (the device plan and org files) cannot drift when a pattern list is added.
func repoScope(repo *domain.Repository) Scope {
	return NewScope(repo.Include, repo.Exclude)
}

// PlanItem is one manifest and the action the diff chose for it.
type PlanItem struct {
	Manifest *Manifest
	Resource *domain.Resource
	Action   Action
	Message  string
	// Drifted reports that the device's config was changed outside this repository since
	// it was last applied. It is a field rather than something a caller infers from
	// Message, so the apply path can record the drifted state without parsing prose.
	Drifted bool
	// Skipped marks an item a single-resource sync deliberately left out. It is applied
	// by nothing and recorded as skipped, so the job shows what the full plan knew without
	// implying the other resources were unchanged.
	Skipped bool
}

// Plan is the whole diff: what a sync would do, computed without writing
// anything.
type Plan struct {
	Items []*PlanItem
}

// Plan computes the diff between the discovered manifests and the resources this
// repository already owns.
//
// It is pure: it reads the resource store and touches nothing. The dry run is
// therefore the same computation as the real run's first half, not a parallel
// implementation that can disagree with it.
func (s *SyncService) Plan(ctx context.Context, repo *domain.Repository, manifests []*Manifest) (*Plan, error) {
	owned, err := s.resources.ListByRepo(ctx, repo.ID)
	if err != nil {
		return nil, err
	}

	byPath := make(map[string]*domain.Resource, len(owned))
	for _, r := range owned {
		// An org resource has no device manifest and is not part of the device plan. Its
		// lifecycle is driven by applyOrgFiles, so including it here would report every
		// applied org file as "missing" on the next sync.
		if r.Kind != "" && r.Kind != domain.KindDevice {
			continue
		}

		byPath[r.Path] = r
	}

	// The repository's declared scope filters discovery, so an excluded file is never
	// planned. It is a read-side filter over the discovered set, not a post-hoc skip: a
	// file the operator excluded must not appear as a resource at all.
	scope := repoScope(repo)

	seenPath := make(map[string]bool, len(manifests))
	// A resource is matched by identity, not only by path: a manifest renamed within the
	// repository (vyzorix.toml -> vyzorix.v) is the same device with a new path. Keying
	// only on path reported it twice — once as an update and once as missing.
	matched := make(map[string]bool, len(manifests))
	plan := &Plan{}

	for _, m := range manifests {
		if !scope.InScope(m.Path) {
			continue
		}

		seenPath[m.Path] = true
		item := s.planOne(ctx, repo, m, byPath[m.Path])
		if item.Resource != nil {
			matched[item.Resource.ID] = true
		}
		plan.Items = append(plan.Items, item)
	}

	// A resource whose file is gone is missing, not deleted. Reverting is an
	// explicit action because a force-push that silently wiped a device's config
	// would be a destructive surprise.
	for _, r := range owned {
		// An org resource is not a device manifest, so it is never "missing" from the device
		// plan. Its own lifecycle handles a deleted org file.
		if r.Kind != "" && r.Kind != domain.KindDevice {
			continue
		}

		if seenPath[r.Path] || matched[r.ID] {
			continue
		}

		// A file that fell out of scope is not a deleted file. Missing invites a revert;
		// ignored says this repository no longer claims it, so the device is left exactly
		// as it is and only the resource's state changes.
		if !scope.InScope(r.Path) {
			plan.Items = append(plan.Items, &PlanItem{
				Resource: r,
				Action:   ActionIgnored,
				Message:  "the file is outside this repository's include/exclude scope",
			})

			continue
		}

		plan.Items = append(plan.Items, &PlanItem{
			Resource: r,
			Action:   ActionMissing,
			Message:  "the manifest is no longer in the repository",
		})
	}

	return plan, nil
}

// planOne decides the action for a single discovered manifest.
func (s *SyncService) planOne(ctx context.Context, repo *domain.Repository, m *Manifest, existing *domain.Resource) *PlanItem {
	item := &PlanItem{Manifest: m, Resource: existing}

	if m.Err != nil {
		item.Action = ActionError
		item.Message = m.Err.Error()

		return item
	}

	// One owner per device is a schema constraint, but checking here turns the
	// violation into a named conflict before the write rather than a 500 from a
	// unique-index failure during it.
	owner, err := s.resources.GetByDevice(ctx, m.DeviceID)
	if err != nil && !errors.Is(err, domain.ErrResourceNotFound) {
		item.Action = ActionError
		item.Message = err.Error()

		return item
	}
	if owner != nil && owner.RepoID != repo.ID {
		item.Action = ActionConflict
		item.Message = (&domain.DeviceOwnedError{DeviceID: m.DeviceID, Owner: owner.RepoID}).Error()

		return item
	}

	// The device is owned by this repository at a different path: the manifest was renamed
	// (for example vyzorix.toml -> vyzorix.v). That is an update of the existing row, not a
	// create — a create would hit the one-owner index and report the device as owned by
	// another repository, which is both wrong and unactionable.
	if existing == nil && owner != nil && owner.RepoID == repo.ID {
		existing = owner
		item.Resource = owner
	}

	if existing == nil {
		item.Action = ActionCreate

		return item
	}

	// Drift: the device was changed by something else after this repository applied
	// it, so its current version no longer matches the one this repository wrote. It
	// is re-applied even when the file is byte-identical, because the file is not what
	// the device is running.
	if s.drifted(ctx, repo, existing) {
		item.Action = ActionUpdate
		item.Drifted = true
		item.Message = "drifted: the device's config was changed after this repository applied it"

		return item
	}

	if existing.ContentHash == m.ContentHash {
		item.Action = ActionUnchanged

		return item
	}

	item.Action = ActionUpdate

	return item
}

// drifted reports whether the device's current config version differs from the one
// this resource recorded. A deployer that cannot report a version (no config, or
// not configured) is treated as not drifted, so an unknown never fabricates a
// state.
func (s *SyncService) drifted(ctx context.Context, repo *domain.Repository, existing *domain.Resource) bool {
	if s.deployer == nil || existing.ConfigVersion == 0 {
		return false
	}

	current, ok, err := s.deployer.CurrentVersion(ctx, repo.OrgID, existing.DeviceID)
	if err != nil || !ok {
		return false
	}

	return current != existing.ConfigVersion
}

// SyncResult is what a completed sync reports.
type SyncResult struct {
	Job *domain.Job
}

// Sync runs one sync for a repository.
//
// A dry run computes the plan and records it in the job without touching a
// device. A real run applies each create/update through the deploy service, which
// already compiles, authorises, signs and persists, and is covered by its own
// roundtrip and refusal tests. Provisioning only decides *which* manifests to
// apply; reimplementing the deploy pipeline here would be a second, untested copy
// of the thing that must not diverge.
func (s *SyncService) Sync(ctx context.Context, repo *domain.Repository, opts SyncOptions) (*SyncResult, error) {
	return s.runSync(ctx, repo, opts, nil)
}

// ErrNotADeviceResource is returned when a single-resource sync is asked to sync a resource
// that is not a device manifest. The endpoint applies one device config; an org file has no
// device to converge, so accepting it produced a job that reported nothing at all.
var ErrNotADeviceResource = errors.New("this resource is not a device manifest, so it cannot be synced on its own")

// SyncResource applies one resource, after the same fetch and plan a full sync uses.
//
// The pipeline is shared rather than copied: a second code path that re-implemented the
// fetch, discovery, and apply would drift from the one the scheduled sync runs, and the
// whole point of a single-device sync is that it does exactly what a full sync would do
// for that one file. Only the apply set is narrowed, and only the resource's own state is
// stamped — the repository's sync state is left alone because the repository was not
// synced.
func (s *SyncService) SyncResource(ctx context.Context, repo *domain.Repository, resourceID string, opts SyncOptions) (*SyncResult, error) {
	// The resource must belong to this repository. A resource id from another repository
	// is a not-found, not a cross-repository apply. Checked before the lock so a bad id
	// costs no lock acquisition.
	resource, err := s.resources.GetByID(ctx, repo.ID, resourceID)
	if err != nil {
		return nil, err
	}

	if resource.Kind != "" && resource.Kind != domain.KindDevice {
		return nil, ErrNotADeviceResource
	}

	return s.runSync(ctx, repo, opts, func(plan *Plan) *Plan {
		return narrowPlan(plan, resourceID)
	})
}

// narrowPlan keeps only the item for resourceID and marks the rest skipped.
//
// The skipped items are kept rather than dropped so the job shows what the full plan knew
// and what this run did. A job that applied one file must not imply the others were
// unchanged.
func narrowPlan(plan *Plan, resourceID string) *Plan {
	narrowed := &Plan{Items: make([]*PlanItem, 0, len(plan.Items))}

	for _, item := range plan.Items {
		if item.Resource != nil && item.Resource.ID == resourceID {
			narrowed.Items = append(narrowed.Items, item)

			continue
		}

		narrowed.Items = append(narrowed.Items, &PlanItem{
			Manifest: item.Manifest,
			Resource: item.Resource,
			// The action is left empty: the item is not "unchanged", it was simply not part
			// of this run. Skipped is the field that says so, and the apply loop checks it
			// before the action.
			Action:  "",
			Message: "not part of this single-device sync",
			Skipped: true,
		})
	}

	return narrowed
}

// runSync is the shared sequence. narrow is nil for a full sync and filters the plan for a
// single-resource sync.
func (s *SyncService) runSync(ctx context.Context, repo *domain.Repository, opts SyncOptions, narrow func(*Plan) *Plan) (*SyncResult, error) {
	if !repo.Enabled {
		return nil, ErrSyncDisabled
	}

	// One sync per repository at a time. A second concurrent run would interleave
	// its writes with the first and neither job's counters would mean anything.
	if !s.locks.acquire(repo.ID) {
		return nil, domain.ErrSyncInProgress
	}
	defer s.locks.release(repo.ID)

	job := &domain.Job{
		ID:        uuid.New().String(),
		RepoID:    repo.ID,
		State:     domain.SyncWorking,
		DryRun:    opts.DryRun,
		StartedAt: s.now(),
		Actor:     resolveActor(opts.Actor),
	}

	if err := s.jobs.StartJob(ctx, job); err != nil {
		return nil, err
	}

	checkout, manifests, err := s.fetchAndDiscover(ctx, repo)
	if err != nil {
		return s.failJob(ctx, repo, job, err)
	}
	defer checkout.Cleanup()

	plan, err := s.Plan(ctx, repo, manifests)
	if err != nil {
		return s.failJob(ctx, repo, job, err)
	}

	if narrow != nil {
		plan = narrow(plan)
	}

	if opts.DryRun {
		t := s.recordPlanResults(ctx, job, plan, opts)

		// A preview must report the org files too. A dry run that showed only device
		// manifests would disagree with what the real run does, which is exactly the
		// divergence the preview exists to prevent.
		if narrow == nil {
			s.previewOrgFileResults(ctx, repo, job, checkout.Dir, t)
		}

		return s.finishJob(ctx, repo, job, t, narrow != nil)
	}

	t := s.apply(ctx, repo, job, plan, opts)

	// Org files are applied on a full sync only. A single-resource sync names one device
	// resource, so it must not quietly write an organisation's alerts as a side effect.
	if narrow == nil {
		s.applyOrgFiles(ctx, repo, job, checkout.Dir, t)
	}

	return s.finishJob(ctx, repo, job, t, narrow != nil)
}

// Discovery is what a repository contains, classified against what it currently
// owns. It is the read the wizard's "which devices will this repository own" step
// needs: every manifest with the action a sync would take, plus the device
// directories the repository does not manage.
type Discovery struct {
	Items     []*PlanItem
	Unmanaged []DeviceDir
	// OrgFiles are the organisation files a sync would apply. They are listed separately
	// from Items because they carry no device and no deploy plan, but they must be reported:
	// a preview that hid them would disagree with what a real sync does.
	OrgFiles []OrgResourceSummary
}

// OrgResourceSummary is one org file as a preview reports it.
type OrgResourceSummary struct {
	Path   string       `json:"path"`
	Org    string       `json:"org"`
	Kind   ResourceKind `json:"kind"`
	Action Action       `json:"action"`
	// Message explains a refusal, such as an unknown organisation.
	Message string `json:"message,omitempty"`
}

// Discover fetches the repository and reports what a sync would do, without doing
// any of it.
//
// It is the same Plan the dry run uses, so the wizard's preview and the real sync
// cannot disagree. The checkout is removed before returning; nothing is written to
// the resource store, the job table, or a device.
func (s *SyncService) Discover(ctx context.Context, repo *domain.Repository) (*Discovery, error) {
	// Discovery answers "what would a sync do", and a disabled repository would do
	// nothing. Reporting a full plan for it would show the wizard an apply list that Sync
	// then refuses with ErrSyncDisabled.
	if !repo.Enabled {
		return nil, ErrSyncDisabled
	}

	token, err := s.connections.ConnectionToken(ctx, repo.OrgID, repo.ConnectionID)
	if err != nil {
		return nil, fmt.Errorf("%w: connection unavailable: %v", ErrFetchFailed, err)
	}

	checkout, err := s.fetcher.Fetch(ctx, repo, token)
	if err != nil {
		return nil, err
	}
	defer checkout.Cleanup()

	survey, err := SurveyRepository(checkout.Dir, repo.Path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetchFailed, err)
	}

	plan, err := s.Plan(ctx, repo, survey.Manifests)
	if err != nil {
		return nil, err
	}

	orgFiles, err := s.previewOrgFiles(ctx, repo, checkout.Dir)
	if err != nil {
		return nil, err
	}

	return &Discovery{Items: plan.Items, Unmanaged: survey.Unmanaged, OrgFiles: orgFiles}, nil
}

// previewOrgFiles classifies the org files a sync would write, without writing them.
//
// It is the read half of applyOrgFiles, so the wizard's preview and the real run cannot
// disagree about which org files a repository contains or whether one would be refused.
func (s *SyncService) previewOrgFiles(ctx context.Context, repo *domain.Repository, dir string) ([]OrgResourceSummary, error) {
	orgFiles, err := DiscoverOrgFiles(dir, repo.Path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFetchFailed, err)
	}

	scope := repoScope(repo)
	out := make([]OrgResourceSummary, 0, len(orgFiles))

	for _, res := range orgFiles {
		if !scope.InScope(res.Path) {
			continue
		}

		summary := OrgResourceSummary{Path: res.Path, Org: res.Org, Kind: res.Kind, Action: ActionCreate}

		switch {
		case res.Err != nil:
			summary.Action = ActionError
			summary.Message = res.Err.Error()
		case !s.hasOrgApplier(res.Kind):
			summary.Action = ActionError
			summary.Message = fmt.Sprintf("no applier is configured for %s files", res.Kind)
		default:
			if msg, ok := s.orgNameMismatch(ctx, repo, res); !ok {
				summary.Action = ActionError
				summary.Message = msg
			}
		}

		out = append(out, summary)
	}

	return out, nil
}

// hasOrgApplier reports whether a kind has an applier wired.
func (s *SyncService) hasOrgApplier(kind ResourceKind) bool {
	_, ok := s.orgAppliers[kind]

	return ok
}

// SyncOptions are the knobs a manual sync exposes.
type SyncOptions struct {
	// Actor is who asked for this sync. It is recorded on the job's event so a
	// scheduled run and a person's run are distinguishable afterwards.
	Actor domain.Actor
	// DryRun computes the plan and applies nothing.
	DryRun bool
	// RevertMissing asks the sync to re-apply a device's previous config when its
	// manifest disappears. Off by default: a force-push must not silently wipe a
	// device.
	RevertMissing bool
}

// fetchAndDiscover does the two read-only halves of a sync.
func (s *SyncService) fetchAndDiscover(ctx context.Context, repo *domain.Repository) (*Checkout, []*Manifest, error) {
	token, err := s.connections.ConnectionToken(ctx, repo.OrgID, repo.ConnectionID)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: connection unavailable: %v", ErrFetchFailed, err)
	}

	checkout, err := s.fetcher.Fetch(ctx, repo, token)
	if err != nil {
		return nil, nil, err
	}

	manifests, err := Discover(checkout.Dir, repo.Path)
	if err != nil {
		checkout.Cleanup()

		return nil, nil, fmt.Errorf("%w: %v", ErrFetchFailed, err)
	}

	return checkout, manifests, nil
}

// previewOrgFileResults records one job result per org file a real run would write.
func (s *SyncService) previewOrgFileResults(ctx context.Context, repo *domain.Repository, job *domain.Job, dir string, t *tally) {
	summaries, err := s.previewOrgFiles(ctx, repo, dir)
	if err != nil {
		s.log.Error("provisioning: could not preview org files", "repo", repo.ID, "err", err)

		return
	}

	for _, summary := range summaries {
		outcome := domain.ResultApplied
		msg := "would apply"

		if summary.Action == ActionError {
			outcome = domain.ResultFailed
			msg = summary.Message
		}

		t.record(outcome)
		s.saveOrgResult(ctx, job, summary.Path, outcome, msg)
	}
}

// applyOrgFiles writes every org file in the checkout and records one job result each.
//
// It runs after the device plan so a broken org file cannot stop a device from converging,
// and each file is its own outcome: a bad alerts.toml fails that file and leaves the rest
// of the repository alone, exactly as a bad device manifest does.
func (s *SyncService) applyOrgFiles(ctx context.Context, repo *domain.Repository, job *domain.Job, dir string, t *tally) {
	orgFiles, err := DiscoverOrgFiles(dir, repo.Path)
	if err != nil {
		s.log.Error("provisioning: could not discover org files", "repo", repo.ID, "err", err)

		return
	}

	scope := repoScope(repo)
	seen := make(map[string]bool, len(orgFiles))

	for _, res := range orgFiles {
		if !scope.InScope(res.Path) {
			continue
		}

		seen[res.Path] = true

		outcome, msg := s.applyOrgFile(ctx, repo, res)
		t.record(outcome)
		s.saveOrgResult(ctx, job, res.Path, outcome, msg)
	}

	s.reconcileMissingOrgFiles(ctx, repo, job, seen, t)
}

// reconcileMissingOrgFiles flags org rows whose file is no longer in the repository.
//
// Without this an org file deleted from the repo left its row reading "applied" forever, so
// the console claimed the alerts matched the repository when the file was gone. A device
// manifest already gets this treatment; the org half was missing.
//
// It flags only. Nothing is deleted and nothing reverts, which is the same rule a device
// manifest follows: a file disappearing is surfaced, not acted on.
func (s *SyncService) reconcileMissingOrgFiles(ctx context.Context, repo *domain.Repository, job *domain.Job, seen map[string]bool, t *tally) {
	owned, err := s.resources.ListByRepo(ctx, repo.ID)
	if err != nil {
		s.log.Error("provisioning: could not list resources to reconcile org files", "repo", repo.ID, "err", err)

		return
	}

	scope := repoScope(repo)

	for _, r := range owned {
		if r.Kind == "" || r.Kind == domain.KindDevice {
			continue
		}
		if seen[r.Path] {
			continue
		}
		// A file that fell out of scope is not a deleted file; it is simply no longer claimed.
		if !scope.InScope(r.Path) {
			continue
		}
		// An org row that failed to apply this run already carries its own error.
		if r.State == domain.ResourceError {
			continue
		}

		msg := "the file is no longer in the repository; the organisation keeps its current settings"

		if err := s.resources.SetState(ctx, r.ID, domain.ResourceMissing, msg); err != nil {
			s.log.Error("provisioning: could not mark an org resource missing", "resource", r.ID, "err", err)
			t.record(domain.ResultFailed)
			s.saveOrgResult(ctx, job, r.Path, domain.ResultFailed, "could not record the missing state: "+err.Error())

			continue
		}

		t.record(domain.ResultMissing)
		s.saveOrgResult(ctx, job, r.Path, domain.ResultMissing, msg)
	}
}

// applyOrgFile applies one org file and reports its outcome.
func (s *SyncService) applyOrgFile(ctx context.Context, repo *domain.Repository, res *OrgResource) (domain.JobResultOutcome, string) {
	if res.Err != nil {
		return domain.ResultFailed, res.Err.Error()
	}

	// The directory names the organisation, so it must be the repository's own. A file in
	// orgs/some-other-org/ writing to this repository's organisation would be a mistake
	// applied silently in the wrong direction.
	if msg, ok := s.orgNameMismatch(ctx, repo, res); !ok {
		return domain.ResultFailed, msg
	}

	applier, ok := s.orgAppliers[res.Kind]
	if !ok {
		return domain.ResultFailed, fmt.Sprintf("no applier is configured for %s files", res.Kind)
	}

	result, err := applier.Apply(ctx, repo.OrgID, res.Bytes)
	if err != nil {
		s.recordOrgResource(ctx, repo, res, 0, domain.ResourceError, err.Error())

		return domain.ResultFailed, err.Error()
	}

	// The org file is an owned resource, exactly as a device manifest is: it carries a state,
	// a content hash and an owning repository.
	//
	// Drift is recorded only when the file itself had not changed since it was last applied.
	// If the hash moved, the file is the change; if the hash is the same but the objects
	// differed, something outside provisioning edited them, which is drift.
	s.recordOrgResourceWithDrift(ctx, repo, res, result.Drifted > 0)

	return domain.ResultApplied, result.Detail
}

// recordOrgResourceWithDrift records an applied org file, flagging drift when the objects
// changed while the file did not.
func (s *SyncService) recordOrgResourceWithDrift(ctx context.Context, repo *domain.Repository, res *OrgResource, objectsDrifted bool) {
	state := domain.ResourceApplied

	if objectsDrifted {
		if existing, err := s.resources.GetByID(ctx, repo.ID, resourceIDForPath(res.Path)); err == nil {
			// A changed file is the file's own change, not drift. Only an unchanged file with
			// changed objects means someone edited outside git.
			if existing.ContentHash == res.ContentHash {
				state = domain.ResourceDrifted
			}
		}
	}

	s.recordOrgResource(ctx, repo, res, 0, state, "")
}

// recordOrgResource upserts the resource row for an org file.
//
// It reuses the device resource table with the org kind and no device, so an org resource
// gets the same state, hash, ownership and release lifecycle as a device manifest. A failure
// to record is logged rather than returned: the apply already happened.
func (s *SyncService) recordOrgResource(ctx context.Context, repo *domain.Repository, res *OrgResource, version int, state domain.ResourceState, lastError string) {
	existing, err := s.resources.GetByID(ctx, repo.ID, resourceIDForPath(res.Path))
	if err != nil && !errors.Is(err, domain.ErrResourceNotFound) {
		s.log.Error("provisioning: could not look up an org resource", "path", res.Path, "err", err)
	}

	row := &domain.Resource{
		ID:            resourceIDForPath(res.Path),
		RepoID:        repo.ID,
		Kind:          res.Kind,
		Path:          res.Path,
		ContentHash:   res.ContentHash,
		State:         state,
		LastError:     lastError,
		ConfigVersion: version,
		AppliedAt:     s.now(),
	}
	if existing != nil {
		row.ID = existing.ID
	}
	appliedAt := row.AppliedAt
	row.LastAppliedAt = &appliedAt

	if err := s.resources.Upsert(ctx, row); err != nil {
		s.log.Error("provisioning: could not record an org resource", "path", res.Path, "err", err)
	}
}

// resourceIDForPath derives a stable id for an org resource from its path, so a re-sync
// updates the same row instead of inserting a new one. A device resource gets its id from
// the device; an org file has only its path to key on.
func resourceIDForPath(path string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("org-resource:"+path)).String()
}

// orgNameMismatch reports whether the file's directory names a different organisation than
// the repository's. It returns ok=true when the names match, or when no resolver is wired.
func (s *SyncService) orgNameMismatch(ctx context.Context, repo *domain.Repository, res *OrgResource) (string, bool) {
	if s.orgNames == nil {
		return "", true
	}

	name, err := s.orgNames.OrgName(ctx, repo.OrgID)
	if err != nil {
		return fmt.Sprintf("could not resolve this repository's organisation: %v", err), false
	}

	if res.Org != name {
		return fmt.Sprintf("%s names organisation %q, but this repository belongs to %q",
			res.Path, res.Org, name), false
	}

	return "", true
}

// saveOrgResult records one org file's outcome. It has no resource id, because an org file
// is addressed by its path, which is what the job result carries.
func (s *SyncService) saveOrgResult(ctx context.Context, job *domain.Job, path string, outcome domain.JobResultOutcome, msg string) {
	res := &domain.JobResult{
		ID:      uuid.New().String(),
		JobID:   job.ID,
		Path:    path,
		Outcome: outcome,
		Message: msg,
	}

	if err := s.jobs.SaveResult(ctx, res); err != nil {
		s.log.Error("provisioning: could not record org file result", "job", job.ID, "path", path, "err", err)
	}
}

// tally counts the outcomes a run actually produced, so the job's counters
// describe what happened rather than what was planned. A planned apply that was
// refused must not be reported as applied.
type tally struct {
	applied int
	skipped int
	failed  int
	warned  int
}

// record counts one outcome.
func (t *tally) record(outcome domain.JobResultOutcome) {
	switch outcome {
	case domain.ResultApplied, domain.ResultUpdated:
		t.applied++
	case domain.ResultUnchanged, domain.ResultSkipped:
		t.skipped++
	case domain.ResultMissing:
		t.warned++
	case domain.ResultIgnored:
		// Ignored is counted with the skipped set: nothing was applied and nothing is
		// wrong, so it must not colour the run as a warning the way missing does.
		t.skipped++
	case domain.ResultFailed:
		t.failed++
	}
}

// apply performs every create and update in the plan.
//
// A per-resource failure is recorded and the loop continues: one malformed
// manifest must not stop the rest of the fleet from converging.
func (s *SyncService) apply(ctx context.Context, repo *domain.Repository, job *domain.Job, plan *Plan, opts SyncOptions) *tally {
	t := &tally{}

	for _, item := range plan.Items {
		// A single-resource sync leaves every other item alone. Recording it as skipped is
		// what keeps the job honest: the item was known and deliberately not acted on,
		// which is not the same as unchanged.
		if item.Skipped {
			t.record(domain.ResultSkipped)
			s.saveResult(ctx, job, item, domain.ResultSkipped, item.Message)

			continue
		}

		switch item.Action {
		case ActionCreate, ActionUpdate:
			s.applyOne(ctx, repo, job, item, t)
		case ActionMissing:
			s.markMissing(ctx, repo, job, item, opts.RevertMissing, t)
		case ActionConflict:
			// A conflict is a failure, not a skip: the contract says the second
			// repository's resource goes to error. The dry run reports the same outcome
			// (see previewOutcome), so the preview and the real run agree.
			t.record(domain.ResultFailed)
			s.saveResult(ctx, job, item, domain.ResultFailed, item.Message)
			s.markResourceError(ctx, item, item.Message)
		case ActionError:
			t.record(domain.ResultFailed)
			s.saveResult(ctx, job, item, domain.ResultFailed, item.Message)
			s.markResourceError(ctx, item, item.Message)
		case ActionIgnored:
			s.markIgnored(ctx, job, item, t)
		case ActionUnchanged:
			t.record(domain.ResultUnchanged)
			s.saveResult(ctx, job, item, domain.ResultUnchanged, "")
			// Content is identical, but the PATH may have moved (a rename from vyzorix.toml
			// to vyzorix.v). The row records the path, so leaving it stale means a later
			// delete of the new path reports "missing" against a file that is gone under the
			// old name — and the drift check compares the wrong path. Re-point it.
			s.refreshPath(ctx, item)
		}
	}

	return t
}

// refreshPath re-points an unchanged resource at its current path when the manifest moved.
//
// It is a no-op when the path is the same, so the common case writes nothing.
func (s *SyncService) refreshPath(ctx context.Context, item *PlanItem) {
	if item.Resource == nil || item.Manifest == nil {
		return
	}
	if item.Resource.Path == item.Manifest.Path {
		return
	}

	if err := s.resources.UpdatePath(ctx, item.Resource.ID, item.Manifest.Path); err != nil {
		s.log.Error("provisioning: could not update the resource path after a rename",
			"resource", item.Resource.ID, "from", item.Resource.Path, "to", item.Manifest.Path, "err", err)
	}
}

// markResourceError records the error state on a resource this repository already owns.
//
// A manifest that cannot be applied — a parse failure, or a device another repository
// claimed — must not keep reading "applied": the row is what the UI and the stats show,
// so leaving it stale would claim the device matches a manifest that is now broken. A
// create has no row yet, so there is nothing to mark.
func (s *SyncService) markResourceError(ctx context.Context, item *PlanItem, msg string) {
	if item.Resource == nil {
		return
	}

	if err := s.resources.SetState(ctx, item.Resource.ID, domain.ResourceError, msg); err != nil {
		s.log.Error("provisioning: could not record resource error state",
			"resource", item.Resource.ID, "err", err)
	}
}

// applyOne applies a single manifest through the deploy service and records the
// outcome.
func (s *SyncService) applyOne(ctx context.Context, repo *domain.Repository, job *domain.Job, item *PlanItem, t *tally) {
	if s.deployer == nil {
		t.record(domain.ResultFailed)
		s.saveResult(ctx, job, item, domain.ResultFailed, "the deploy service is not configured")

		return
	}

	// The device drifted, so its row must stop claiming it matches until the re-apply
	// below succeeds. Without this the drifted state was never written anywhere, and a
	// sync interrupted between detecting drift and re-applying left a false "applied".
	if item.Drifted && item.Resource != nil {
		if err := s.resources.SetState(ctx, item.Resource.ID, domain.ResourceDrifted, item.Message); err != nil {
			s.log.Error("provisioning: could not record drifted state", "resource", item.Resource.ID, "err", err)
		}
	}

	// MemberID is the actor that triggered this sync, so the signed config names who
	// authorised the change rather than an empty field. A scheduled or provider-triggered
	// sync names the scheduler/provider actor, which is the honest answer: nobody's user id
	// should be stamped on a change a machine made.
	result, err := s.deployer.Deploy(ctx, deploy.DeployRequest{
		Caller:   s.caller(repo),
		OrgID:    repo.OrgID,
		DeviceID: item.Manifest.DeviceID,
		MemberID: syncActorID(job.Actor),
		Manifest: item.Manifest.Bytes,
	})
	if err != nil {
		msg := err.Error()
		t.record(domain.ResultFailed)
		s.saveResult(ctx, job, item, domain.ResultFailed, msg)

		if item.Resource != nil {
			if setErr := s.resources.SetState(ctx, item.Resource.ID, domain.ResourceError, msg); setErr != nil {
				s.log.Error("provisioning: could not record resource error", "resource", item.Resource.ID, "err", setErr)
			}
		}

		return
	}

	resource := &domain.Resource{
		ID:            resourceID(item.Resource),
		RepoID:        repo.ID,
		Kind:          domain.KindDevice,
		DeviceID:      item.Manifest.DeviceID,
		Path:          item.Manifest.Path,
		ContentHash:   item.Manifest.ContentHash,
		ConfigVersion: result.Version,
		State:         domain.ResourceApplied,
		AppliedAt:     s.now(),
	}
	appliedAt := resource.AppliedAt
	resource.LastAppliedAt = &appliedAt

	if err := s.resources.Upsert(ctx, resource); err != nil {
		t.record(domain.ResultFailed)
		s.saveResult(ctx, job, item, domain.ResultFailed, err.Error())

		return
	}

	outcome := domain.ResultApplied
	if item.Action == ActionUpdate {
		outcome = domain.ResultUpdated
	}
	t.record(outcome)
	s.saveResult(ctx, job, item, outcome, "")
}

// markIgnored records that a resource is now outside the repository's scope.
//
// It changes the state and nothing else: no revert, no apply. The device keeps exactly
// what it has, because "this repository no longer claims this file" is not the same as
// "this file was deleted", and conflating the two would invite a revert nobody asked for.
func (s *SyncService) markIgnored(ctx context.Context, job *domain.Job, item *PlanItem, t *tally) {
	if item.Resource == nil {
		return
	}

	msg := "the file is outside this repository's include/exclude scope; the device keeps its current config"

	if err := s.resources.SetState(ctx, item.Resource.ID, domain.ResourceIgnored, msg); err != nil {
		s.log.Error("provisioning: could not mark resource ignored", "resource", item.Resource.ID, "err", err)
		t.record(domain.ResultFailed)
		s.saveResult(ctx, job, item, domain.ResultFailed, "could not record the ignored state: "+err.Error())

		return
	}

	t.record(domain.ResultIgnored)
	s.saveResult(ctx, job, item, domain.ResultIgnored, msg)
}

// markMissing flags a resource whose file disappeared. With revert it restores the
// device's previous config first, which is the explicit action the contract
// requires before anything is reverted.
func (s *SyncService) markMissing(ctx context.Context, repo *domain.Repository, job *domain.Job, item *PlanItem, revert bool, t *tally) {
	if item.Resource == nil {
		return
	}

	msg := "the manifest is no longer in the repository; the device keeps its current config"

	if revert {
		if err := s.revert(ctx, repo, item); err != nil {
			t.record(domain.ResultFailed)
			s.saveResult(ctx, job, item, domain.ResultFailed, "revert failed: "+err.Error())

			return
		}
		msg = "the manifest is no longer in the repository; reverted to the previous config"
	}

	// The reason is kept on the row: an operator reading a "missing" resource needs to know
	// whether it was reverted or left alone, and an empty last_error erased that.
	if err := s.resources.SetState(ctx, item.Resource.ID, domain.ResourceMissing, msg); err != nil {
		s.log.Error("provisioning: could not mark resource missing", "resource", item.Resource.ID, "err", err)
		t.record(domain.ResultFailed)
		s.saveResult(ctx, job, item, domain.ResultFailed, "could not record the missing state: "+err.Error())

		return
	}

	t.record(domain.ResultMissing)
	s.saveResult(ctx, job, item, domain.ResultMissing, msg)
}

// revert restores the device's previous config. It reports an error rather than
// claiming a revert that did not happen.
func (s *SyncService) revert(ctx context.Context, repo *domain.Repository, item *PlanItem) error {
	if s.deployer == nil {
		return errors.New("the deploy service is not configured")
	}

	if _, err := s.deployer.Rollback(ctx, s.caller(repo), repo.OrgID, item.Resource.DeviceID); err != nil {
		return err
	}

	return nil
}

// recordPlanResults writes the preview rows for a dry run. Nothing is persisted
// beyond the job: a dry run must not change a resource.
func (s *SyncService) recordPlanResults(ctx context.Context, job *domain.Job, plan *Plan, opts SyncOptions) *tally {
	t := &tally{}

	for _, item := range plan.Items {
		// A skipped item is previewed as skipped, not passed to previewOutcome: it has no
		// action, and the default branch there would read it as a failure.
		if item.Skipped {
			t.record(domain.ResultSkipped)
			s.saveResult(ctx, job, item, domain.ResultSkipped, item.Message)

			continue
		}

		outcome, msg := previewOutcome(item)

		// A dry run with revert_missing would really revert a device, so the preview must
		// say so. Reporting plain "missing" for a run that would roll the device back
		// understates the change the user is approving.
		if opts.RevertMissing && item.Action == ActionMissing {
			msg = "would revert to the previous config; the manifest is no longer in the repository"
		}

		t.record(outcome)
		s.saveResult(ctx, job, item, outcome, msg)
	}

	return t
}

// previewOutcome renders a plan item as a job result. It must return the same
// outcome a real run would, so the preview and the result can be compared line for
// line: a conflict is a failure in both, never "skipped" in one and "failed" in the
// other.
func previewOutcome(item *PlanItem) (domain.JobResultOutcome, string) {
	switch item.Action {
	case ActionCreate:
		return domain.ResultApplied, "would apply"
	case ActionUpdate:
		return domain.ResultUpdated, "would update"
	case ActionUnchanged:
		return domain.ResultUnchanged, ""
	case ActionMissing:
		return domain.ResultMissing, item.Message
	case ActionIgnored:
		return domain.ResultIgnored, item.Message
	case ActionConflict:
		return domain.ResultFailed, item.Message
	default:
		return domain.ResultFailed, item.Message
	}
}

// saveResult appends one row to the job. A failure to record is logged rather
// than propagated: the apply already happened, and losing its row must not
// cascade into a second failure.
func (s *SyncService) saveResult(ctx context.Context, job *domain.Job, item *PlanItem, outcome domain.JobResultOutcome, msg string) {
	path := ""
	resourceID := ""
	if item.Manifest != nil {
		path = item.Manifest.Path
	}
	if item.Resource != nil {
		resourceID = item.Resource.ID
		// A missing resource has no manifest, so its path comes from the record it
		// still owns; without it the row names no file.
		if path == "" {
			path = item.Resource.Path
		}
	}

	res := &domain.JobResult{
		ID:         uuid.New().String(),
		JobID:      job.ID,
		ResourceID: resourceID,
		Path:       path,
		Outcome:    outcome,
		Message:    msg,
	}
	if err := s.jobs.SaveResult(ctx, res); err != nil {
		s.log.Error("provisioning: could not record job result", "job", job.ID, "path", path, "err", err)
	}
}

// finishJob writes the counters and the terminal state, and stamps the repository.
func (s *SyncService) finishJob(ctx context.Context, repo *domain.Repository, job *domain.Job, t *tally, singleResource bool) (*SyncResult, error) {
	job.Applied = t.applied
	job.Skipped = t.skipped
	job.Failed = t.failed

	switch {
	case job.Failed > 0:
		job.State = domain.SyncError
	case t.warned > 0:
		job.State = domain.SyncWarning
	default:
		job.State = domain.SyncSuccess
	}

	// A run with per-resource failures is an error, and the repository's last_error is
	// what the UI shows beside the red badge. Without a summary it read red with no
	// reason, which is worse than the generic message this supplies.
	if job.Failed > 0 && job.Error == "" {
		job.Error = fmt.Sprintf("%d manifest(s) failed", job.Failed)
	}

	finished := s.now()
	s.finishStamp(job, finished)

	if err := s.jobs.FinishJob(ctx, job); err != nil {
		return nil, err
	}

	// A dry run changes nothing, so it must not stamp the repository. last_sync_at is what
	// ListDueForSync compares against, so stamping it here would reset the sync interval and
	// could suppress the next real run for a whole interval. It IS still audited, because
	// "someone previewed a sync" is worth a row even though it wrote nothing.
	//
	// A single-resource sync does not stamp either: it did not sync the repository, and
	// stamping would claim the whole repository is current when only one file was applied.
	if job.DryRun || singleResource {
		s.recordSyncEvent(ctx, repo, job)
	} else {
		s.stampRepository(ctx, repo, job)
		s.recordSyncEvent(ctx, repo, job)
	}

	return &SyncResult{Job: job}, nil
}

// resolveActor guarantees every job carries a valid actor.
//
// A caller that supplies nothing is recorded as the system rather than left blank: an
// unattributed provisioning change is exactly what the event table exists to prevent,
// and a sync must never write one.
func resolveActor(a domain.Actor) domain.Actor {
	if a.Kind.Valid() && a.ID != "" {
		return a
	}

	return domain.SystemActor()
}

// finishStamp sets the finish time and the duration derived from it.
//
// Both are set together so a job can never record one without the other: a duration
// computed against a stale start, or a finish time with no duration, would both be
// wrong in the health trend.
func (s *SyncService) finishStamp(job *domain.Job, finished time.Time) {
	job.FinishedAt = &finished

	if elapsed := finished.Sub(job.StartedAt); elapsed > 0 {
		job.DurationMs = elapsed.Milliseconds()
	}
}

// stampRepository records the sync outcome on the repository, so a list view can
// show sync health without reading the job table.
//
// It writes only the sync-owned columns. A full-row save would overwrite a
// branch/path/interval the user edited while the sync was running, silently
// reverting their change with values read before the edit landed.
func (s *SyncService) stampRepository(ctx context.Context, repo *domain.Repository, job *domain.Job) {
	repo.SyncState = job.State
	repo.LastError = job.Error
	repo.LastSyncAt = job.FinishedAt

	if err := s.repositories.UpdateSyncStatus(ctx, repo.ID, job.State, job.Error, s.now()); err != nil {
		s.log.Error("provisioning: could not stamp repository after sync", "repo", repo.ID, "err", err)
	}
}

// recordSyncEvent appends the audit row for a finished sync, attributed to whoever
// triggered it.
//
// It runs on both the success and failure path (they share stampRepository), so a
// failed sync is still attributed. A recording failure is logged, never returned: the
// sync already happened, and losing its audit row must not turn it into an error.
func (s *SyncService) recordSyncEvent(ctx context.Context, repo *domain.Repository, job *domain.Job) {
	if s.auditor == nil {
		return
	}

	result := "success"
	if job.State == domain.SyncError {
		result = "error"
	}

	event := &domain.ProvisioningEvent{
		ID:           uuid.New().String(),
		OrgID:        repo.OrgID,
		Actor:        job.Actor,
		Type:         domain.EventSyncApplied,
		ResourceType: "repository",
		ResourceID:   repo.ID,
		Result:       result,
		Detail:       syncEventDetail(job),
		CreatedAt:    s.now(),
	}

	// A dry run changes nothing, so it is recorded as a trigger rather than an apply.
	if job.DryRun {
		event.Type = domain.EventSyncTriggered
	}

	// A failed write is escalated by the Auditor; the sync itself already succeeded,
	// so its error is not propagated here.
	if err := s.auditor.Record(ctx, event); err != nil {
		s.log.Error("provisioning: could not record sync event", "repo", repo.ID, "job", job.ID, "err", err)
	}
}

// syncEventDetail summarises a run without any manifest content, which could name a
// device or carry a value the audit reader should not see.
func syncEventDetail(job *domain.Job) string {
	detail := fmt.Sprintf("applied=%d skipped=%d failed=%d", job.Applied, job.Skipped, job.Failed)
	if job.Error != "" {
		detail += ": " + job.Error
	}

	return detail
}

// failJob records a job-level failure (the fetch or the plan could not be done).
func (s *SyncService) failJob(ctx context.Context, repo *domain.Repository, job *domain.Job, err error) (*SyncResult, error) {
	job.State = domain.SyncError
	job.Error = err.Error()

	finished := s.now()
	s.finishStamp(job, finished)

	if finishErr := s.jobs.FinishJob(ctx, job); finishErr != nil {
		s.log.Error("provisioning: could not record failed job", "job", job.ID, "err", finishErr)
	}

	// Same rule as finishJob: a dry run is audited but never stamps the repository, so a
	// failed preview cannot reset the interval and delay the real sync that would find out
	// whether the remote is really broken.
	if !job.DryRun {
		s.stampRepository(ctx, repo, job)
	}
	s.recordSyncEvent(ctx, repo, job)

	return nil, err
}

// syncActorID is the id a deploy is attributed to: the user or service account that
// triggered the sync, or a stable name for the scheduler/provider/system actor.
func syncActorID(a domain.Actor) string {
	if a.ID != "" {
		return a.ID
	}

	return domain.SystemActor().ID
}

// caller builds the deploy principal a repository acts as.
//
// A provisioned repository may rewrite any device's config in its organisation,
// which is exactly what provisioning.write grants. The caller is therefore a
// member of the org holding that authority, expressed as an explicit device-deploy
// grant over every device.
//
// It is NOT marked IsSuperAdmin. That flag is a bypass in deploy.Authorize that skips
// the grant check entirely, so setting it here would inherit every future
// superadmin-only privilege the deploy path gains — authority nobody granted to
// provisioning. Declaring the one grant the sync actually needs keeps the blast
// radius to what provisioning.write already allows.
func (s *SyncService) caller(repo *domain.Repository) *deploy.Caller {
	return &deploy.Caller{
		OrgID:         repo.OrgID,
		Authenticated: true,
		Permissions: permission.ScopedPermissions{
			{Action: permission.ActionDeviceDeploy, Scope: permission.WildcardScope(permission.ScopeDevices)},
		},
	}
}

// resourceID reuses an existing row's id when updating, so an update keeps the
// resource's identity rather than replacing the row.
func resourceID(existing *domain.Resource) string {
	if existing != nil {
		return existing.ID
	}

	return uuid.New().String()
}

// locks serializes syncs per repository.
//
// The service is a singleton shared by every HTTP handler and the scheduler, so
// the map is guarded: an unsynchronised map read and write from two goroutines is
// a data race, and Go may abort the process on a corrupted map rather than merely
// miscount.
type locks struct {
	held map[string]struct{}
	mu   sync.Mutex
}

func newLocks() *locks { return &locks{held: map[string]struct{}{}} }

// acquire claims a repository's slot, reporting false when one is already held.
func (l *locks) acquire(id string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, ok := l.held[id]; ok {
		return false
	}
	l.held[id] = struct{}{}

	return true
}

// release frees a repository's slot.
func (l *locks) release(id string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.held, id)
}

// SyncRepositoryStore is the repository persistence a sync needs. It is narrower
// than the full Store: a sync reads a repository and writes back only the
// sync-owned columns, so it cannot accidentally clobber the user's configuration.
type SyncRepositoryStore interface {
	GetRepository(ctx context.Context, orgID, id string) (*domain.Repository, error)
	ListDueForSync(ctx context.Context, now time.Time) ([]*domain.Repository, error)
	UpdateSyncStatus(ctx context.Context, repoID string, state domain.SyncState, lastError string, lastSyncAt time.Time) error
}

// SetAuditor injects the audit trail. It is optional: a service built without it
// still syncs, it just records no events.
func (s *SyncService) SetAuditor(a *Auditor) {
	s.auditor = a
}

// SyncService runs syncs for repositories.
type SyncService struct {
	fetcher      Fetcher
	resources    domain.ResourceRepository
	connections  ConnectionStore
	repositories SyncRepositoryStore
	deployer     Deployer
	jobs         domain.JobRepository
	auditor      *Auditor
	// orgAppliers maps a resource kind to the applier that writes it. A kind with no
	// applier is reported as an error for that file, not silently skipped: a repository
	// that declares orgs/acme/groups.toml and finds nothing happens should be told why.
	orgAppliers map[ResourceKind]OrgApplier
	orgNames    OrgNameLookup
	locks       *locks
	log         *slog.Logger
	now         func() time.Time
}

// OrgApplier writes one kind of org file for an organisation.
type OrgApplier interface {
	Apply(ctx context.Context, orgID string, data []byte) (*ApplyResult, error)
}

// SetOrgApplier registers the applier for a kind. It is a setter because each applier needs
// a repository the server builds alongside, after this service.
func (s *SyncService) SetOrgApplier(kind ResourceKind, a OrgApplier) {
	if s.orgAppliers == nil {
		s.orgAppliers = make(map[ResourceKind]OrgApplier, len(OrgFileNames))
	}

	s.orgAppliers[kind] = a
}

// OrgNameLookup resolves an organisation's name from its id.
//
// It exists so an org file's directory can be checked against the repository's own
// organisation. Without that check a file in orgs/some-other-org/ would silently write to
// whichever organisation the repository belongs to, which is the wrong direction for a
// mistake to fail in.
type OrgNameLookup interface {
	OrgName(ctx context.Context, orgID string) (string, error)
}

// SetOrgNameLookup injects the resolver. A service built without one skips the check, which
// keeps tests and the offline path working; the server always wires it.
func (s *SyncService) SetOrgNameLookup(l OrgNameLookup) { s.orgNames = l }

// Deployer is the slice of the deploy service a sync uses. It is an interface so
// the engine can be tested without the real compile/sign pipeline, and so a
// failure can be injected per device.
type Deployer interface {
	Deploy(ctx context.Context, req deploy.DeployRequest) (*deploy.DeployResult, error)
	// Rollback restores a device's previous config, used by revert_missing.
	Rollback(ctx context.Context, caller *deploy.Caller, orgID, deviceID string) (*deployconfig.Record, error)
	// CurrentVersion reports the version a device is meant to be running, so drift
	// is detected against what the device actually holds rather than assumed. It is
	// scoped to the repository's organisation, so a device that has moved to another
	// tenant reads as having no config rather than as drift.
	CurrentVersion(ctx context.Context, orgID, deviceID string) (int, bool, error)
}

// SetDeployer injects the deploy service.
//
// It is a setter rather than a constructor argument because the deploy service is
// built by the server after the handler set, the same ordering SetConfigPusher
// solves for the deploy handler. A sync run before it is set fails its job with a
// clear reason rather than panicking.
func (s *SyncService) SetDeployer(d Deployer) { s.deployer = d }

// NewSyncService builds a SyncService.
func NewSyncService(
	fetcher Fetcher,
	resources domain.ResourceRepository,
	connections ConnectionStore,
	repositories SyncRepositoryStore,
	deployer Deployer,
	jobs domain.JobRepository,
	log *slog.Logger,
) *SyncService {
	if log == nil {
		log = slog.Default()
	}

	return &SyncService{
		fetcher:      fetcher,
		resources:    resources,
		connections:  connections,
		repositories: repositories,
		deployer:     deployer,
		jobs:         jobs,
		locks:        newLocks(),
		log:          log,
		now:          time.Now,
	}
}
