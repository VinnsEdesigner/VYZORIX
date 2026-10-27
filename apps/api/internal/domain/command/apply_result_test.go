package command_test

import (
	"errors"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/command"
)

var resultAt = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

func deliveredCommand() *command.Command {
	delivered := resultAt.Add(-time.Minute).UnixMilli()

	return &command.Command{
		Status:      command.StatusDelivered,
		DispatchID:  "d1",
		DeviceID:    "dev-1",
		DeliveredAt: &delivered,
	}
}

func TestApplyResultSuccessRecordsOutputNotFailure(t *testing.T) {
	cmd := deliveredCommand()

	changed, err := cmd.ApplyResult(true, "hello world", resultAt)
	if err != nil {
		t.Fatalf("ApplyResult: %v", err)
	}
	if !changed {
		t.Fatal("a delivered command becoming completed must report a change")
	}
	if cmd.Status != command.StatusCompleted {
		t.Fatalf("status = %s, want completed", cmd.Status)
	}
	if cmd.Result != "hello world" {
		t.Fatalf("result = %q, want the device output", cmd.Result)
	}
	if cmd.FailureReason != "" {
		t.Fatalf("failure reason = %q, want empty for a successful command", cmd.FailureReason)
	}
	if cmd.CompletedAt == nil || *cmd.CompletedAt != resultAt.UnixMilli() {
		t.Fatalf("completed at = %v, want %d", cmd.CompletedAt, resultAt.UnixMilli())
	}
}

func TestApplyResultFailureRecordsReasonNotResult(t *testing.T) {
	cmd := deliveredCommand()

	changed, err := cmd.ApplyResult(false, "permission denied", resultAt)
	if err != nil {
		t.Fatalf("ApplyResult: %v", err)
	}
	if !changed {
		t.Fatal("a delivered command becoming failed must report a change")
	}
	if cmd.Status != command.StatusFailed {
		t.Fatalf("status = %s, want failed", cmd.Status)
	}
	if cmd.FailureReason != "permission denied" {
		t.Fatalf("failure reason = %q, want the device error", cmd.FailureReason)
	}
	if cmd.Result != "" {
		t.Fatalf("result = %q, want empty for a failed command", cmd.Result)
	}
}

func TestApplyResultIsIdempotentForRepeatedTerminalStatus(t *testing.T) {
	cmd := deliveredCommand()

	if _, err := cmd.ApplyResult(true, "first", resultAt); err != nil {
		t.Fatalf("first ApplyResult: %v", err)
	}

	// A device that re-sends its result (for example after a reconnect) must not
	// produce an error or a second transition.
	changed, err := cmd.ApplyResult(true, "second", resultAt.Add(time.Second))
	if err != nil {
		t.Fatalf("repeat ApplyResult: %v", err)
	}
	if changed {
		t.Fatal("re-applying the same terminal status must not report a change")
	}
	if cmd.Result != "first" {
		t.Fatalf("result = %q, want the first report to stand", cmd.Result)
	}
}

func TestApplyResultRejectsTransitionFromOtherTerminalState(t *testing.T) {
	cmd := deliveredCommand()

	if _, err := cmd.ApplyResult(true, "done", resultAt); err != nil {
		t.Fatalf("complete: %v", err)
	}

	// A late failure report for an already-completed command must be rejected
	// rather than flipping a finished command to failed.
	_, err := cmd.ApplyResult(false, "too late", resultAt.Add(time.Second))
	if !errors.Is(err, command.ErrInvalidCommandTransition) {
		t.Fatalf("err = %v, want ErrInvalidCommandTransition", err)
	}
	if cmd.Status != command.StatusCompleted {
		t.Fatalf("status = %s, want completed to be preserved", cmd.Status)
	}
}

func TestApplyResultAllowsPendingToTerminal(t *testing.T) {
	// A device may report a result without the server having observed delivery
	// (for example the delivery confirmation was lost). The lifecycle permits
	// pending -> completed, so this must succeed.
	cmd := &command.Command{Status: command.StatusPending, DispatchID: "d2", DeviceID: "dev-1"}

	changed, err := cmd.ApplyResult(true, "ok", resultAt)
	if err != nil {
		t.Fatalf("ApplyResult: %v", err)
	}
	if !changed || cmd.Status != command.StatusCompleted {
		t.Fatalf("status = %s changed = %v, want completed true", cmd.Status, changed)
	}
}

func TestApplyResultSuccessClearsEarlierFailureReason(t *testing.T) {
	cmd := deliveredCommand()
	cmd.FailureReason = "stale"

	if _, err := cmd.ApplyResult(true, "ok", resultAt); err != nil {
		t.Fatalf("ApplyResult: %v", err)
	}
	if cmd.FailureReason != "" {
		t.Fatalf("failure reason = %q, want it cleared on success", cmd.FailureReason)
	}
}
