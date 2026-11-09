// Package websocket provides WebSocket handler implementations.
package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	cmdapp "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/command"
	hub "github.com/VinnsEdesigner/vyzorix/apps/api/internal/ws"
)

// Device frame types a device may send upstream over the device WebSocket. Only
// telemetry is handled by the hub itself; everything else arrives here.
const (
	// frameTypeCommandResult reports the terminal outcome of a dispatched command.
	frameTypeCommandResult = "command_result"
	// frameTypeDeviceReady reports that a device applied a config version
	// (VYZORIX_DEPLOY_CONTRACT.md §9.1 step 8).
	frameTypeDeviceReady = "device_ready"
)

// CommandResultSink records a device-reported command outcome. *cmdapp.Service
// implements it; the interface keeps this handler testable without a database.
type CommandResultSink interface {
	ReportResult(ctx context.Context, deviceID, dispatchID string, success bool, output string) error
}

// commandResultFrame is the wire shape of a command_result frame:
//
//	{"type":"command_result","dispatchId":"d1","success":true,"output":"..."}
//
// "error" is accepted as an alias for the failure text so a device can send
// either spelling; "output" wins when both are present.
//
//nolint:govet // fieldalignment: field order mirrors the wire shape for clarity.
type commandResultFrame struct {
	Type       string `json:"type"`
	DispatchID string `json:"dispatchId"`
	Output     string `json:"output"`
	Error      string `json:"error"`
	Success    bool   `json:"success"`
}

// PTYDeviceSink receives binary PTY frames from a device. It is satisfied by the
// PTY relay; the interface keeps this handler independent of the relay's wiring.
type PTYDeviceSink interface {
	// HandleDeviceBinary handles one binary frame from a device. It reports
	// whether the frame was recognised.
	HandleDeviceBinary(deviceID string, raw []byte) bool
}

// ApplyReporter records that a device applied a config version. *deploy.DeployService
// implements it; the interface keeps this handler independent of the deploy wiring.
type ApplyReporter interface {
	MarkApplied(ctx context.Context, deviceID string, version int) error
}

// deviceReadyFrame is the wire shape of a device_ready frame:
//
//	{"type":"device_ready","version":7}
//
// It is the device's confirmation that it applied a config, which is how the
// server knows a deploy landed rather than merely being stored (§9.1 step 8).
type deviceReadyFrame struct {
	Type    string `json:"type"`
	Version int    `json:"version"`
}

// DeviceFrameHandler routes non-telemetry device frames to the feature that owns
// them. It handles command results (text) and PTY traffic (binary); app
// registration will add another text case here rather than in the transport
// layer.
type DeviceFrameHandler struct {
	commands CommandResultSink
	pty      PTYDeviceSink
	log      *slog.Logger
	// applies is set after the hub is already running, because the deploy feature
	// is wired last. The lock makes that late binding safe against a frame arriving
	// mid-assignment.
	applies ApplyReporter
	mu      sync.RWMutex
}

// NewDeviceFrameHandler creates a DeviceFrameHandler. Either sink may be nil; a
// frame for a missing sink is dropped rather than treated as an error.
func NewDeviceFrameHandler(commands CommandResultSink, pty PTYDeviceSink, log *slog.Logger) *DeviceFrameHandler {
	if log == nil {
		log = slog.Default()
	}

	return &DeviceFrameHandler{commands: commands, pty: pty, log: log}
}

// SetApplyReporter wires the feature that records an applied config version. It
// is called once the deploy feature exists, which is after the hub starts, so it
// is safe to call on a live handler.
func (h *DeviceFrameHandler) SetApplyReporter(reporter ApplyReporter) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.applies = reporter
}

// applyReporter returns the installed reporter under the read lock.
func (h *DeviceFrameHandler) applyReporter() ApplyReporter {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.applies
}

// HandleDeviceFrame implements hub.DeviceFrameHandler. It returns whether the
// frame was recognised.
func (h *DeviceFrameHandler) HandleDeviceFrame(_ context.Context, frame hub.DeviceFrame) bool {
	if frame.Kind == hub.FrameBinary {
		return h.handleBinary(frame)
	}

	switch frame.Type {
	case frameTypeCommandResult:
		h.handleCommandResult(frame)

		return true
	case frameTypeDeviceReady:
		h.handleDeviceReady(frame)

		return true
	default:
		return false
	}
}

// handleBinary routes a binary frame to the PTY relay.
func (h *DeviceFrameHandler) handleBinary(frame hub.DeviceFrame) bool {
	if h.pty == nil {
		return false
	}

	return h.pty.HandleDeviceBinary(frame.DeviceID, frame.Raw)
}

// handleCommandResult records a command's terminal outcome. A malformed or
// unknown frame is logged and dropped: a device must never be able to break its
// own connection (or another device's) by sending a bad frame.
func (h *DeviceFrameHandler) handleCommandResult(frame hub.DeviceFrame) {
	var payload commandResultFrame
	if err := json.Unmarshal(frame.Raw, &payload); err != nil {
		h.log.Warn("bad command_result frame", "deviceId", frame.DeviceID, "err", err)

		return
	}

	if payload.DispatchID == "" {
		h.log.Warn("command_result frame missing dispatchId", "deviceId", frame.DeviceID)

		return
	}

	if h.commands == nil {
		h.log.Warn("command_result received but no command sink is wired", "deviceId", frame.DeviceID)

		return
	}

	output := payload.Output
	if output == "" {
		output = payload.Error
	}

	err := h.commands.ReportResult(context.Background(), frame.DeviceID, payload.DispatchID, payload.Success, output)
	switch {
	case err == nil:
		h.log.Debug("command result recorded",
			"deviceId", frame.DeviceID, "dispatchId", payload.DispatchID, "success", payload.Success)
	case errors.Is(err, application.ErrCommandNotFound):
		// A device reporting a dispatch ID we do not know, or one that belongs to
		// another device. Do not reveal which; just note it.
		h.log.Warn("command_result for unknown or foreign dispatchId",
			"deviceId", frame.DeviceID, "dispatchId", payload.DispatchID)
	default:
		h.log.Error("failed to record command result",
			"deviceId", frame.DeviceID, "dispatchId", payload.DispatchID, "err", err)
	}
}

// handleDeviceReady records that a device applied a config version. A malformed
// frame is logged and dropped, like a command result: a device must never be able
// to break its own connection with a bad frame.
func (h *DeviceFrameHandler) handleDeviceReady(frame hub.DeviceFrame) {
	var payload deviceReadyFrame
	if err := json.Unmarshal(frame.Raw, &payload); err != nil {
		h.log.Warn("bad device_ready frame", "deviceId", frame.DeviceID, "err", err)

		return
	}

	if payload.Version <= 0 {
		h.log.Warn("device_ready frame missing a version", "deviceId", frame.DeviceID)

		return
	}

	reporter := h.applyReporter()
	if reporter == nil {
		h.log.Warn("device_ready received but no apply reporter is wired", "deviceId", frame.DeviceID)

		return
	}

	if err := reporter.MarkApplied(context.Background(), frame.DeviceID, payload.Version); err != nil {
		h.log.Error("failed to record applied config version",
			"deviceId", frame.DeviceID, "version", payload.Version, "err", err)

		return
	}

	h.log.Info("device reported a config applied", "deviceId", frame.DeviceID, "version", payload.Version)
}

// Compile-time references so the contracts stay checked.
var (
	_ hub.DeviceFrameHandler = (*DeviceFrameHandler)(nil)
	_ CommandResultSink      = (*cmdapp.Service)(nil)
)
