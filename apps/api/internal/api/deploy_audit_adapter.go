package api

import (
	"context"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/audit"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploy"
)

// deployAuditAdapter maps deploy audit events onto the audit logger.
//
// The deploy package must not import the audit package (it is application code, and the
// audit logger is infrastructure), so it declares its own small event. This adapter is the
// one place the two vocabularies are reconciled.
//
// It has two entry points because the two producers carry the same facts differently: the
// HTTP handler uses audit.DeployEvent directly (it already depends on the audit package for
// the action constants), while the service carries plain strings so it stays
// infrastructure-free. The string-to-constant cast is safe because the service spells the
// audit constants out, and a test pins the mapping.
type deployAuditAdapter struct {
	logger *audit.Logger
}

// newDeployAuditAdapter builds the adapter. A nil logger yields a nil adapter, so the
// caller can skip wiring rather than installing a sink that silently does nothing.
func newDeployAuditAdapter(logger *audit.Logger) *deployAuditAdapter {
	if logger == nil {
		return nil
	}

	return &deployAuditAdapter{logger: logger}
}

// Deploy records one handler-side deploy action.
func (a *deployAuditAdapter) Deploy(ctx context.Context, e audit.DeployEvent) {
	if a == nil || a.logger == nil {
		return
	}

	a.logger.Deploy(ctx, e)
}

// DeployFromService records one service-side deploy action (a rollback, or a device
// reporting a version applied).
func (a *deployAuditAdapter) DeployFromService(ctx context.Context, e deploy.DeployEvent) {
	if a == nil || a.logger == nil {
		return
	}

	a.logger.Deploy(ctx, audit.DeployEvent{
		UserID:   e.UserID,
		OrgID:    e.OrgID,
		DeviceID: e.DeviceID,
		Version:  e.Version,
		Action:   audit.Action(e.Action),
		Result:   audit.Result(e.Result),
		Detail:   e.Detail,
	})
}

// Compile-time proof the handler's sink accepts this adapter.
var _ DeployAuditSink = (*deployAuditAdapter)(nil)

// DeployAuditSink is the handler-side interface the adapter satisfies, matching the
// handler package's own narrow interface.
type DeployAuditSink interface {
	Deploy(ctx context.Context, e audit.DeployEvent)
}
