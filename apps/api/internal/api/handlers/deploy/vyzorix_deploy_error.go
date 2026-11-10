package deployhandlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/responses"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/deploy"
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
	operators         OperatorLookup
	organizations     OrganizationLookup
	log               *slog.Logger
	pusher            ConfigPusher
}

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
	operators OperatorLookup,
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
		operators:         operators,
		organizations:     organizations,
		log:               log,
	}
}

// respondDeployError maps a deploy-package refusal to its HTTP response. The
// mapping lives in one place so a new refusal cannot ship without a status, and
// so the reason an operator sees is decided by the predicate rather than guessed
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

func respondBadBody(c *gin.Context) {
	responses.RespondStructured(c, http.StatusBadRequest, "could not read the request body")
}
