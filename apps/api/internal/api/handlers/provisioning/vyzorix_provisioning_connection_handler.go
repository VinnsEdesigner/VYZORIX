// Package provisioninghandler exposes the GitOps provisioning surface over HTTP.
package provisioninghandler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/adapters/response"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	svc "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/provisioning"
	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
	"github.com/VinnsEdesigner/vyzorix/apps/api/openapi"
)

// Compile-time reference for the swaggo-annotated DTO used in the delete route.
var _ openapi.SuccessResult

// ConnectionHandler serves the connection routes.
type ConnectionHandler struct {
	service   *svc.ConnectionService
	presenter *response.Presenter
}

// NewConnectionHandler builds a ConnectionHandler.
func NewConnectionHandler(service *svc.ConnectionService, presenter *response.Presenter) *ConnectionHandler {
	return &ConnectionHandler{service: service, presenter: presenter}
}

// connectionJSON is the wire shape. It has no token field: the secret is
// write-only and no response can carry it.
type connectionJSON struct {
	CheckedAt *int64 `json:"checked_at,omitempty"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Host      string `json:"host"`
	Status    string `json:"status"`
	LastError string `json:"last_error,omitempty"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
	// WriteEnabled reports whether the token has push scope. It is a capability flag,
	// not secret material, so it is safe to return and the UI needs it to offer editing.
	WriteEnabled bool `json:"write_enabled"`
}

type connectionListJSON struct {
	Connections []connectionJSON `json:"connections"`
	Total       int              `json:"total"`
}

type connectionRequest struct {
	// WriteEnabled declares the token has push scope. It is a pointer so a request that
	// only renames a connection cannot silently flip it either way.
	WriteEnabled *bool  `json:"write_enabled"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Host         string `json:"host"`
	Token        string `json:"token"`
}

func toConnectionJSON(c *domain.Connection) connectionJSON {
	out := connectionJSON{
		ID:           c.ID,
		Name:         c.Name,
		Type:         string(c.Type),
		Host:         c.Host,
		Status:       string(c.Status),
		LastError:    c.LastError,
		CreatedAt:    c.CreatedAt.UnixMilli(),
		UpdatedAt:    c.UpdatedAt.UnixMilli(),
		WriteEnabled: c.WriteEnabled,
	}
	if c.CheckedAt != nil {
		ms := c.CheckedAt.UnixMilli()
		out.CheckedAt = &ms
	}

	return out
}

// List handles GET /v1/provisioning/connections.
// @Summary      List provisioning connections
// @Description  Lists an organisation's stored git credentials. Token material is never returned.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Success      200  {object}  provisioninghandler.connectionListJSON  "connections"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.read required"
// @Router       /provisioning/connections [get]
func (h *ConnectionHandler) List(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	connections, err := h.service.List(c.Request.Context(), orgID)
	if err != nil {
		h.presenter.InternalError(c, "failed to list connections")

		return
	}

	out := make([]connectionJSON, len(connections))
	for i, conn := range connections {
		out[i] = toConnectionJSON(conn)
	}

	c.JSON(http.StatusOK, connectionListJSON{Connections: out, Total: len(out)})
}

// Get handles GET /v1/provisioning/connections/:id.
// @Summary      Get provisioning connection
// @Description  Returns one stored git credential. Token material is never returned.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id  path  string  true  "connection ID"
// @Success      200  {object}  provisioninghandler.connectionJSON  "connection"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.read required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Router       /provisioning/connections/{id} [get]
func (h *ConnectionHandler) Get(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	conn, err := h.service.Get(c.Request.Context(), orgID, c.Param("id"))
	if err != nil {
		h.writeConnectionError(c, err)

		return
	}

	c.JSON(http.StatusOK, toConnectionJSON(conn))
}

// Create handles POST /v1/provisioning/connections.
// @Summary      Create provisioning connection
// @Description  Stores a git credential, encrypts the token at rest, and runs a health check. A failed check still stores the connection, with status error and the reason.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        body  body  provisioninghandler.connectionRequest  true  "connection"
// @Success      201  {object}  provisioninghandler.connectionJSON  "created"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid input"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.write required"
// @Router       /provisioning/connections [post]
func (h *ConnectionHandler) Create(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	var req connectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.presenter.BadRequest(c, "invalid request body")

		return
	}

	conn, err := h.service.Create(c.Request.Context(), orgID, svc.ConnectionInput{
		Actor:        actorFrom(c),
		Name:         req.Name,
		Type:         domain.ConnectionType(req.Type),
		Host:         req.Host,
		Token:        redaction.NewSecret(req.Token),
		WriteEnabled: req.WriteEnabled,
	})
	if err != nil {
		h.writeConnectionError(c, err)

		return
	}

	c.JSON(http.StatusCreated, toConnectionJSON(conn))
}

// Update handles PATCH /v1/provisioning/connections/:id.
// @Summary      Update provisioning connection
// @Description  Renames, re-hosts, or rotates the token. An empty token keeps the stored one. A token or host change re-runs the health check.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id    path  string  true  "connection ID"
// @Param        body  body  provisioninghandler.connectionRequest  true  "fields to change"
// @Success      200  {object}  provisioninghandler.connectionJSON  "updated"
// @Failure      400  {object}  openapi.ErrorResponse  "invalid input"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.write required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Router       /provisioning/connections/{id} [patch]
func (h *ConnectionHandler) Update(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	var req connectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.presenter.BadRequest(c, "invalid request body")

		return
	}

	conn, err := h.service.Update(c.Request.Context(), orgID, c.Param("id"), svc.ConnectionInput{
		Actor:        actorFrom(c),
		Name:         req.Name,
		Type:         domain.ConnectionType(req.Type),
		Host:         req.Host,
		Token:        redaction.NewSecret(req.Token),
		WriteEnabled: req.WriteEnabled,
	})
	if err != nil {
		h.writeConnectionError(c, err)

		return
	}

	c.JSON(http.StatusOK, toConnectionJSON(conn))
}

// Delete handles DELETE /v1/provisioning/connections/:id.
// @Summary      Delete provisioning connection
// @Description  Removes a connection. Refused with 409 while a repository still uses it.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id  path  string  true  "connection ID"
// @Success      200  {object}  openapi.SuccessResult  "deleted"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.write required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Failure      409  {object}  openapi.ErrorResponse  "connection still in use"
// @Router       /provisioning/connections/{id} [delete]
func (h *ConnectionHandler) Delete(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	if err := h.service.Delete(c.Request.Context(), orgID, c.Param("id"), actorFrom(c)); err != nil {
		h.writeConnectionError(c, err)

		return
	}

	h.presenter.OK(c, openapi.SuccessResult{Success: true, Message: "connection deleted"})
}

// Check handles POST /v1/provisioning/connections/:id/check.
// @Summary      Check provisioning connection
// @Description  Re-runs the health check and records the result.
// @Tags         provisioning
// @Accept       json
// @Produce      json
// @Param        X-Organization-ID  header  string  true  "Organization ID"
// @Param        id  path  string  true  "connection ID"
// @Success      200  {object}  provisioninghandler.connectionJSON  "checked"
// @Failure      401  {object}  openapi.ErrorResponse  "authentication required"
// @Failure      403  {object}  openapi.ErrorResponse  "provisioning.write required"
// @Failure      404  {object}  openapi.ErrorResponse  "not found"
// @Router       /provisioning/connections/{id}/check [post]
func (h *ConnectionHandler) Check(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		h.presenter.BadRequest(c, "organization context required")

		return
	}

	conn, err := h.service.CheckNow(c.Request.Context(), orgID, c.Param("id"), actorFrom(c))
	if err != nil {
		h.writeConnectionError(c, err)

		return
	}

	c.JSON(http.StatusOK, toConnectionJSON(conn))
}

func (h *ConnectionHandler) writeConnectionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrConnectionNotFound):
		h.presenter.NotFound(c, "connection not found")
	case errors.Is(err, domain.ErrConnectionInUse):
		h.presenter.Conflict(c, err.Error())
	case errors.Is(err, svc.ErrInvalidConnection):
		h.presenter.BadRequest(c, err.Error())
	default:
		h.presenter.InternalError(c, "provisioning connection operation failed")
	}
}
