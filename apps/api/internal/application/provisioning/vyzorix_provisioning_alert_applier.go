package provisioning

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/alert"
	"github.com/google/uuid"
)

// AlertApplier writes an organisation's alert rules from an alerts.toml.
//
// Rules are matched by name within the organisation, because a name is what a person
// writes in the file and what they recognise in the console. An existing rule with the same
// name is updated in place; a rule the file does not name is left alone, because deleting
// an alert is a destructive act the operator must ask for.
type AlertApplier struct {
	rules alert.Repository
	now   func() time.Time
}

// NewAlertApplier builds the applier.
func NewAlertApplier(rules alert.Repository) *AlertApplier {
	return &AlertApplier{rules: rules, now: time.Now}
}

// ApplyResult reports what one org file did, so the job can record it per resource.
type ApplyResult struct {
	// Detail is a short summary for the job result.
	Detail string
	// Created and Updated count the objects the file wrote.
	Created int
	// Updated counts objects that already existed and were rewritten.
	Updated int
	// Drifted counts objects that differed from the declaration even though the file had not
	// changed since it was last applied. That is the signature of a console edit: git did not
	// move, but the stored object no longer matches what the file says, so the object was
	// changed outside provisioning.
	Drifted int
}

// Apply writes every rule in the file for the organisation and returns the counts.
func (a *AlertApplier) Apply(ctx context.Context, orgID string, data []byte) (*ApplyResult, error) {
	file, err := ParseAlertFile(data)
	if err != nil {
		return nil, err
	}

	existing, err := a.rules.ListByOrg(ctx, orgID)
	if err != nil {
		return nil, err
	}

	byName := make(map[string]*alert.Rule, len(existing))
	for _, r := range existing {
		byName[r.Name] = r
	}

	res := &ApplyResult{}

	for i := range file.Rules {
		spec := &file.Rules[i]

		rule, ok := byName[spec.Name]
		if ok {
			// A rule that exists but no longer matches the declaration was changed outside
			// provisioning — a console edit. That is drift, and it is counted before the
			// rewrite erases the evidence.
			drifted, cmpErr := a.differs(rule, spec)
			if cmpErr != nil {
				return nil, cmpErr
			}
			if drifted {
				res.Drifted++
			}

			res.Updated++
		} else {
			rule = &alert.Rule{ID: uuid.New().String(), OrgID: orgID, Name: spec.Name, CreatedAt: a.now()}
			res.Created++
		}

		if err := a.fill(rule, spec); err != nil {
			return nil, err
		}
		rule.UpdatedAt = a.now()

		if err := a.rules.Save(ctx, rule); err != nil {
			return nil, err
		}
	}

	res.Detail = fmt.Sprintf("%d rule(s) created, %d updated", res.Created, res.Updated)
	if res.Drifted > 0 {
		res.Detail += fmt.Sprintf(", %d had drifted", res.Drifted)
	}

	return res, nil
}

// differs reports whether a stored rule's declared fields no longer match the file.
//
// It compares only the fields the file controls, so a field provisioning does not manage
// (an id, a timestamp) cannot be mistaken for drift.
func (a *AlertApplier) differs(rule *alert.Rule, spec *AlertRule) (bool, error) {
	want := &alert.Rule{}
	if err := a.fill(want, spec); err != nil {
		return false, err
	}

	return rule.Metric != want.Metric ||
		rule.Condition != want.Condition ||
		rule.Threshold != want.Threshold ||
		rule.ForSeconds != want.ForSeconds ||
		rule.NotifyIntervalSeconds != want.NotifyIntervalSeconds ||
		rule.Enabled != want.Enabled ||
		rule.WebhookURL != want.WebhookURL, nil
}

// fill copies the declared fields onto a rule, parsing the two duration fields.
func (a *AlertApplier) fill(rule *alert.Rule, spec *AlertRule) error {
	forSeconds, err := parseSeconds(spec.For)
	if err != nil {
		return fmt.Errorf("rule %q: for: %w", spec.Name, err)
	}
	notifySeconds, err := parseSeconds(spec.Notify)
	if err != nil {
		return fmt.Errorf("rule %q: notify: %w", spec.Name, err)
	}

	rule.Metric = alert.Metric(spec.Metric)
	rule.Condition = alert.Condition(spec.Condition)
	rule.Threshold = spec.Threshold
	rule.ForSeconds = forSeconds
	rule.NotifyIntervalSeconds = notifySeconds
	rule.Enabled = spec.Enabled
	// A provisioned rule never carries a webhook URL: the field is not in the file, and a
	// rule written by git must not inherit a secret-bearing URL from a previous edit.
	rule.WebhookURL = ""

	return nil
}

// parseSeconds reads a duration like "5m" or "90s". Empty is zero.
func parseSeconds(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}

	// A bare number is seconds, which is the least surprising reading of "notify = 300".
	if n, err := strconv.Atoi(raw); err == nil {
		if n < 0 {
			return 0, errors.New("must not be negative")
		}

		return n, nil
	}

	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%q is not a duration", raw)
	}
	if d < 0 {
		return 0, errors.New("must not be negative")
	}

	return int(d.Seconds()), nil
}
