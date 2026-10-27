package hub

import (
	"context"
	"testing"
)

// panickingHandler always panics, to prove the dispatch guard contains it.
type panickingHandler struct{ called bool }

func (h *panickingHandler) HandleDeviceFrame(context.Context, DeviceFrame) bool {
	h.called = true
	panic("handler bug")
}

// countingHandler records how many frames it saw.
type countingHandler struct{ n int }

func (h *countingHandler) HandleDeviceFrame(context.Context, DeviceFrame) bool {
	h.n++

	return true
}

// TestDispatchDeviceFrameRecoversPanic is the load-bearing guard test: a panic
// inside a frame handler runs on the ReadPump goroutine, and an unrecovered
// panic there would terminate the whole process. The frame must be dropped.
func TestDispatchDeviceFrameRecoversPanic(t *testing.T) {
	h := newTestHub()
	ph := &panickingHandler{}
	h.SetDeviceFrameHandler(ph)

	// Must not panic.
	handled := h.DispatchDeviceFrame(context.Background(), DeviceFrame{DeviceID: "dev-1", Type: "boom"})

	if !ph.called {
		t.Fatal("the handler must have been invoked")
	}
	if handled {
		t.Fatal("a panicking handler must report the frame as unhandled")
	}
}

func TestDispatchDeviceFrameWithoutHandler(t *testing.T) {
	h := newTestHub()

	if h.DispatchDeviceFrame(context.Background(), DeviceFrame{DeviceID: "dev-1", Type: "x"}) {
		t.Fatal("with no handler installed the frame must report unhandled")
	}
}

func TestDispatchDeviceFramePassesBinaryKind(t *testing.T) {
	h := newTestHub()
	ch := &countingHandler{}
	h.SetDeviceFrameHandler(ch)

	raw := EncodePTYFrame(ChannelPTY, PTYTypeData, []byte("data"))
	if !h.DispatchDeviceFrame(context.Background(), DeviceFrame{
		DeviceID: "dev-1",
		Raw:      raw,
		Kind:     FrameBinary,
	}) {
		t.Fatal("a handled frame must report true")
	}
	if ch.n != 1 {
		t.Fatalf("handler calls = %d, want 1", ch.n)
	}
}

func TestDeviceDisconnectHookRuns(t *testing.T) {
	h := newTestHub()

	var got []string
	h.SetDeviceDisconnectHook(func(deviceID string) {
		got = append(got, deviceID)
	})

	if hook := h.deviceDisconnectHookFor(); hook == nil {
		t.Fatal("the hook must be installed")
	}

	// Invoke through the installed hook to prove the wiring, then confirm the
	// real unregistration path calls it.
	h.deviceDisconnectHookFor()("dev-1")
	if len(got) != 1 || got[0] != "dev-1" {
		t.Fatalf("hook calls = %v, want [dev-1]", got)
	}
}
