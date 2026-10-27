// Package websocket provides WebSocket handler implementations.
package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	cmdapp "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/command"
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

// DeviceFrameHandler routes non-telemetry device frames to the feature that owns
// them. It currently handles command results; PTY data and app registration will
// register additional cases here rather than in the transport layer.
type DeviceFrameHandler struct {
	commands CommandResultSink
	log      *slog.Logger
}

// NewDeviceFrameHandler creates a DeviceFrameHandler.
func NewDeviceFrameHandler(commands CommandResultSink, log *slog.Logger) *DeviceFrameHandler {
	if log == nil {
		log = slog.Default()
	}

	return &DeviceFrameHandler{commands: commands, log: log}
}

// HandleDeviceFrame implements hub.DeviceFrameHandler. It returns whether the
// frame type was recognised.
func (h *DeviceFrameHandler) HandleDeviceFrame(ctx context.Context, deviceID, frameType string, raw []byte) bool {
	switch frameType {
	case frameTypeCommandResult:
		h.handleCommandResult(ctx, deviceID, raw)

		return true
	default:
		return false
	}
}

// handleCommandResult records a command's terminal outcome. A malformed or
// unknown frame is logged and dropped: a device must never be able to break its
// own connection (or another device's) by sending a bad frame.
func (h *DeviceFrameHandler) handleCommandResult(ctx context.Context, deviceID string, raw []byte) {
	var frame commandResultFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		h.log.Warn("bad command_result frame", "deviceId", deviceID, "err", err)

		return
	}

	if frame.DispatchID == "" {
		h.log.Warn("command_result frame missing dispatchId", "deviceId", deviceID)

		return
	}

	if h.commands == nil {
		h.log.Warn("command_result received but no command sink is wired", "deviceId", deviceID)

		return
	}

	output := frame.Output
	if output == "" {
		output = frame.Error
	}

	err := h.commands.ReportResult(ctx, deviceID, frame.DispatchID, frame.Success, output)
	switch {
	case err == nil:
		h.log.Debug("command result recorded",
			"deviceId", deviceID, "dispatchId", frame.DispatchID, "success", frame.Success)
	case errors.Is(err, application.ErrCommandNotFound):
		// A device reporting a dispatch ID we do not know, or one that belongs to
		// another device. Do not reveal which; just note it.
		h.log.Warn("command_result for unknown or foreign dispatchId",
			"deviceId", deviceID, "dispatchId", frame.DispatchID)
	default:
		h.log.Error("failed to record command result",
			"deviceId", deviceID, "dispatchId", frame.DispatchID, "err", err)
	}
}

// Compile-time reference so the unused import stays meaningful if the command
// service signature changes; ReportResult is the contract this handler needs.
var _ CommandResultSink = (*cmdapp.Service)(nil)
