// Package websocket provides WebSocket handler implementations.
package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	cmdapp "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/command"
	hub "github.com/VinnsEdesigner/vyzorix/apps/api/internal/ws"
)

// Device frame types a device may send upstream over the device WebSocket. Only
// telemetry is handled by the hub itself; everything else arrives here.
const (
	// frameTypeCommandResult reports the terminal outcome of a dispatched command.
	frameTypeCommandResult = "command_result"
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

// DeviceFrameHandler routes non-telemetry device frames to the feature that owns
// them. It handles command results (text) and PTY traffic (binary); app
// registration will add another text case here rather than in the transport
// layer.
type DeviceFrameHandler struct {
	commands CommandResultSink
	pty      PTYDeviceSink
	log      *slog.Logger
}

// NewDeviceFrameHandler creates a DeviceFrameHandler. Either sink may be nil; a
// frame for a missing sink is dropped rather than treated as an error.
func NewDeviceFrameHandler(commands CommandResultSink, pty PTYDeviceSink, log *slog.Logger) *DeviceFrameHandler {
	if log == nil {
		log = slog.Default()
	}

	return &DeviceFrameHandler{commands: commands, pty: pty, log: log}
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

// Compile-time references so the contracts stay checked.
var (
	_ hub.DeviceFrameHandler = (*DeviceFrameHandler)(nil)
	_ CommandResultSink      = (*cmdapp.Service)(nil)
)
