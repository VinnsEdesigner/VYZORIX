// Package websocket provides WebSocket handler implementations.
package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/audit"
	apperrors "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/errors"
	hub "github.com/VinnsEdesigner/vyzorix/apps/api/internal/ws"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// PTY user stream limits.
const (
	// ptyUserWriteTimeout bounds a single write to the user.
	ptyUserWriteTimeout = 10 * time.Second
	// ptyUserPongWait is how long the user may go silent before the
	// connection is considered dead.
	ptyUserPongWait = 70 * time.Second
	// ptyUserPingPeriod is the keepalive interval.
	ptyUserPingPeriod = 30 * time.Second
	// ptyUserIdleTimeout closes a session whose user has sent nothing for
	// this long. The registry TTL (2h) bounds a session's absolute life; this is
	// tighter and input-driven, so a tab left open does not hold a shell: a
	// terminal with no keystrokes is a terminal nobody is using.
	ptyUserIdleTimeout = 15 * time.Minute
	// ptyUserIdleCheck is how often the idle watchdog runs.
	ptyUserIdleCheck = time.Minute
	// ptyUserReadLimit caps an inbound frame. User input is keystrokes
	// and control messages; a paste is the realistic upper bound.
	ptyUserReadLimit = 1 << 20
	// ptyUserOutbox is the user's outbound queue depth. Output that
	// exceeds it is dropped and its credit returned, so a slow browser cannot
	// grow server memory.
	ptyUserOutbox = 256
	// ptyUserSendWait bounds how long delivery will block before dropping.
	ptyUserSendWait = 5 * time.Second
)

// PTYDeviceResolver resolves a device ID to the organization that owns it, so a
// session can be scoped to the user's org. It mirrors the check the command
// path performs before dispatch.
type PTYDeviceResolver interface {
	// DeviceOrganization returns the org ID for a device, or "" when unknown.
	DeviceOrganization(ctx context.Context, deviceID string) string
}

// PTYAudit records terminal session lifecycle events. *audit.Logger implements
// it; the interface keeps the handler independent of the concrete logger (and
// lets a no-op stand in where no audit store is configured).
type PTYAudit interface {
	PTYSession(ctx context.Context, e audit.PTYSessionEvent)
}

// PTYCapabilityResolver resolves the capability a session carries to the device:
// the API allowlist the server decided, the device's tier, and an expiry. It is
// the boundary contract's resolution, invoked at session open so the decision
// is made once rather than per call.
//
// The device's cloud WebSocket (Build 4) is what will deliver this to the device.
// Until it exists the resolved capability is recorded on the session and audited,
// so the decision is made and observable now and the delivery is the only piece
// left to add.
type PTYCapabilityResolver interface {
	ResolveForSession(ctx context.Context, userID, orgID, deviceID string) (SessionCapabilityInfo, bool)
}

// SessionCapabilityInfo is the resolved capability, kept as plain data so this
// package does not import the permission domain.
//
// It is both the decision (what the device may call) and the identity (who the
// caller is), because a terminal session needs both: the device enforces the
// allowlist, and the Vyzorix CLI shows the user who they are. They are
// resolved together, at open, so the two cannot disagree.
type SessionCapabilityInfo struct {
	// Identity — shown by the CLI's `whoami`.
	UserID   string
	UserName string
	Email        string
	OrgID        string
	OrgName      string
	// Role is the machine value (super_admin, admin, user, viewer); RoleLabel
	// is its human name ("Super admin"). Both, so a client need not map.
	Role      string
	RoleLabel string

	// Capability — enforced by the device.
	APIAllowlist []string
	Tier         int
	ExpiresAt    int64
}

// PTYAuthRequest carries the actor context a terminal-open decision needs. It
// mirrors the command path's authorization inputs so the same risk gate applies
// to a shell as to any other critical operation.
type PTYAuthRequest struct {
	UserID        string
	OrgID             string
	DeviceID          string
	ConfirmationToken string
	MFAVerified       bool
	IsSuperAdmin      bool
}

// PTYAuthDecision is the outcome of the terminal-open gate.
type PTYAuthDecision struct {
	// Reason is the audit reason for a denial.
	Reason string
	// Message is the client-facing explanation.
	Message string
	// Allowed reports whether the session may open.
	Allowed bool
	// NeedsConfirmation is true when the block is a confirmation/MFA gate the
	// client can satisfy and retry, as opposed to a hard deny.
	NeedsConfirmation bool
}

// PTYAuthorizer gates opening a terminal. It is the PTY_SESSION_OPEN risk gate:
// a shell is the most dangerous capability on the device, so it must not be
// reachable without the same authorization the command path applies. The
// concrete implementation delegates to the shared command Authorizer, so the
// critical tier registered for PTY_SESSION_OPEN is actually consulted rather
// than being dead policy.
type PTYAuthorizer interface {
	// AuthorizePTYOpen evaluates the request against the PTY_SESSION_OPEN risk
	// profile.
	AuthorizePTYOpen(ctx context.Context, req PTYAuthRequest) PTYAuthDecision
}

// ptyUserClient is one browser terminal connection.
//
//nolint:govet // fieldalignment: grouping by role reads better than packing.
type ptyUserClient struct {
	conn          *websocket.Conn
	log           *slog.Logger
	sessions      map[string]bool
	outbox        chan []byte
	closed        chan struct{}
	user      string
	orgID         string
	lastInput     time.Time
	closeOnce     sync.Once
	wantRecording bool
	mu            sync.Mutex
}

// UserTerminalHandler serves the browser terminal endpoint.
//
// Unlike the device stream, this endpoint speaks the same binary PTY framing as
// the device side, with JSON only for control messages (open, resize, close).
type UserTerminalHandler struct {
	relay        *hub.PTYRelay
	log          *slog.Logger
	clients      map[string]*ptyUserClient
	devices      PTYDeviceResolver
	authorizer   PTYAuthorizer
	capabilities PTYCapabilityResolver
	audit        PTYAudit
	upgrader     websocket.Upgrader
	recordingDir string
	mu           sync.RWMutex
}

// NewUserTerminalHandler creates the user terminal handler.
func NewUserTerminalHandler(
	relay *hub.PTYRelay,
	devices PTYDeviceResolver,
	authorizer PTYAuthorizer,
	capabilities PTYCapabilityResolver,
	auditLogger PTYAudit,
	recordingDir string,
	allowedOrigins []string,
	log *slog.Logger,
) *UserTerminalHandler {
	if log == nil {
		log = slog.Default()
	}

	checkOrigin := func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		for _, allowed := range allowedOrigins {
			if allowed == "*" || allowed == origin {
				return true
			}
		}

		return false
	}

	return &UserTerminalHandler{
		relay:        relay,
		devices:      devices,
		authorizer:   authorizer,
		capabilities: capabilities,
		audit:        auditLogger,
		recordingDir: recordingDir,
		log:          log,
		upgrader:     websocket.Upgrader{CheckOrigin: checkOrigin, HandshakeTimeout: 10 * time.Second},
		clients:      make(map[string]*ptyUserClient),
	}
}

// Handle handles GET /v1/device/:imei/terminal.
//
// The session cookie is validated by the normal middleware chain before this
// runs, so the user is already in the context.
func (h *UserTerminalHandler) Handle(c *gin.Context) {
	deviceID := c.Param("imei")
	if deviceID == "" {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "device imei required"))

		return
	}

	op := middleware.GetUserFromContext(c)
	if op == nil {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthTokenInvalid, "authentication required"))

		return
	}

	orgID := middleware.GetOrganizationID(c)
	if orgID == "" {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeValidationFailed, "organization context required"))

		return
	}

	// Verify the device belongs to the user's organization before upgrading,
	// so a user cannot open a shell on another org's device.
	if h.devices != nil && h.devices.DeviceOrganization(c.Request.Context(), deviceID) != orgID {
		_ = c.Error(apperrors.NewServerError(apperrors.CodeResourceNotFound, "device not found in organization"))

		return
	}

	// The risk gate for a shell session, evaluated before the upgrade so the
	// client gets a normal HTTP error rather than a socket that closes
	// immediately. This consults the PTY_SESSION_OPEN risk profile, so a shell
	// gets the same critical-tier treatment as any other dangerous operation:
	// an MFA-verified session plus a single-use confirmation token.
	if h.authorizer != nil {
		decision := h.authorizer.AuthorizePTYOpen(c.Request.Context(), PTYAuthRequest{
			UserID:        op.ID,
			OrgID:             orgID,
			DeviceID:          deviceID,
			ConfirmationToken: c.Query("confirmation_token"),
			MFAVerified:       sessionMFAVerified(c),
			IsSuperAdmin:      op.IsSuperAdmin(),
		})
		if !decision.Allowed {
			h.log.Warn("pty open denied",
				"userId", op.ID, "deviceId", deviceID, "reason", decision.Reason)
			h.auditSession(c.Request.Context(), audit.PTYSessionEvent{
				DeviceID:   deviceID,
				UserID: op.ID,
				Reason:     decision.Reason,
				IPAddress:  c.ClientIP(),
				UserAgent:  c.Request.UserAgent(),
				Result:     audit.ResultBlocked,
				Opened:     true,
			})

			// A confirmation gate is satisfiable: report it as such so the client
			// knows to obtain a token and retry, rather than treating it as a
			// permanent denial.
			if decision.NeedsConfirmation {
				_ = c.Error(apperrors.NewServerError(apperrors.CodeSecurityRiskUnconfirmed, decision.Message))

				return
			}

			_ = c.Error(apperrors.NewServerError(apperrors.CodeAuthzScopeForbidden, decision.Message))

			return
		}
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.log.Warn("pty terminal upgrade failed", "userId", op.ID, "deviceId", deviceID, "err", err)

		return
	}

	client := &ptyUserClient{
		conn:      conn,
		log:       h.log,
		user:  op.ID,
		orgID:     orgID,
		sessions:  make(map[string]bool),
		outbox:    make(chan []byte, ptyUserOutbox),
		lastInput: time.Now(),
		closed:    make(chan struct{}),
	}

	h.registerClient(client)

	go client.writePump()
	go client.idleWatchdog(h)

	client.readPump(h, deviceID)
}

// idleWatchdog closes this user's sessions once no input has arrived for
// ptyUserIdleTimeout. The socket itself is left alone: a client may still
// send pty_open to start a fresh session. Without this a forgotten tab holds a
// shell open until the registry's much longer absolute TTL.
func (c *ptyUserClient) idleWatchdog(h *UserTerminalHandler) {
	ticker := time.NewTicker(ptyUserIdleCheck)
	defer ticker.Stop()

	for {
		select {
		case <-c.done():
			return
		case <-ticker.C:
			c.closeIdleSessions(h)
		}
	}
}

// closeIdleSessions ends sessions the user has not typed into for too long.
func (c *ptyUserClient) closeIdleSessions(h *UserTerminalHandler) {
	c.mu.Lock()
	idleFor := time.Since(c.lastInput)
	owned := make([]string, 0, len(c.sessions))
	for id := range c.sessions {
		owned = append(owned, id)
	}
	c.mu.Unlock()

	if idleFor < ptyUserIdleTimeout {
		return
	}

	for _, id := range owned {
		h.log.Info("closing idle terminal session",
			"sessionId", id, "userId", c.user, "idleFor", idleFor.String())
		h.relay.CloseSession(id, "idle_timeout", c)

		c.mu.Lock()
		delete(c.sessions, id)
		c.mu.Unlock()
	}
}

// done reports whether the client's connection has been torn down.
func (c *ptyUserClient) done() <-chan struct{} {
	return c.closed
}

// touchInput records user activity, deferring the idle timeout.
func (c *ptyUserClient) touchInput() {
	c.mu.Lock()
	c.lastInput = time.Now()
	c.mu.Unlock()
}

// UserFor returns the user endpoint for a session, or nil when the
// user has disconnected. It implements PTYUserLookup.
func (h *UserTerminalHandler) UserFor(sessionID string) hub.PTYUser {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, c := range h.clients {
		c.mu.Lock()
		owned := c.sessions[sessionID]
		c.mu.Unlock()

		if owned {
			return c
		}
	}

	return nil
}

// DeliverOutput implements hub.PTYUser for the client that owns a session.
func (c *ptyUserClient) DeliverOutput(sessionID string, payload []byte) bool {
	// Frame the output for the browser: same binary format as the device side,
	// so the front end has one decoder.
	frame := hub.EncodePTYFrame(hub.ChannelPTY, hub.PTYTypeData, hub.EncodeSessionData(sessionID, payload))

	select {
	case c.outbox <- frame:
		return true
	case <-time.After(ptyUserSendWait):
		c.log.Debug("pty output dropped (operator too slow)", "sessionId", sessionID)

		return false
	}
}

// NotifyClosed implements hub.PTYUser.
func (c *ptyUserClient) NotifyClosed(sessionID, reason string) {
	msg, err := json.Marshal(map[string]string{
		"type":      "pty_closed",
		"sessionId": sessionID,
		"reason":    reason,
	})
	if err != nil {
		return
	}

	select {
	case c.outbox <- msg:
	default:
	}
}

// readPump handles user input: control messages (JSON) and keystrokes
// (binary). It returns when the connection closes.
func (c *ptyUserClient) readPump(h *UserTerminalHandler, deviceID string) {
	defer func() {
		h.removeClient(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(ptyUserReadLimit)
	_ = c.conn.SetReadDeadline(time.Now().Add(ptyUserPongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(ptyUserPongWait))

		return nil
	})

	for {
		msgType, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		// Any traffic counts as activity, so the idle watchdog only fires on a
		// genuinely forgotten tab.
		c.touchInput()

		switch msgType {
		case websocket.BinaryMessage:
			c.handleBinaryInput(h, raw)
		case websocket.TextMessage:
			c.handleControl(h, deviceID, raw)
		}
	}
}

// handleBinaryInput forwards keystrokes to the device.
func (c *ptyUserClient) handleBinaryInput(h *UserTerminalHandler, raw []byte) {
	_, _, payload, err := hub.DecodePTYFrame(raw)
	if err != nil {
		return
	}

	sessionID, data, err := hub.DecodeSessionData(payload)
	if err != nil {
		return
	}

	if !c.ownsSession(sessionID) {
		return
	}

	h.relay.RelayInput(sessionID, data)
}

// handleControl processes a JSON control message. A malformed or unknown
// message is logged and ignored rather than closing the connection: a bad
// control frame must not tear down a working terminal.
func (c *ptyUserClient) handleControl(h *UserTerminalHandler, deviceID string, raw []byte) {
	// Field order groups the three strings, then the scalars, then the bool, to
	// avoid padding.
	var msg struct {
		Type      string `json:"type"`
		SessionID string `json:"sessionId"`
		Reason    string `json:"reason"`
		Credits   int    `json:"credits"`
		Rows      uint16 `json:"rows"`
		Cols      uint16 `json:"cols"`
		Record    bool   `json:"record"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		c.log.Warn("bad pty control frame", "userId", c.user, "err", err)

		return
	}

	switch msg.Type {
	case "pty_open":
		c.wantRecording = msg.Record
		c.handleOpen(h, deviceID, msg.SessionID)
	case "pty_resize":
		c.handleResize(h, msg.SessionID, msg.Rows, msg.Cols)
	case "pty_ack":
		if c.ownsSession(msg.SessionID) {
			h.relay.AckCredit(msg.SessionID, msg.Credits)
		}
	case "pty_close":
		if c.ownsSession(msg.SessionID) {
			h.relay.CloseSession(msg.SessionID, "user_closed", c)
		}
	default:
		c.log.Debug("unknown pty control type", "userId", c.user, "type", msg.Type)
	}
}

// handleOpen registers a new session and tells the device to start a shell.
func (c *ptyUserClient) handleOpen(h *UserTerminalHandler, deviceID, sessionID string) {
	if sessionID == "" {
		c.sendError("", "session id required")

		return
	}

	// Recording is opt-in per session and only available when a directory is
	// configured. The user's request is honoured only if both hold.
	record := c.wantRecording && h.recordingDir != ""
	session, err := h.relay.Registry().Open(sessionID, deviceID, c.user, c.orgID, record)
	// Open returns a nil session on every error path (too many sessions, a duplicate
	// id, a bad id). Dereferencing it before this check panicked the handler — a
	// client retrying an open with the same session id crashed the request instead of
	// being told the id was taken.
	if err != nil {
		c.sendError(sessionID, err.Error())

		return
	}

	// Resolve the session's capability before the device is told to open a
	// shell. This is the boundary contract's decision — role tags intersected
	// with the device tier and the scoped grant — made once, here, rather than
	// re-derived per call. A resolution that yields an empty allowlist is a
	// refusal: the session would be able to call nothing, so it is not opened.
	identity := hub.SessionIdentity{}
	if h.capabilities != nil {
		capability, ok := h.capabilities.ResolveForSession(
			context.Background(), c.user, c.orgID, session.DeviceID)
		if !ok || len(capability.APIAllowlist) == 0 {
			_, _ = h.relay.Registry().Close(sessionID)
			h.auditSession(context.Background(), audit.PTYSessionEvent{
				SessionID:  sessionID,
				DeviceID:   session.DeviceID,
				UserID: c.user,
				Reason:     "no capability resolved for session",
				Result:     audit.ResultBlocked,
				Opened:     true,
			})
			c.sendError(sessionID, "no capability resolved for this session")

			return
		}

		session.SetCapability(capability.APIAllowlist, capability.Tier, capability.ExpiresAt)
		identity = hub.SessionIdentity{
			User:  capability.UserID,
			Name:      capability.UserName,
			Email:     capability.Email,
			Org:       capability.OrgID,
			OrgName:   capability.OrgName,
			Role:      capability.Role,
			RoleLabel: capability.RoleLabel,
			Tier:      capability.Tier,
			ExpiresAt: capability.ExpiresAt,
			Allowlist: capability.APIAllowlist,
		}
	}

	// Ask the device to allocate the PTY. The open frame carries the session ID
	// and window size so the device can size the pty from the first moment.
	openPayload := hub.EncodeSessionOpen(sessionID, 24, 80, identity)
	if !h.relay.SendToDevice(session.DeviceID, hub.PTYTypeOpen, openPayload) {
		// Device offline: roll the session back so the ID is not left dangling.
		_, _ = h.relay.Registry().Close(sessionID)
		c.sendError(sessionID, "device is offline")

		return
	}

	c.mu.Lock()
	c.sessions[sessionID] = true
	c.mu.Unlock()

	h.auditSession(context.Background(), audit.PTYSessionEvent{
		SessionID:  sessionID,
		DeviceID:   session.DeviceID,
		UserID: c.user,
		Result:     audit.ResultSuccess,
		Opened:     true,
		Recording:  session.Recording,
	})

	c.sendJSON(map[string]string{"type": "pty_opened", "sessionId": sessionID})
}

// auditSession records a terminal session event when an audit logger is wired.
func (h *UserTerminalHandler) auditSession(ctx context.Context, e audit.PTYSessionEvent) {
	if h.audit != nil {
		h.audit.PTYSession(ctx, e)
	}
}

// handleResize forwards a window size change to the device.
func (c *ptyUserClient) handleResize(h *UserTerminalHandler, sessionID string, rows, cols uint16) {
	if !c.ownsSession(sessionID) {
		return
	}

	payload := hub.EncodeSessionData(sessionID, hub.EncodePTYResize(rows, cols))
	h.relay.SendToDevice("", hub.PTYTypeResize, payload) // device resolved by session
}

// ownsSession reports whether this client owns a session.
func (c *ptyUserClient) ownsSession(sessionID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.sessions[sessionID]
}

func (c *ptyUserClient) sendError(sessionID, message string) {
	c.sendJSON(map[string]string{"type": "pty_error", "sessionId": sessionID, "error": message})
}

func (c *ptyUserClient) sendJSON(v interface{}) {
	raw, err := json.Marshal(v)
	if err != nil {
		return
	}

	select {
	case c.outbox <- raw:
	default:
	}
}

// writePump drains the outbox to the browser.
func (c *ptyUserClient) writePump() {
	ticker := time.NewTicker(ptyUserPingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case frame, ok := <-c.outbox:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})

				return
			}

			_ = c.conn.SetWriteDeadline(time.Now().Add(ptyUserWriteTimeout))

			// Control messages are JSON text; PTY data is binary. The browser
			// distinguishes them by the WebSocket opcode.
			msgType := websocket.BinaryMessage
			if len(frame) > 0 && frame[0] == '{' {
				msgType = websocket.TextMessage
			}

			if err := c.conn.WriteMessage(msgType, frame); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(ptyUserWriteTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// registerClient tracks a client so UserFor can find it.
func (h *UserTerminalHandler) registerClient(c *ptyUserClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[c.user+"#"+c.conn.RemoteAddr().String()] = c
}

// removeClient unregisters a client.
//
// It deliberately does **not** close the user's sessions. A session now
// outlives the user's connection: the browser reconnects and re-opens the
// same session id, and the device re-binds the shell it kept running. Ending the
// session here would defeat that — every network blip would kill the shell the
// reattach exists to preserve.
//
// A session whose user never returns is not leaked: the registry's idle TTL
// reaps it (the expiry worker), and the device independently reaps a detached
// shell after its own shorter TTL. This is the deliberate trade — a transient
// disconnect must not end the session, so only time (or an explicit pty_close)
// does.
func (h *UserTerminalHandler) removeClient(c *ptyUserClient) {
	h.mu.Lock()
	delete(h.clients, c.user+"#"+c.conn.RemoteAddr().String())
	h.mu.Unlock()

	// Drop this connection's ownership set. The sessions themselves stay in the
	// registry; a reconnect re-populates ownership as the browser re-opens them.
	c.mu.Lock()
	c.sessions = make(map[string]bool)
	c.mu.Unlock()

	c.closeOnce.Do(func() {
		close(c.closed)
		close(c.outbox)
	})
}

// Compile-time assertions.
var (
	_ hub.PTYUser   = (*ptyUserClient)(nil)
	_ PTYUserLookup = (*UserTerminalHandler)(nil)
)

// sessionMFAVerified reports whether the authenticated session has cleared MFA.
// A critical-tier operation requires it, matching the command path.
func sessionMFAVerified(c *gin.Context) bool {
	sess := middleware.GetSession(c)

	return sess != nil && sess.MFAVerifiedAt != nil
}
