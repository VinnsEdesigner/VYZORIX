// Package subscription provides GraphQL subscription support via WebSocket.
package subscription

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/graphql/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/graphql/resolver"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/responses"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/config"
	hub "github.com/VinnsEdesigner/vyzorix/apps/api/internal/ws"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// AllowedOrigins returns origin validation function based on config.
func allowedOrigins(cfg config.Config) func(r *http.Request) bool {
	origins := cfg.AllowedOrigins
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // Allow empty origin (e.g., same-origin requests).
		}

		u, err := url.Parse(origin)
		if err != nil || u.Host == "" {
			return false
		}
		host := u.Hostname()

		for _, allowed := range origins {
			if allowed == "*" {
				return true
			}
			if strings.EqualFold(allowed, origin) {
				return true
			}
			// Wildcard subdomain. Accept both "*.example.com" and
			// "https://*.example.com": the config validator requires a scheme on every
			// entry, but the bare form is what the tests and older configs use. Matching
			// only the bare form would make the wildcard dead for any valid config, and
			// matching only the schemed form would break the bare form.
			wildcardDomain := strings.TrimPrefix(allowed, "*.")
			if parsed, parseErr := url.Parse(allowed); parseErr == nil && parsed.Scheme != "" {
				wildcardDomain = strings.TrimPrefix(parsed.Hostname(), "*.")
			}
			if strings.HasPrefix(allowed, "*.") || strings.Contains(allowed, "://*.") {
				if hostMatchesDomain(host, wildcardDomain) {
					return true
				}
			}
		}
		return false
	}
}

// hostMatchesDomain reports whether host is domain or a subdomain of it.
func hostMatchesDomain(host, domain string) bool {
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	host = strings.ToLower(host)
	return host == domain || strings.HasSuffix(host, "."+domain)
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     nil, // Set during initialization.
	// The browser client (graphql-ws) requests the "graphql-transport-ws"
	// subprotocol and rejects the handshake unless the server echoes it. Without
	// this the upgrade fails with "Sent non-empty 'Sec-WebSocket-Protocol' header
	// but no response was received", which surfaces in the client as a failed
	// connection rather than a protocol error.
	Subprotocols: []string{"graphql-transport-ws", "graphql-ws"},
}

// AuditLogger interface for audit logging.
type AuditLogger interface {
	LogAction(ctx context.Context, userID, action, resourceType, resourceID string)
}

// Config holds handler configuration.
type Config struct {
	Hub         *hub.Hub
	Resolver    *resolver.Resolver
	AuthMw      *middleware.AuthMiddleware
	Logger      Logger
	AuditLogger AuditLogger
	Config      config.Config
}

// Handler manages WebSocket connections for GraphQL subscriptions.
type Handler struct {
	hub       *hub.Hub
	resolver  *resolver.Resolver
	authMw    *middleware.AuthMiddleware
	presenter *Presenter
	clients   map[*websocket.Conn]*Client
	mu        sync.Mutex
}

// NewHandler creates a new subscription handler.
func NewHandler(cfg *Config) *Handler {
	// Set origin check function based on config.
	upgrader.CheckOrigin = allowedOrigins(cfg.Config)

	return &Handler{
		hub:       cfg.Hub,
		resolver:  cfg.Resolver,
		authMw:    cfg.AuthMw,
		presenter: NewPresenter(cfg.Logger, cfg.AuditLogger),
		clients:   make(map[*websocket.Conn]*Client),
	}
}

// removeClient removes a client from the handler's client map.
func (h *Handler) removeClient(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
}

// HandleWebSocket upgrades HTTP to WebSocket and handles subscription connections.
func (h *Handler) HandleWebSocket(c *gin.Context) {
	// Extract org from URL parameter.
	orgID := c.Param("org")
	if orgID == "" {
		responses.RespondStructured(c, http.StatusBadRequest, "organization ID required")
		return
	}

	// The system middleware chain (cookieAuth/tenantAPIKeyAuth) has already.
	// authenticated the user and set it in the gin context. Extract it.
	// from there instead of re-authenticating.
	op := h.authMw.GetUserFromGinContext(c)
	if op == nil {
		h.presenter.LogAuthFail(c.Request.Context(), nil)
		responses.RespondStructured(c, http.StatusUnauthorized, "unauthorized")

		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.presenter.LogWebSocketError(c.Request.Context(), "upgrade", err)
		return
	}

	client := NewClient(conn, op, orgID, h, h.presenter)

	h.mu.Lock()
	h.clients[conn] = client
	h.mu.Unlock()

	h.presenter.LogConnect(c.Request.Context(), op)

	go client.readPump()
	go client.writePump()
}
