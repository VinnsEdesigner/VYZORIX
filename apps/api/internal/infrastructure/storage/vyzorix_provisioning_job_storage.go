package storage

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// JobStore is the sync-history half of the provisioning store.
type JobStore interface {
	StartJob(ctx context.Context, job *provisioning.Job) error
	FinishJob(ctx context.Context, job *provisioning.Job) error
	GetJob(ctx context.Context, repoID, id string) (*provisioning.Job, error)
	ListJobs(ctx context.Context, repoID string, limit int) ([]*provisioning.Job, error)
	SaveResult(ctx context.Context, result *provisioning.JobResult) error
	ListResults(ctx context.Context, jobID string) ([]*provisioning.JobResult, error)
	FailInterrupted(ctx context.Context, olderThan time.Time) (int, error)
}

var _ JobStore = (*ProvisioningStorage)(nil)

const provisioningJobColumns = `id, repo_id, state, dry_run, applied, skipped, failed,
        error, started_at, finished_at, duration_ms`

// maxJobListLimit bounds a job listing at the storage layer. The handler already clamps
// its input, but a caller that reaches the store directly (a future job, a test helper)
// must not be able to ask for the whole table.
const maxJobListLimit = 200

// clampLimit applies a default and a ceiling to a caller-supplied limit.
func clampLimit(limit, def, ceiling int) int {
	if limit <= 0 {
		return def
	}
	if limit > ceiling {
		return ceiling
	}

	return limit
}

// StartJob records a job in the working state.
func (r *ProvisioningStorage) StartJob(ctx context.Context, job *provisioning.Job) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO provisioning_jobs
                        (id, repo_id, state, dry_run, applied, skipped, failed, error,
                         started_at, finished_at)
                 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)`,
		job.ID, job.RepoID, string(job.State), boolToInt(job.DryRun),
		job.Applied, job.Skipped, job.Failed, job.Error, job.StartedAt.UnixMilli(),
	)

	return err
}

// FinishJob writes the terminal state and counters.
func (r *ProvisioningStorage) FinishJob(ctx context.Context, job *provisioning.Job) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE provisioning_jobs
                 SET state = ?, applied = ?, skipped = ?, failed = ?, error = ?,
                     finished_at = ?, duration_ms = ?
                 WHERE id = ?`,
		string(job.State), job.Applied, job.Skipped, job.Failed, job.Error,
		nullTimeMillis(job.FinishedAt), job.DurationMs, job.ID,
	)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return provisioning.ErrJobNotFound
	}

	return nil
}

// GetJob returns one job, scoped to its repository so a job id from another
// repository cannot be read through the wrong route.
func (r *ProvisioningStorage) GetJob(ctx context.Context, repoID, id string) (*provisioning.Job, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+provisioningJobColumns+`
                 FROM provisioning_jobs WHERE repo_id = ? AND id = ?`, repoID, id)

	return scanProvisioningJob(row)
}

// ListJobs returns a repository's recent jobs, newest first.
func (r *ProvisioningStorage) ListJobs(ctx context.Context, repoID string, limit int) ([]*provisioning.Job, error) {
	limit = clampLimit(limit, 20, maxJobListLimit)

	rows, err := r.db.QueryContext(ctx,
		`SELECT `+provisioningJobColumns+`
                 FROM provisioning_jobs WHERE repo_id = ?
                 ORDER BY started_at DESC LIMIT ?`, repoID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []*provisioning.Job
	for rows.Next() {
		job, err := scanProvisioningJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}

	return out, rows.Err()
}

// SaveResult appends one manifest's outcome to a job.
func (r *ProvisioningStorage) SaveResult(ctx context.Context, res *provisioning.JobResult) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO provisioning_job_results (id, job_id, resource_id, path, outcome, message)
                 VALUES (?, ?, ?, ?, ?, ?)`,
		res.ID, res.JobID, res.ResourceID, res.Path, string(res.Outcome), res.Message,
	)

	return err
}

// ListResults returns every result row for a job.
func (r *ProvisioningStorage) ListResults(ctx context.Context, jobID string) ([]*provisioning.JobResult, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, job_id, resource_id, path, outcome, message
                 FROM provisioning_job_results WHERE job_id = ? ORDER BY path`, jobID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []*provisioning.JobResult
	for rows.Next() {
		var res provisioning.JobResult
		var outcome string
		if err := rows.Scan(&res.ID, &res.JobID, &res.ResourceID, &res.Path, &outcome, &res.Message); err != nil {
			return nil, err
		}
		res.Outcome = provisioning.JobResultOutcome(outcome)
		out = append(out, &res)
	}

	return out, rows.Err()
}

// FailInterrupted corrects jobs left working by a restart.
//
// A job cannot outlive the process running it, so a working row older than the
// cutoff is a lie the next boot must fix. Only jobs started before the cutoff are
// touched, so a sync another worker started a moment ago is not stolen.
func (r *ProvisioningStorage) FailInterrupted(ctx context.Context, olderThan time.Time) (int, error) {
	// duration_ms is set alongside finished_at. Leaving it at 0 made an interrupted job
	// report a zero-second duration, and the health view averages that column, so one
	// restart silently pulled the repository's reported sync time toward zero.
	now := time.Now().UnixMilli()

	res, err := r.db.ExecContext(ctx,
		`UPDATE provisioning_jobs
		 SET state = 'error', error = 'interrupted by a server restart', finished_at = ?,
		     duration_ms = MAX(0, ? - started_at)
		 WHERE state = 'working' AND started_at < ?`,
		now, now, olderThan.UnixMilli(),
	)
	if err != nil {
		return 0, err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	return int(n), nil
}

// Health returns a repository's sync health over its most recent completed runs.
//
// Only finished runs count: a working job has no duration and no outcome, so
// including it would drag the average down and break the streak. Dry runs are
// excluded too — they apply nothing, so their speed says nothing about how long a
// real sync takes.
func (r *ProvisioningStorage) Health(ctx context.Context, repoID string, limit int) (*provisioning.RepositoryHealth, error) {
	limit = clampLimit(limit, 20, maxJobListLimit)

	rows, err := r.db.QueryContext(ctx,
		`SELECT state, duration_ms, failed, finished_at
                 FROM provisioning_jobs
                 WHERE repo_id = ? AND finished_at IS NOT NULL AND dry_run = 0
                 ORDER BY started_at DESC LIMIT ?`, repoID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	health := &provisioning.RepositoryHealth{}
	var totalDuration int64

	for rows.Next() {
		var state string
		var durationMs int64
		var failed int
		var finishedAt int64

		if err := rows.Scan(&state, &durationMs, &failed, &finishedAt); err != nil {
			return nil, err
		}

		if state == string(provisioning.SyncError) {
			health.Failures++
		} else {
			health.Successes++
		}

		totalDuration += durationMs
		health.Timeline = append(health.Timeline, provisioning.HealthPoint{
			FinishedAt: time.UnixMilli(finishedAt),
			State:      provisioning.SyncState(state),
			DurationMs: durationMs,
			Failed:     failed,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	health.Timeline = reverseHealthPoints(health.Timeline)

	if n := len(health.Timeline); n > 0 {
		health.LastDurationMs = health.Timeline[n-1].DurationMs
		health.AverageDurationMs = totalDuration / int64(n)
		health.FailureStreak = failureStreak(health.Timeline)
	}

	return health, nil
}

// failureStreak counts the trailing runs that failed. The timeline is oldest first,
// so the streak is read from the end.
func failureStreak(timeline []provisioning.HealthPoint) int {
	streak := 0

	for i := len(timeline) - 1; i >= 0; i-- {
		if timeline[i].State != provisioning.SyncError {
			break
		}
		streak++
	}

	return streak
}

// reverseHealthPoints flips newest-first into oldest-first, in place.
func reverseHealthPoints(points []provisioning.HealthPoint) []provisioning.HealthPoint {
	for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
		points[i], points[j] = points[j], points[i]
	}

	return points
}

func scanProvisioningJob(scanner interface{ Scan(...any) error }) (*provisioning.Job, error) {
	var job provisioning.Job
	var state string
	var dryRun int
	var startedAt int64
	var finishedAt sql.NullInt64

	err := scanner.Scan(&job.ID, &job.RepoID, &state, &dryRun, &job.Applied, &job.Skipped,
		&job.Failed, &job.Error, &startedAt, &finishedAt, &job.DurationMs)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, provisioning.ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}

	job.State = provisioning.SyncState(state)
	job.DryRun = dryRun == 1
	job.StartedAt = time.UnixMilli(startedAt)
	if finishedAt.Valid {
		t := time.UnixMilli(finishedAt.Int64)
		job.FinishedAt = &t
	}

	return &job, nil
}
