package deploy

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// stubSecrets is a SecretReadiness with a fixed answer per app.
type stubSecrets struct {
	byApp map[string][]string
	err   error
}

func (s *stubSecrets) SetSecretNames(_ context.Context, _, app string) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.byApp[app], nil
}

// manifestWithSecret is a manifest whose app declares one required secret key.
const manifestWithSecret = `
[device]
id = "d_01H"
org = "org_01H"

[[app]]
name = "web"
cmd  = "/home/apps/web/run"

[app.secrets]
API_TOKEN = ""
`

// TestDeployRefusesAMissingDeclaredSecret proves a manifest that names a secret the device
// has not been given is refused at apply time, with a message naming the app and key. The
// alternative is a device that gets a config naming a secret it can never fetch.
func TestDeployRefusesAMissingDeclaredSecret(t *testing.T) {
	svc, _, caller := serviceFixture(t, deployCaller())
	svc.SetSecretReadiness(&stubSecrets{byApp: map[string][]string{"web": {}}})

	_, err := svc.Deploy(context.Background(), DeployRequest{
		Caller:   caller,
		Manifest: []byte(manifestWithSecret),
		OrgID:    "org_01H",
		DeviceID: "d_01H",
		MemberID: "op_01H",
	})

	var secretErr *SecretError
	if !errors.As(err, &secretErr) {
		t.Fatalf("got %v, want a SecretError", err)
	}
	if secretErr.App != "web" || secretErr.Key != "API_TOKEN" {
		t.Fatalf("error names %s/%s, want web/API_TOKEN", secretErr.App, secretErr.Key)
	}
	if !strings.Contains(err.Error(), "has not been set") {
		t.Fatalf("error does not explain the problem: %v", err)
	}
}

// TestDeployAcceptsAPresentDeclaredSecret is the counterweight: once the key is set the
// deploy proceeds.
func TestDeployAcceptsAPresentDeclaredSecret(t *testing.T) {
	svc, _, caller := serviceFixture(t, deployCaller())
	svc.SetSecretReadiness(&stubSecrets{byApp: map[string][]string{"web": {"API_TOKEN"}}})

	if _, err := svc.Deploy(context.Background(), DeployRequest{
		Caller:   caller,
		Manifest: []byte(manifestWithSecret),
		OrgID:    "org_01H",
		DeviceID: "d_01H",
		MemberID: "op_01H",
	}); err != nil {
		t.Fatalf("a declared and set secret was refused: %v", err)
	}
}

// TestDeployWithoutSecretStoreSkipsTheCheck proves the check is a property of the running
// server: a service with no store wired deploys as before, which is what keeps the offline
// path and the existing tests working.
func TestDeployWithoutSecretStoreSkipsTheCheck(t *testing.T) {
	svc, _, caller := serviceFixture(t, deployCaller())

	if _, err := svc.Deploy(context.Background(), DeployRequest{
		Caller:   caller,
		Manifest: []byte(manifestWithSecret),
		OrgID:    "org_01H",
		DeviceID: "d_01H",
		MemberID: "op_01H",
	}); err != nil {
		t.Fatalf("a deploy without a secret store was refused: %v", err)
	}
}

// TestDeployRefusesWhenTheSecretStoreErrors proves a store failure is surfaced, not read as
// "no secrets set" — which would turn a database fault into a misleading missing-secret
// message.
func TestDeployRefusesWhenTheSecretStoreErrors(t *testing.T) {
	svc, _, caller := serviceFixture(t, deployCaller())
	svc.SetSecretReadiness(&stubSecrets{err: errors.New("store unavailable")})

	_, err := svc.Deploy(context.Background(), DeployRequest{
		Caller:   caller,
		Manifest: []byte(manifestWithSecret),
		OrgID:    "org_01H",
		DeviceID: "d_01H",
		MemberID: "op_01H",
	})
	if err == nil || !strings.Contains(err.Error(), "store unavailable") {
		t.Fatalf("a store error was swallowed: %v", err)
	}
}

// TestDeployWithNoDeclaredSecretsIsUnaffected proves an ordinary manifest never touches the
// secret store, so the check costs nothing when it does not apply.
func TestDeployWithNoDeclaredSecretsIsUnaffected(t *testing.T) {
	svc, _, caller := serviceFixture(t, deployCaller())
	svc.SetSecretReadiness(&stubSecrets{err: errors.New("should not be called")})

	const noSecrets = `
[device]
id = "d_01H"
org = "org_01H"

[[app]]
name = "web"
cmd  = "/home/apps/web/run"
`

	if _, err := svc.Deploy(context.Background(), DeployRequest{
		Caller:   caller,
		Manifest: []byte(noSecrets),
		OrgID:    "org_01H",
		DeviceID: "d_01H",
		MemberID: "op_01H",
	}); err != nil {
		t.Fatalf("a manifest with no secrets read the secret store: %v", err)
	}
}
