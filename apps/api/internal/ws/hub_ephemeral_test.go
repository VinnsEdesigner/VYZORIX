package hub

import (
	"context"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/command"
)

func TestSendEphemeralDeliversToConnectedDevice(t *testing.T) {
	h := newTestHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(20 * time.Millisecond)

	c := &Client{
		DeviceID: "dev-1",
		Send:     make(chan command.CommandFrame, 4),
		PtySend:  make(chan []byte, 4),
		Hub:      h,
		Done:     make(chan struct{}),
	}
	h.Register(c)
	time.Sleep(50 * time.Millisecond)

	frame := EncodePTYFrame(ChannelPTY, PTYTypeData, []byte("hi"))
	if !h.SendEphemeral("dev-1", frame) {
		t.Fatal("SendEphemeral must deliver to a connected device")
	}

	select {
	case got := <-c.PtySend:
		if string(got) != string(frame) {
			t.Fatalf("frame = %v, want %v", got, frame)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for the ephemeral frame")
	}
}

// TestSendEphemeralDropsWhenOffline is the core guarantee: unlike Send, an
// offline device must NOT cause a persisted queue write. A nil message queue
// makes any queue attempt a no-op, so the observable contract is simply false.
func TestSendEphemeralDropsWhenOffline(t *testing.T) {
	h := newTestHub()
	// No message queue is configured on newTestHub; Send would return false too,
	// but the point is that SendEphemeral never even tries to persist.

	if h.SendEphemeral("nobody-home", EncodePTYFrame(ChannelPTY, PTYTypeData, []byte("x"))) {
		t.Fatal("SendEphemeral must report false for an offline device")
	}
}

func TestSendEphemeralDropsWhenBufferFull(t *testing.T) {
	h := newTestHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(20 * time.Millisecond)

	// A zero-capacity buffer is always full for a non-blocking send.
	c := &Client{
		DeviceID: "dev-1",
		Send:     make(chan command.CommandFrame, 4),
		PtySend:  make(chan []byte, 0),
		Hub:      h,
		Done:     make(chan struct{}),
	}
	h.Register(c)
	time.Sleep(50 * time.Millisecond)

	if h.SendEphemeral("dev-1", EncodePTYFrame(ChannelPTY, PTYTypeData, []byte("x"))) {
		t.Fatal("SendEphemeral must drop (return false) when the buffer is full")
	}
}

func TestSendEphemeralIgnoresEmptyFrame(t *testing.T) {
	h := newTestHub()

	if h.SendEphemeral("dev-1", nil) {
		t.Fatal("an empty frame must not be reported as sent")
	}
}

func TestSendEphemeralClientWithoutPtyChannel(t *testing.T) {
	h := newTestHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)
	time.Sleep(20 * time.Millisecond)

	// A client created before the PTY channel existed has no PtySend; the send
	// must report false rather than panic on a nil channel.
	c := &Client{
		DeviceID: "dev-1",
		Send:     make(chan command.CommandFrame, 4),
		Hub:      h,
		Done:     make(chan struct{}),
	}
	h.Register(c)
	time.Sleep(50 * time.Millisecond)

	if h.SendEphemeral("dev-1", EncodePTYFrame(ChannelPTY, PTYTypeData, []byte("x"))) {
		t.Fatal("a client with no PTY channel must report false")
	}
}
