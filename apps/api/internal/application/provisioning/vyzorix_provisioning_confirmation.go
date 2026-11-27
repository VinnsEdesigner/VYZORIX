package provisioning

import (
	"context"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// DeviceEvidence is what the server knows about a device at the moment it is asked.
//
// It carries reachability as well as the reported version because a stored version is not
// the same claim as a live one. A device that reported version 5 a week ago and has been
// unreachable since is not evidence that it still runs version 5; stamping "applied" from
// that row would report a fact nobody can see, which is exactly the defect this type
// exists to prevent.
type DeviceEvidence struct {
	// ReportedAt is when the device last reported the version below. Zero when it has
	// never reported.
	ReportedAt time.Time
	// LastSeen is the last time the device was in contact at all, reported or not.
	LastSeen time.Time
	// ReportedVersion is the config version the device says it holds. Nil when it has
	// never reported one, which stays distinguishable from a reported 0.
	ReportedVersion *int
	// Online is true when the device holds a live connection right now.
	Online bool
	// Found is false when the device is not registered, or is not this organisation's to
	// manage. That is "cannot verify", which is different from "verified false".
	Found bool
}

// DeviceConfirmation answers "is this device actually running the config we signed?".
//
// It exists because a successful deploy proves only that the server compiled, authorised,
// signed and stored a document. It says nothing about the device: the device may not be
// registered, may be offline, or may simply not have fetched yet. Reporting such a resource
// as "applied" claims a fact the server cannot see, so the sync asks this first and records
// `pending` when the answer is not a confirmed, current match.
type DeviceConfirmation interface {
	// ConfirmConfig reports what the device is and what it last reported running.
	ConfirmConfig(ctx context.Context, orgID, deviceID string, version int) DeviceEvidence
}

// defaultEvidenceFreshness bounds how old a device report may be and still count as
// evidence while the device is unreachable. It matches the diagnostics offline threshold's
// default, so one feature never calls a device online while another calls it stale.
const defaultEvidenceFreshness = 5 * time.Minute

// confirmationOutcome reduces the evidence to the answers the sync needs.
type confirmationOutcome int

const (
	// configConfirmed: the device reports this version and the evidence is current — it is
	// connected now, or its report is inside the freshness window.
	configConfirmed confirmationOutcome = iota
	// configUnconfirmed: the device exists but does not (yet) report this version.
	configUnconfirmed
	// configStale: the device's stored report names this version, but it is unreachable and
	// the report is older than the freshness window. The row may be right, but nobody can
	// see it, so it is not evidence enough to claim "applied".
	configStale
	// configUnverifiable: no reader is wired, or the device is not registered.
	configUnverifiable
)

// evidenceFreshness is how long a report counts as evidence without a live connection.
func (s *SyncService) evidenceFreshness() time.Duration {
	if s.freshness > 0 {
		return s.freshness
	}

	return defaultEvidenceFreshness
}

func (s *SyncService) confirmConfig(ctx context.Context, orgID, deviceID string, version int) confirmationOutcome {
	if s.confirmations == nil {
		return configUnverifiable
	}

	evidence := s.confirmations.ConfirmConfig(ctx, orgID, deviceID, version)
	if !evidence.Found {
		return configUnverifiable
	}

	if evidence.ReportedVersion == nil || *evidence.ReportedVersion != version {
		return configUnconfirmed
	}

	// The version matches. Whether that counts depends on how current the evidence is: a
	// connected device is current by definition, and an unreachable one only while its last
	// report is inside the freshness window.
	if evidence.Online || s.reportIsFresh(evidence.ReportedAt) {
		return configConfirmed
	}

	return configStale
}

// reportIsFresh reports whether a device's last report is recent enough to stand in for a
// live one.
func (s *SyncService) reportIsFresh(reportedAt time.Time) bool {
	if reportedAt.IsZero() {
		return false
	}

	now := time.Now()
	if s.now != nil {
		now = s.now()
	}

	return now.Sub(reportedAt) <= s.evidenceFreshness()
}

// stateForApply decides the state a freshly applied device resource records.
//
// Only a device the server can actually see running the version earns `applied`.
// Everything else is `pending`, which is the truthful state: the config is stored and
// signed, and whether the device runs it is not yet known. `reason` is empty for a
// confirmation and explains the pending case otherwise, so the UI can say why rather than
// showing a bare badge.
func stateForApply(outcome confirmationOutcome) (domain.ResourceState, string) {
	switch outcome {
	case configConfirmed:
		return domain.ResourceApplied, ""
	case configUnconfirmed:
		return domain.ResourcePending, "the device has not reported this config version yet"
	case configStale:
		return domain.ResourcePending, "the device is offline and its last report is stale, so it cannot prove it runs this config"
	default:
		return domain.ResourcePending, "the device is not registered, so it cannot confirm the config"
	}
}
