package deployhandlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/responses"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/audit"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploy"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/appsecret"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/deploycredential"
	apperrors "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/errors"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/configscan"
)

// Handler serves the deploy endpoints. Every field is a port the server wires;
// nothing here reaches a database directly.
type Handler struct {
	service           *deploy.DeployService
	secrets           *deploy.SecretService
	credentials       *deploy.CredentialService
	devices           deploy.DeviceLookup
	membershipChecker MembershipChecker
	grants            GrantLister
	users             UserLookup
	organizations     OrganizationLookup
	audit             DeployAudit
	log               *slog.Logger
	pusher            ConfigPusher
}

// DeployAudit records a deploy action in the audit trail. It is a narrow interface over
// *audit.Logger so this package does not depend on the audit package, and so a test can
// assert exactly which events a request produced.
//
// A deploy decides what code a device runs, so it is audited with the same weight as a
// command: who asked, for which device and organisation, at which version, and whether it
// succeeded.
type DeployAudit interface {
	Deploy(ctx context.Context, e audit.DeployEvent)
}

// SetAudit injects the audit trail. It is a setter because the audit logger is built
// alongside the database, and a handler wired without one simply records nothing rather
// than panicking on the first deploy.
func (h *Handler) SetAudit(a DeployAudit) { h.audit = a }

// ConfigPusher delivers a freshly deployed config to a device that is connected
// right now. The push is an optimisation over the device's own re-fetch
// (GET /v1/device/{imei}/config, contract): a device that is offline when it
// is deployed to simply re-fetches on reconnect, so a push that finds no
// connection is not an error.
//
// It is an interface, set after the handler is built, because the pusher is the
// onboarding handler and the deploy handler is wired first. *inbox.EnrollHandler
// implements it.
type ConfigPusher interface {
	PushConfig(ctx context.Context, deviceID string) bool
}

// SetConfigPusher wires the pusher used to notify a connected device of a new
// deploy. It may be called on a live handler.
func (h *Handler) SetConfigPusher(pusher ConfigPusher) {
	h.pusher = pusher
}

// NewHandler builds a deploy Handler.
func NewHandler(
	service *deploy.DeployService,
	secrets *deploy.SecretService,
	credentials *deploy.CredentialService,
	devices deploy.DeviceLookup,
	membershipChecker MembershipChecker,
	grants GrantLister,
	users UserLookup,
	organizations OrganizationLookup,
	log *slog.Logger,
) *Handler {
	return &Handler{
		service:           service,
		secrets:           secrets,
		credentials:       credentials,
		devices:           devices,
		membershipChecker: membershipChecker,
		grants:            grants,
		users:             users,
		organizations:     organizations,
		log:               log,
	}
}

// respondDeployError maps a deploy-package refusal to its HTTP response. The
// mapping lives in one place so a new refusal cannot ship without a status, and
// so the reason a user sees is decided by the predicate rather than guessed
// from an error string.
func (h *Handler) respondDeployError(c *gin.Context, err error) {
	var denied *deploy.DeniedError
	if errors.As(err, &denied) {
		switch denied.Denial {
		case deploy.DenialUnauthenticated:
			responses.RespondWithErrorCode(c, apperrors.CodeAuthTokenInvalid, "authentication required")
		case deploy.DenialNotInOrg:
			// 404, never 403: a 403 would confirm the device exists in another org.
			responses.RespondNotFoundError(c, "device")
		case deploy.DenialNotRegistered:
			responses.RespondConflictError(c, "device is not registered")
		default:
			responses.RespondForbiddenError(c, "device.deploy is required for this device")
		}

		return
	}

	var manifestErr *deploy.ManifestError
	if errors.As(err, &manifestErr) {
		responses.RespondValidationError(c, []apperrors.ValidationDetail{
			apperrors.NewValidationDetail(manifestErr.Field, manifestErr.Message),
		})

		return
	}

	var scanErr *configscan.Error
	if errors.As(err, &scanErr) {
		responses.RespondValidationError(c, []apperrors.ValidationDetail{
			apperrors.NewValidationDetail(scanErr.Key, scanErr.Reason),
		})

		return
	}

	// A config that names a secret the device has not been given is a 404: the
	// request is well-formed, the value simply is not there.
	if errors.Is(err, deploy.ErrSecretNotSet) {
		responses.RespondStructured(c, http.StatusNotFound, err.Error())

		return
	}

	var secretErr *deploy.SecretError
	if errors.As(err, &secretErr) {
		responses.RespondStructured(c, http.StatusBadRequest, secretErr.Error())

		return
	}

	var credErr *deploy.CredentialError
	if errors.As(err, &credErr) {
		responses.RespondStructured(c, http.StatusBadRequest, credErr.Error())

		return
	}

	if errors.Is(err, deploy.ErrNoConfig) {
		responses.RespondNotFoundError(c, "deploy config")

		return
	}

	// The device exists and the request is valid, but the device has no secret key, so a value
	// cannot be wrapped to it. That is a state the caller can fix (register the device's key),
	// not a server fault — a 500 here told the user the server had broken.
	if errors.Is(err, deploy.ErrNoSecretKey) {
		responses.RespondConflictError(c, "this device has no secret key registered, so it cannot receive secrets")

		return
	}

	// A deploy is already in flight for this device. The caller can retry; proceeding
	// would produce the version collision the per-device lock exists to prevent.
	// A revoke that matched no live credential is a 404, not a 204: answering "done" for
	// a credential that is not there tells the operator a token was killed when it was not.
	// A secret unset that matched nothing is a 404 for the same reason a revoke is:
	// answering "done" for a key that was never set tells the operator something was
	// cleared when nothing was.
	if errors.Is(err, appsecret.ErrNotFound) {
		responses.RespondNotFoundError(c, "app secret")

		return
	}

	if errors.Is(err, deploycredential.ErrNotFound) {
		responses.RespondNotFoundError(c, "deploy credential")

		return
	}

	if errors.Is(err, deploy.ErrDeployInProgress) {
		responses.RespondConflictError(c, err.Error())

		return
	}

	// A missing or non-signing signer is a server fault, not a caller one: the
	// contract is that a config is never served unsigned, so this must be a 500
	// with the cause in the log, never a silently unsigned document.
	if errors.Is(err, deploy.ErrNoSigner) || errors.Is(err, deploy.ErrNotSigned) {
		h.logError(c, err)
		responses.RespondInternalError(c)

		return
	}

	h.logError(c, err)
	responses.RespondInternalError(c)
}

// auditEvent assembles the common fields of a deploy audit entry from the request.
//
// It reads the actor from the same place the caller resolution does, so a credential
// deploy is attributed to the credential's author and a session deploy to the user — the
// audit row names a human in both cases. The trace id joins the row to the access log.
func (h *Handler) auditEvent(c *gin.Context, orgID, deviceID string, action audit.Action, result audit.Result, detail string) audit.DeployEvent {
	e := audit.DeployEvent{
		OrgID:     orgID,
		DeviceID:  deviceID,
		Action:    action,
		Result:    result,
		Detail:    detail,
		IPAddress: c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
		TraceID:   responses.GetTraceID(c),
	}

	if identity, ok := middleware.DeployCredentialFromContext(c); ok {
		e.UserID = identity.CreatedBy
		e.ActorType = "deploy_credential"
	} else if op := middleware.GetUserFromContext(c); op != nil {
		e.UserID = op.ID
		e.ActorEmail = op.Email
		e.ActorType = "user"
	}

	return e
}

// recordAudit writes a deploy audit entry when an audit sink is wired.
func (h *Handler) recordAudit(c *gin.Context, e audit.DeployEvent) {
	if h.audit == nil {
		return
	}

	h.audit.Deploy(c.Request.Context(), e)
}

// logError records a server-side failure with its trace id. The response body
// stays generic; the log carries the cause.
func (h *Handler) logError(c *gin.Context, err error) {
	if h.log == nil {
		return
	}

	h.log.Error("deploy request failed",
		slog.String("err", err.Error()),
		slog.String("trace_id", responses.GetTraceID(c)),
		slog.String("path", c.FullPath()),
	)
}

// The three request-level refusals every deploy handler shares. They are
// separate helpers because the same three answers recur on each endpoint, and a
// handler that invents its own status for one of them is the drift this prevents.

func respondMissingOrg(c *gin.Context) {
	responses.RespondStructured(c, http.StatusBadRequest, "organization context required")
}

func respondNotAMember(c *gin.Context) {
	responses.RespondForbiddenError(c, "not a member of this organization")
}

// respondBadQuery refuses a query parameter that is present but unrecognised. It is
// distinct from a bad body: the request read fine, the caller asked for something the
// server does not understand, and saying so points at the right thing to fix.
func respondBadQuery(c *gin.Context, name string) {
	responses.RespondStructured(c, http.StatusBadRequest, name+" must be true or false")
}

func respondBadBody(c *gin.Context) {
	responses.RespondStructured(c, http.StatusBadRequest, "could not read the request body")
}
