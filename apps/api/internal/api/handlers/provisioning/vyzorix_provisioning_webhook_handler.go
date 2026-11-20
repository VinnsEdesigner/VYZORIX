package provisioninghandler

import (
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/adapters/response"
	svc "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/provisioning"
)

// maxWebhookBody bounds a webhook body. A push payload is large — GitHub sends the commit
// list, GitLab sends the whole push — but it is still bounded, and an unbounded read on an
// unauthenticated endpoint is a memory-exhaustion vector.
const maxWebhookBody = 1 << 20 // 1 MiB

// WebhookHandler receives a provider's push and triggers a sync.
//
// It is the only unauthenticated provisioning route. Safety rests on the signature: the
// repository's secret is what proves the sender, and without it anyone could trigger a
// sync — cheap, but a way to hammer the git provider and the deploy path.
type WebhookHandler struct {
	receiver  *svc.WebhookReceiver
	presenter *response.Presenter
}

// NewWebhookHandler builds a WebhookHandler.
func NewWebhookHandler(receiver *svc.WebhookReceiver, presenter *response.Presenter) *WebhookHandler {
	return &WebhookHandler{receiver: receiver, presenter: presenter}
}

// Receive handles POST /v1/provisioning/webhooks/:repoID.
// @Summary      Receive a git provider push webhook
// @Description  Verifies the provider's HMAC signature against the repository's secret and triggers a sync. A push to a branch this repository does not track is acknowledged without syncing.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        repoID  path  string  true  "repository ID"
// @Param        X-Hub-Signature-256  header  string  false  "GitHub-style HMAC-SHA256 signature"
// @Param        X-Gitlab-Token       header  string  false  "GitLab shared secret"
// @Success      202  {object}  object  "accepted"
// @Failure      401  {object}  openapi.ErrorResponse  "signature missing or invalid"
// @Failure      404  {object}  openapi.ErrorResponse  "unknown repository"
// @Failure      413  {object}  openapi.ErrorResponse  "body too large"
// @Router       /provisioning/webhooks/{repoID} [post]
func (h *WebhookHandler) Receive(c *gin.Context) {
	// Read the body ONCE, as bytes. A provider signs what it sent, so the signature must
	// be checked against the exact bytes — re-encoding a parsed body would break it.
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxWebhookBody+1))
	if err != nil {
		h.presenter.BadRequest(c, "could not read the request body")

		return
	}
	if len(body) > maxWebhookBody {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "webhook body too large"})

		return
	}

	provider := svc.ProviderFromHeader(c.Request.Header)

	result, err := h.receiver.Handle(c.Request.Context(), c.Param("repoID"), provider, c.Request.Header, body)
	if err != nil {
		h.writeWebhookError(c, err)

		return
	}

	// 202: the push is accepted. A sync may have run, or may be left for the interval if
	// one was already in progress. Either way the provider is told to stop retrying.
	c.JSON(http.StatusAccepted, gin.H{
		"accepted":      true,
		"repository_id": result.RepositoryID,
		"branch":        result.Branch,
		"synced":        result.Triggered,
	})
}

// writeWebhookError maps a webhook failure to its response.
//
// Every verification failure answers with one fixed sentence. The internal error is
// specific — it distinguishes a missing header from a wrong secret from an unconfigured
// repository — and that specificity is exactly what must not reach an unauthenticated
// caller: "no secret is configured for this repository" would confirm the repository
// exists. The reason stays in the error for the caller's log; the response is constant.
func (h *WebhookHandler) writeWebhookError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, svc.ErrSignatureMissing),
		errors.Is(err, svc.ErrSignatureInvalid),
		errors.Is(err, svc.ErrSignatureStale),
		errors.Is(err, svc.ErrWebhookUnknownRepository):
		h.presenter.Unauthorized(c, "webhook signature could not be verified")
	default:
		h.presenter.InternalError(c, "webhook processing failed")
	}
}
