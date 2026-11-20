package provisioning

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// Assembled so the committed file holds no contiguous provider-shaped literal.
var probeToken = "gh" + "p_" + "probeTOKENvalueThatMustNeverAppear1234"

// TestConnectionTokenIsASecret proves the credential the store returns cannot be
// printed. Before this change ConnectionToken returned a plain string, so any log line
// that happened to include it leaked the token.
func TestConnectionTokenIsASecret(t *testing.T) {
	conns := newFakeConnectionStore()
	conns.tokens["conn-1"] = probeToken

	token, err := conns.ConnectionToken(context.Background(), "", "conn-1")
	if err != nil {
		t.Fatalf("ConnectionToken: %v", err)
	}

	// Every rendering path a logger or an error message would use.
	renderings := []string{
		token.String(),
		fmt.Sprintf("%v", token),
		fmt.Sprintf("%+v", token),
		fmt.Sprintf("%#v", token),
		fmt.Sprintf("authenticating with %s", token),
		fmt.Sprintf("%v", map[string]any{"token": token}),
	}

	for _, out := range renderings {
		if strings.Contains(out, probeToken) {
			t.Fatalf("the token leaked through a rendering: %q", out)
		}
	}

	// It must still be usable where authentication genuinely needs it.
	if token.Reveal() != probeToken {
		t.Fatal("Reveal must return the plaintext for the caller that authenticates")
	}
}

// TestConnectionTokenNeverReachesALog is the end-to-end guard: the token flows through
// the same service the sync uses, and a logger records the connection, with the token
// masked at every step.
func TestConnectionTokenNeverReachesALog(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	conns := newFakeConnectionStore()
	conns.connections["conn-1"] = &domain.Connection{
		ID: "conn-1", OrgID: "org-1", Name: "gh", Type: domain.ConnectionGitHub,
	}
	conns.tokens["conn-1"] = probeToken

	token, err := conns.ConnectionToken(context.Background(), "", "conn-1")
	if err != nil {
		t.Fatalf("ConnectionToken: %v", err)
	}

	// A caller that logs the connection and the token, which is the mistake this type
	// exists to survive.
	conn, err := conns.GetConnection(context.Background(), "org-1", "conn-1")
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	logger.Info("provisioning connection", "connection", conn, "token", token)

	if strings.Contains(buf.String(), probeToken) {
		t.Fatalf("the log leaked the token: %s", buf.String())
	}
}

// TestSaveConnectionWithTokenTakesASecret proves the write path is typed too, so a
// caller cannot pass a bare string by accident.
func TestSaveConnectionWithTokenTakesASecret(t *testing.T) {
	conns := newFakeConnectionStore()

	conn := &domain.Connection{ID: "conn-1", OrgID: "org-1", Name: "gh", Type: domain.ConnectionGitHub}
	if err := conns.SaveConnectionWithToken(context.Background(), conn, redaction.NewSecret(probeToken)); err != nil {
		t.Fatalf("SaveConnectionWithToken: %v", err)
	}

	// The fake stores plaintext internally; what matters is that the boundary is typed.
	if conns.tokens["conn-1"] != probeToken {
		t.Fatal("the token was not stored")
	}
}

// TestRenameDoesNotBlankTheToken is the regression guard for a Secret-specific hazard:
// a zero Secret must mean "not supplied", so a rename cannot wipe the credential.
func TestRenameDoesNotBlankTheToken(t *testing.T) {
	conns := newFakeConnectionStore()
	conns.connections["conn-1"] = &domain.Connection{
		ID: "conn-1", OrgID: "org-1", Name: "gh", Type: domain.ConnectionGitHub,
	}
	conns.tokens["conn-1"] = probeToken

	svc := NewConnectionService(conns, nil, nil)

	// A rename sends no token.
	if _, err := svc.Update(context.Background(), "org-1", "conn-1", ConnectionInput{
		Actor: domain.SystemActor(),
		Name:  "renamed",
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if conns.tokens["conn-1"] != probeToken {
		t.Fatal("a rename blanked the stored credential")
	}
}
