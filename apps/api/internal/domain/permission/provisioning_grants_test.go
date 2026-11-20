package permission

import "testing"

// TestProvisioningGrantedOnlyToAdmins is the guard for the contract's §6 rule.
//
// A provisioned repository can rewrite any device's config, so write access to it
// is fleet-wide deploy authority. It is therefore granted to admin and super_admin
// and withheld from operator and viewer. The role ladder is the enforcement point:
// requireScope builds an Evaluator from the member's role, so a role that does not
// carry the action cannot reach the route however the request is shaped.
func TestProvisioningGrantedOnlyToAdmins(t *testing.T) {
	cases := []struct {
		role     string
		wantRead bool
		wantWrit bool
	}{
		{"viewer", false, false},
		{"operator", false, false},
		{"admin", true, true},
		{"super_admin", true, true},
		// An unknown role fails closed to the viewer set.
		{"not_a_role", false, false},
	}

	orgAll := WildcardScope(ScopeOrg)

	for _, tc := range cases {
		t.Run(tc.role, func(t *testing.T) {
			eval := NewEvaluator(tc.role, nil)

			if got := eval.Grants(ActionProvisioningRead, orgAll); got != tc.wantRead {
				t.Errorf("provisioning.read granted = %v, want %v", got, tc.wantRead)
			}
			if got := eval.Grants(ActionProvisioningWrite, orgAll); got != tc.wantWrit {
				t.Errorf("provisioning.write granted = %v, want %v", got, tc.wantWrit)
			}
		})
	}
}

// TestProvisioningWriteImpliesFleetDeployAuthority records why §6 grants it to
// admins only: the same capability that writes a repository can deploy to every
// device in the organisation.
func TestProvisioningWriteImpliesFleetDeployAuthority(t *testing.T) {
	if got := RequiredAccess(ActionProvisioningWrite); got != AccessAdmin {
		t.Fatalf("provisioning.write requires %v, want admin", got)
	}

	// Read is harmless: it exposes no secret and changes nothing.
	if got := RequiredAccess(ActionProvisioningRead); got != AccessRead {
		t.Fatalf("provisioning.read requires %v, want read", got)
	}
}

// TestProvisioningIsOrgScoped pins that the actions are satisfied by an
// organisation-scoped credential, never an instance or personal one.
func TestProvisioningIsOrgScoped(t *testing.T) {
	for _, action := range []Action{ActionProvisioningRead, ActionProvisioningWrite} {
		if got := actionScope[action]; got != ScopeOrg {
			t.Errorf("%s scope = %q, want %q", action, got, ScopeOrg)
		}
	}
}
