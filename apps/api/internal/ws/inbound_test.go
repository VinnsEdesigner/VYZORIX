package hub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/command"
	"github.com/gorilla/websocket"
)

// recordingFrameHandler captures the frames ReadPump routes to the handler.
type recordingFrameHandler struct {
	mu     sync.Mutex
	frames []recordedFrame
}

type recordedFrame struct {
	deviceID  string
	frameType string
	raw       string
}

func (h *recordingFrameHandler) HandleDeviceFrame(_ context.Context, deviceID, frameType string, raw []byte) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.frames = append(h.frames, recordedFrame{deviceID, frameType, string(raw)})

	return frameType == "command_result"
}

func (h *recordingFrameHandler) snapshot() []recordedFrame {
	h.mu.Lock()
	defer h.mu.Unlock()

	out := make([]recordedFrame, len(h.frames))
	copy(out, h.frames)

	return out
}

// TestReadPumpRoutesNonTelemetryFrames proves the transport actually hands a
// command_result frame to the installed handler, and that telemetry still takes
// the telemetry path rather than being double-delivered.
func TestReadPumpRoutesNonTelemetryFrames(t *testing.T) {
	h := newTestHub()
	handler := &recordingFrameHandler{}
	h.SetDeviceFrameHandler(handler)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.Run(ctx)
	time.Sleep(20 * time.Millisecond)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		client := &Client{
			DeviceID: "dev-1",
			Conn:     conn,
			Send:     make(chan command.CommandFrame, 8),
			Hub:      h,
			Done:     make(chan struct{}),
		}
		go client.WritePump()
		client.ReadPump()
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if err := conn.WriteMessage(websocket.TextMessage,
		[]byte(`{"type":"command_result","dispatchId":"d1","success":true,"output":"hi"}`)); err != nil {
		t.Fatalf("write command_result: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage,
		[]byte(`{"type":"telemetry","riskScore":7}`)); err != nil {
		t.Fatalf("write telemetry: %v", err)
	}

	deadline := time.After(2 * time.Second)
	for {
		frames := handler.snapshot()
		if len(frames) >= 1 {
			if frames[0].deviceID != "dev-1" {
				t.Fatalf("deviceId = %q, want dev-1", frames[0].deviceID)
			}
			if frames[0].frameType != "command_result" {
				t.Fatalf("first routed frame = %q, want command_result", frames[0].frameType)
			}
			if !strings.Contains(frames[0].raw, `"output":"hi"`) {
				t.Fatalf("raw = %q, want the original frame bytes", frames[0].raw)
			}

			// Telemetry must NOT have been routed to the frame handler.
			for _, f := range frames {
				if f.frameType == "telemetry" {
					t.Fatal("telemetry must be handled by the telemetry path, not the frame handler")
				}
			}

			return
		}

		select {
		case <-deadline:
			t.Fatal("timed out waiting for the command_result frame to be routed")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
