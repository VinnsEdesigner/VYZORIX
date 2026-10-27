package websocket

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	hub "github.com/VinnsEdesigner/vyzorix/apps/api/internal/ws"
)

// fakeSink records the command results the frame handler forwards.
type fakeSink struct {
	calls []resultCall
	err   error
}

type resultCall struct {
	deviceID   string
	dispatchID string
	success    bool
	output     string
}

func (f *fakeSink) ReportResult(_ context.Context, deviceID, dispatchID string, success bool, output string) error {
	f.calls = append(f.calls, resultCall{deviceID, dispatchID, success, output})

	return f.err
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestHandleDeviceFrameRoutesCommandResult(t *testing.T) {
	sink := &fakeSink{}
	h := NewDeviceFrameHandler(sink, nil, discardLogger())

	raw := []byte(`{"type":"command_result","dispatchId":"d1","success":true,"output":"hello"}`)
	if handled := h.HandleDeviceFrame(context.Background(), hub.DeviceFrame{DeviceID: "dev-1", Type: "command_result", Raw: raw, Kind: hub.FrameText}); !handled {
		t.Fatal("command_result must be reported as handled")
	}

	if len(sink.calls) != 1 {
		t.Fatalf("sink calls = %d, want 1", len(sink.calls))
	}
	got := sink.calls[0]
	if got.deviceID != "dev-1" || got.dispatchID != "d1" || !got.success || got.output != "hello" {
		t.Fatalf("call = %+v, want deviceId=dev-1 dispatchId=d1 success=true output=hello", got)
	}
}

func TestHandleDeviceFrameAcceptsErrorAsOutputAlias(t *testing.T) {
	sink := &fakeSink{}
	h := NewDeviceFrameHandler(sink, nil, discardLogger())

	raw := []byte(`{"type":"command_result","dispatchId":"d2","success":false,"error":"boom"}`)
	h.HandleDeviceFrame(context.Background(), hub.DeviceFrame{DeviceID: "dev-1", Type: "command_result", Raw: raw, Kind: hub.FrameText})

	if len(sink.calls) != 1 {
		t.Fatalf("sink calls = %d, want 1", len(sink.calls))
	}
	if sink.calls[0].output != "boom" {
		t.Fatalf("output = %q, want the error field used as the failure text", sink.calls[0].output)
	}
}

func TestHandleDeviceFrameUnknownTypeIsNotHandled(t *testing.T) {
	sink := &fakeSink{}
	h := NewDeviceFrameHandler(sink, nil, discardLogger())

	if handled := h.HandleDeviceFrame(context.Background(), hub.DeviceFrame{DeviceID: "dev-1", Type: "something_new", Raw: []byte(`{}`), Kind: hub.FrameText}); handled {
		t.Fatal("an unknown frame type must report false so the caller can log it")
	}
	if len(sink.calls) != 0 {
		t.Fatal("an unknown frame type must not reach the sink")
	}
}

func TestHandleDeviceFrameDropsMalformedFrames(t *testing.T) {
	sink := &fakeSink{}
	h := NewDeviceFrameHandler(sink, nil, discardLogger())

	cases := map[string][]byte{
		"invalid json":       []byte(`{not json`),
		"missing dispatchId": []byte(`{"type":"command_result","success":true}`),
	}

	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			// A malformed frame must not panic and must not reach the sink.
			h.HandleDeviceFrame(context.Background(), hub.DeviceFrame{DeviceID: "dev-1", Type: "command_result", Raw: raw, Kind: hub.FrameText})
		})
	}

	if len(sink.calls) != 0 {
		t.Fatalf("sink calls = %d, want 0 for malformed frames", len(sink.calls))
	}
}

func TestHandleDeviceFrameNilSinkDoesNotPanic(t *testing.T) {
	h := NewDeviceFrameHandler(nil, nil, discardLogger())

	raw := []byte(`{"type":"command_result","dispatchId":"d1","success":true}`)
	// No sink is wired: the frame is dropped, but the type is still recognised.
	if handled := h.HandleDeviceFrame(context.Background(), hub.DeviceFrame{DeviceID: "dev-1", Type: "command_result", Raw: raw, Kind: hub.FrameText}); !handled {
		t.Fatal("command_result must be recognised even without a sink")
	}
}

func TestHandleDeviceFrameSwallowsSinkError(t *testing.T) {
	sink := &fakeSink{err: application.ErrCommandNotFound}
	h := NewDeviceFrameHandler(sink, nil, discardLogger())

	raw := []byte(`{"type":"command_result","dispatchId":"missing","success":true}`)
	// A rejected result must not panic or propagate; it is logged and dropped.
	if handled := h.HandleDeviceFrame(context.Background(), hub.DeviceFrame{DeviceID: "dev-1", Type: "command_result", Raw: raw, Kind: hub.FrameText}); !handled {
		t.Fatal("command_result must still be recognised when the sink errors")
	}
	if len(sink.calls) != 1 {
		t.Fatalf("sink calls = %d, want 1", len(sink.calls))
	}
}

func TestHandleDeviceFrameOtherErrorStillHandled(t *testing.T) {
	sink := &fakeSink{err: errors.New("database down")}
	h := NewDeviceFrameHandler(sink, nil, discardLogger())

	raw := []byte(`{"type":"command_result","dispatchId":"d3","success":true}`)
	if handled := h.HandleDeviceFrame(context.Background(), hub.DeviceFrame{DeviceID: "dev-1", Type: "command_result", Raw: raw, Kind: hub.FrameText}); !handled {
		t.Fatal("a sink error must not change whether the frame type was recognised")
	}
}
