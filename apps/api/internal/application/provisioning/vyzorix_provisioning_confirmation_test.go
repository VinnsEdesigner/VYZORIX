package provisioning

import (
	"context"
	"testing"
	"time"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// fakeConfirmation answers with fixed evidence, so each test states the case it means.
//
// confirmsAll is the shorthand for "a device that is connected and running whatever version
// was just applied", which most apply-path tests want.
type fakeConfirmation struct {
	evidence    DeviceEvidence
	confirmsAll bool
}

func (f fakeConfirmation) ConfirmConfig(_ context.Context, _, _ string, version int) DeviceEvidence {
	if f.confirmsAll {
		return DeviceEvidence{Found: true, Online: true, ReportedVersion: reportedVersion(version), ReportedAt: time.Now()}
	}

	return f.evidence
}

func reportedVersion(v int) *int { return &v }

// TestAppliedRequiresDeviceConfirmation is the regression test for the honesty defect: a
// deploy only proves the server signed and stored a document. Until the device reports it
// runs that version, and that report is current, the resource must read `pending`, never
// `applied`.
func TestAppliedRequiresDeviceConfirmation(t *testing.T) {
	now := time.Now()

	cases := []struct {
		name        string
		evidence    DeviceEvidence
		wantState   domain.ResourceState
		wantMessage bool
	}{
		{
			name:      "device is online and reports this version",
			evidence:  DeviceEvidence{Found: true, Online: true, ReportedVersion: reportedVersion(3), ReportedAt: now},
			wantState: domain.ResourceApplied,
		},
		{
			name:      "device is offline but reported this version recently",
			evidence:  DeviceEvidence{Found: true, ReportedVersion: reportedVersion(3), ReportedAt: now.Add(-time.Minute)},
			wantState: domain.ResourceApplied,
		},
		{
			name:        "device exists but reports another version",
			evidence:    DeviceEvidence{Found: true, Online: true, ReportedVersion: reportedVersion(2), ReportedAt: now},
			wantState:   domain.ResourcePending,
			wantMessage: true,
		},
		{
			name:        "device never reported",
			evidence:    DeviceEvidence{Found: true, Online: true},
			wantState:   domain.ResourcePending,
			wantMessage: true,
		},
		{
			name:        "device is not registered",
			evidence:    DeviceEvidence{},
			wantState:   domain.ResourcePending,
			wantMessage: true,
		},
		{
			// The stored row says the right version, but nobody has heard from the device in a
			// long time. Reporting "applied" here is the exact claim this feature removes.
			name:        "device reports this version but is stale and unreachable",
			evidence:    DeviceEvidence{Found: true, ReportedVersion: reportedVersion(3), ReportedAt: now.Add(-48 * time.Hour)},
			wantState:   domain.ResourcePending,
			wantMessage: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &SyncService{confirmations: fakeConfirmation{evidence: tc.evidence}}
			state, reason := stateForApply(svc.confirmConfig(context.Background(), "org-1", "device-1", 3))

			if state != tc.wantState {
				t.Fatalf("state = %q, want %q", state, tc.wantState)
			}
			if tc.wantMessage && reason == "" {
				t.Fatal("a pending resource must explain why, or the UI shows a bare badge")
			}
			if !tc.wantMessage && reason != "" {
				t.Fatalf("a confirmed resource needs no reason, got %q", reason)
			}
		})
	}
}

// A stale report must name staleness, not "not reported yet": the two are different facts and
// the person reading the row needs to know which one it is.
func TestStaleReportExplainsItself(t *testing.T) {
	svc := &SyncService{
		confirmations: fakeConfirmation{evidence: DeviceEvidence{
			Found: true, ReportedVersion: reportedVersion(3), ReportedAt: time.Now().Add(-time.Hour),
		}},
	}

	_, reason := stateForApply(svc.confirmConfig(context.Background(), "org-1", "device-1", 3))
	if reason == "" || reason == "the device has not reported this config version yet" {
		t.Fatalf("a stale report must say it is stale, got %q", reason)
	}
}

// An unwired confirmation reader must fail closed: with no way to check, the sync reports
// pending rather than assuming success.
func TestNilConfirmationReaderFailsClosed(t *testing.T) {
	svc := &SyncService{}

	if got := svc.confirmConfig(t.Context(), "org-1", "device-1", 3); got != configUnverifiable {
		t.Fatalf("confirmConfig with no reader = %v, want configUnverifiable", got)
	}
}

// A single-result test for the interface plumbing, so a future refactor cannot quietly drop
// the check back to "assume applied".
func TestConfirmConfigUsesTheReader(t *testing.T) {
	svc := &SyncService{confirmations: fakeConfirmation{evidence: DeviceEvidence{
		Found: true, Online: true, ReportedVersion: reportedVersion(3), ReportedAt: time.Now(),
	}}}

	if got := svc.confirmConfig(t.Context(), "org-1", "device-1", 3); got != configConfirmed {
		t.Fatalf("confirmConfig = %v, want configConfirmed", got)
	}
}
