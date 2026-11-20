package deploy

import (
	"context"
	"testing"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/deployconfig"
)

// TestDeclaredSecretsReadsNames proves the declared secret names are read from a signed
// config by parsing it, so a comment or another app's node cannot be mistaken for one.
func TestDeclaredSecretsReadsNames(t *testing.T) {
	doc := `' a config
v device d_1
vv org org_1
vv apps
vvv app web
vvvv secret API_TOKEN
vvvv secret DB_URL
vvv app worker
vvvv secret QUEUE_KEY
`
	got, err := DeclaredSecrets(doc)
	if err != nil {
		t.Fatalf("DeclaredSecrets: %v", err)
	}

	want := map[string]string{"API_TOKEN": "web", "DB_URL": "web", "QUEUE_KEY": "worker"}
	if len(got) != len(want) {
		t.Fatalf("got %d declarations, want %d: %+v", len(got), len(want), got)
	}
	for _, r := range got {
		if want[r.Key] != r.App {
			t.Fatalf("%s declared under %q, want %q", r.Key, r.App, want[r.Key])
		}
	}
}

// TestSecretReadinessReportsSetAndMissing proves the readiness read marks each declared key
// set or missing, which is what lets the console offer to fix a key before the device asks
// for it and gets a 404.
func TestSecretReadinessReportsSetAndMissing(t *testing.T) {
	svc, configs, _ := serviceFixture(t, deployCaller())

	// A stored config that declares two keys.
	configs.records["d_01H"] = &deployconfig.Record{
		DeviceID: "d_01H",
		Kind:     deployconfig.KindCurrent,
		Version:  1,
		Document: "' c\nv device d_01H\nvv apps\nvvv app web\nvvvv secret PRESENT\nvvvv secret ABSENT\n",
	}

	svc.SetSecretReadiness(&stubSecrets{byApp: map[string][]string{"web": {"PRESENT"}}})

	got, ok, err := svc.SecretReadinessFor(context.Background(), "org_01H", "d_01H")
	if err != nil {
		t.Fatalf("SecretReadinessFor: %v", err)
	}
	if !ok {
		t.Fatal("a device with a config read as having none")
	}

	byKey := map[string]bool{}
	for _, r := range got {
		byKey[r.Key] = r.Set
	}
	if !byKey["PRESENT"] {
		t.Fatal("a set secret read as missing")
	}
	if byKey["ABSENT"] {
		t.Fatal("a missing secret read as set")
	}
}

// TestSecretReadinessWithoutAConfig proves a device that has never been deployed to reads
// as "no config", which the route turns into a 404 rather than an empty list a client would
// read as "nothing required".
func TestSecretReadinessWithoutAConfig(t *testing.T) {
	svc, _, _ := serviceFixture(t, deployCaller())

	_, ok, err := svc.SecretReadinessFor(context.Background(), "org_01H", "d_01H")
	if err != nil {
		t.Fatalf("SecretReadinessFor: %v", err)
	}
	if ok {
		t.Fatal("a device with no config read as having one")
	}
}
