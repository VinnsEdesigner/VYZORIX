package provisioning

import (
	"context"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/alert"
)

// memAlerts is an in-memory alert.Repository.
type memAlerts struct {
	rules map[string]*alert.Rule
}

func newMemAlerts() *memAlerts { return &memAlerts{rules: map[string]*alert.Rule{}} }

func (m *memAlerts) Save(_ context.Context, rule *alert.Rule) error {
	copied := *rule
	m.rules[rule.ID] = &copied

	return nil
}

func (m *memAlerts) GetByID(_ context.Context, id string) (*alert.Rule, error) {
	r, ok := m.rules[id]
	if !ok {
		return nil, alert.ErrNotFound
	}
	copied := *r

	return &copied, nil
}

func (m *memAlerts) ListByOrg(_ context.Context, orgID string) ([]*alert.Rule, error) {
	var out []*alert.Rule
	for _, r := range m.rules {
		if r.OrgID == orgID {
			copied := *r
			out = append(out, &copied)
		}
	}

	return out, nil
}

func (m *memAlerts) ListEnabled(context.Context) ([]*alert.Rule, error) { return nil, nil }

func (m *memAlerts) Delete(_ context.Context, id string) (bool, error) {
	_, ok := m.rules[id]
	delete(m.rules, id)

	return ok, nil
}

const alertFileOneRule = `
[[rule]]
name      = "device-offline"
metric    = "device_offline_count"
condition = "gt"
threshold = 0
for       = "5m"
notify    = "5m"
enabled   = true
`

// TestAlertApplierCreatesRules proves a file with a new rule creates it.
func TestAlertApplierCreatesRules(t *testing.T) {
	store := newMemAlerts()
	a := NewAlertApplier(store)
	a.now = func() time.Time { return time.Unix(0, 0) }

	res, err := a.Apply(context.Background(), "org-1", []byte(alertFileOneRule))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Created != 1 || res.Updated != 0 {
		t.Fatalf("result = %+v, want 1 created", res)
	}

	rules, _ := store.ListByOrg(context.Background(), "org-1")
	if len(rules) != 1 {
		t.Fatalf("stored %d rules, want 1", len(rules))
	}
	if rules[0].ForSeconds != 300 || rules[0].NotifyIntervalSeconds != 300 {
		t.Fatalf("durations not parsed: for=%d notify=%d", rules[0].ForSeconds, rules[0].NotifyIntervalSeconds)
	}
	if !rules[0].Enabled {
		t.Fatal("enabled was not applied")
	}
}

// TestAlertApplierUpdatesByName proves a rule already in the org is updated in place, not
// duplicated. Matching by name is what lets a file be the source of truth.
func TestAlertApplierUpdatesByName(t *testing.T) {
	store := newMemAlerts()
	store.rules["existing"] = &alert.Rule{
		ID: "existing", OrgID: "org-1", Name: "device-offline",
		Metric: alert.MetricDeviceOfflineCount, Condition: alert.ConditionGt, Threshold: 99,
	}

	a := NewAlertApplier(store)
	a.now = func() time.Time { return time.Unix(0, 0) }

	res, err := a.Apply(context.Background(), "org-1", []byte(alertFileOneRule))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Updated != 1 || res.Created != 0 {
		t.Fatalf("result = %+v, want 1 updated", res)
	}

	rules, _ := store.ListByOrg(context.Background(), "org-1")
	if len(rules) != 1 {
		t.Fatalf("stored %d rules, want 1 (no duplicate)", len(rules))
	}
	if rules[0].ID != "existing" {
		t.Fatalf("the existing rule was replaced, not updated: id=%s", rules[0].ID)
	}
	if rules[0].Threshold != 0 {
		t.Fatalf("threshold = %v, want the file's 0", rules[0].Threshold)
	}
}

// TestAlertApplierLeavesUnnamedRulesAlone proves a rule the file does not mention survives.
// Deleting an alert is destructive and must be asked for, never inferred.
func TestAlertApplierLeavesUnnamedRulesAlone(t *testing.T) {
	store := newMemAlerts()
	store.rules["manual"] = &alert.Rule{ID: "manual", OrgID: "org-1", Name: "made-in-console"}

	a := NewAlertApplier(store)
	if _, err := a.Apply(context.Background(), "org-1", []byte(alertFileOneRule)); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	rules, _ := store.ListByOrg(context.Background(), "org-1")
	if len(rules) != 2 {
		t.Fatalf("stored %d rules, want 2 (the manual one must survive)", len(rules))
	}
}

// TestAlertApplierScopesToTheOrganisation proves a rule of the same name in another org is
// not touched.
func TestAlertApplierScopesToTheOrganisation(t *testing.T) {
	store := newMemAlerts()
	store.rules["other"] = &alert.Rule{ID: "other", OrgID: "org-2", Name: "device-offline", Threshold: 42}

	a := NewAlertApplier(store)
	a.now = func() time.Time { return time.Unix(0, 0) }

	if _, err := a.Apply(context.Background(), "org-1", []byte(alertFileOneRule)); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if store.rules["other"].Threshold != 42 {
		t.Fatal("a rule in another organisation was modified")
	}
}

// TestAlertApplierClearsAWebhookURL proves a provisioned rule cannot keep a secret-bearing
// URL from a previous console edit. The file has no field for it, so it is cleared.
func TestAlertApplierClearsAWebhookURL(t *testing.T) {
	store := newMemAlerts()
	store.rules["existing"] = &alert.Rule{
		ID: "existing", OrgID: "org-1", Name: "device-offline",
		WebhookURL: "https://hooks.example.test/secret-token",
	}

	a := NewAlertApplier(store)
	if _, err := a.Apply(context.Background(), "org-1", []byte(alertFileOneRule)); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if store.rules["existing"].WebhookURL != "" {
		t.Fatalf("a webhook URL survived provisioning: %q", store.rules["existing"].WebhookURL)
	}
}

// TestAlertApplierRefusesABadFile proves a malformed file fails before any rule is written,
// so a broken file cannot half-apply.
func TestAlertApplierRefusesABadFile(t *testing.T) {
	store := newMemAlerts()
	a := NewAlertApplier(store)

	bad := []byte(`
[[rule]]
name = "r"
metric = "not_a_metric"
condition = "gt"
`)
	if _, err := a.Apply(context.Background(), "org-1", bad); err == nil {
		t.Fatal("a file naming an unknown metric was accepted")
	}

	if len(store.rules) != 0 {
		t.Fatalf("%d rules were written by a refused file", len(store.rules))
	}
}

// TestParseSeconds pins the duration reading.
func TestParseSeconds(t *testing.T) {
	cases := map[string]int{"": 0, "300": 300, "5m": 300, "90s": 90, "1h": 3600}
	for raw, want := range cases {
		got, err := parseSeconds(raw)
		if err != nil {
			t.Fatalf("%q: %v", raw, err)
		}
		if got != want {
			t.Fatalf("%q = %d, want %d", raw, got, want)
		}
	}

	for _, bad := range []string{"-5m", "abc"} {
		if _, err := parseSeconds(bad); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
}
