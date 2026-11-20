package api

import (
	"testing"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/audit"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploy"
)

// TestDeployAuditConstantsMatchAuditPackage is the drift guard for the two vocabularies.
//
// The deploy package spells the audit action and result values out as string constants
// because it must not import the audit package. That copy is only safe while it matches:
// if someone renames audit.ActionDeployRolledBack, the service would keep emitting the old
// string and every rollback would land in the audit table under an action nothing
// recognises. This test is the join, so the rename fails here rather than silently.
func TestDeployAuditConstantsMatchAuditPackage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"deployed", deploy.ActionDeployed, string(audit.ActionDeploySucceeded)},
		{"rolled back", deploy.ActionRolledBack, string(audit.ActionDeployRolledBack)},
		{"applied", deploy.ActionApplied, string(audit.ActionDeployApplied)},
		{"success result", deploy.ResultSuccess, string(audit.ResultSuccess)},
	}

	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s: deploy emits %q but the audit package defines %q", tc.name, tc.got, tc.want)
		}
	}
}

// TestEveryDeployAuditActionIsHandled pins the deploy action set: each one must be a
// distinct non-empty value, so two actions cannot collapse into one audit trail entry.
func TestEveryDeployAuditActionIsHandled(t *testing.T) {
	t.Parallel()

	actions := []audit.Action{
		audit.ActionDeploySucceeded,
		audit.ActionDeployFailed,
		audit.ActionDeployRolledBack,
		audit.ActionDeployApplied,
		audit.ActionDeployCredentialNew,
		audit.ActionDeployCredentialOff,
		audit.ActionDeploySecretSet,
		audit.ActionDeploySecretUnset,
	}

	seen := make(map[audit.Action]bool, len(actions))

	for _, a := range actions {
		if a == "" {
			t.Fatal("a deploy audit action is empty")
		}
		if seen[a] {
			t.Fatalf("duplicate deploy audit action %q", a)
		}

		seen[a] = true
	}
}
