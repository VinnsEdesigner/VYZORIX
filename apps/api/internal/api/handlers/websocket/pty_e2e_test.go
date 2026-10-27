package websocket_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	websockethandlers "github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/handlers/websocket"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/audit"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/command"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/operator"
	hub "github.com/VinnsEdesigner/vyzorix/apps/api/internal/ws"
)

// End-to-end test for the terminal feature. Every server component is real -
// the hub, the device frame handler, the PTY device handler, the relay and the
// operator handler - talking over two real WebSocket servers. Only the device's
// shell is simulated, and it simulates it by echoing.
//
// Topology, identical to production:
//
//	operator browser --WS--> OperatorTerminalHandler --> PTYRelay --> Hub --> device WS
//	device WS --> Hub.ReadPump --> DeviceFrameHandler --> PTYDeviceHandler --> PTYRelay --> operator
//
// The server owns the device socket's read loop (ReadPump dispatches into the
// frame handler); the fake device is the dialer, reading what the server sends
// and writing what it wants to send.

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeDevice is the Android agent's stand-in: the dialer side of the device
// socket. It reads frames the server sends and echoes DATA back as output.
type fakeDevice struct {
	conn      *websocket.Conn
	received  chan string
	writeMu   sync.Mutex
	sessionID string
}

// write sends a frame to the server, which its ReadPump will dispatch.
func (d *fakeDevice) write(t *testing.T, frameType byte, sessionID string, data []byte) {
	t.Helper()

	frame := hub.EncodePTYFrame(hub.ChannelPTY, frameType, hub.EncodeSessionData(sessionID, data))

	d.writeMu.Lock()
	defer d.writeMu.Unlock()

	if err := d.conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
		t.Fatalf("device write: %v", err)
	}
}

// run reads server-to-device frames and answers DATA with an echo.
func (d *fakeDevice) run(t *testing.T) {
	t.Helper()

	for {
		msgType, raw, err := d.conn.ReadMessage()
		if err != nil {
			return
		}
		if msgType != websocket.BinaryMessage {
			continue
		}

		channel, frameType, payload, err := hub.DecodePTYFrame(raw)
		if err != nil || channel != hub.ChannelPTY {
			continue
		}

		sessionID, data, err := hub.DecodeSessionData(payload)
		if err != nil {
			continue
		}

		switch frameType {
		case hub.PTYTypeOpen:
			d.sessionID = sessionID
		case hub.PTYTypeData:
			d.received <- string(data)
			// A shell echoes its input back to the operator.
			d.write(t, hub.PTYTypeData, sessionID, append([]byte("echo:"), data...))
		}
	}
}

type e2eRig struct {
	operatorConn *websocket.Conn
	device       *fakeDevice
	relay        *hub.PTYRelay
	hub          *hub.Hub
}

func newE2ERig(t *testing.T, recordingDir string) *e2eRig {
	t.Helper()
	gin.SetMode(gin.TestMode)

	hubInstance := hub.New(discardLogger(), nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	go hubInstance.Run(ctx)
	time.Sleep(20 * time.Millisecond)

	// The relay sends device-bound frames through the hub, exactly as
	// wire_handlers.go wires it.
	relay := hub.NewPTYRelay(hub.NewPTYRegistry(hub.PTYRegistryConfig{
		NewRecorder: func(sessionID string) (*hub.PTYRecorder, error) {
			return hub.NewPTYRecorder(recordingDir, sessionID, 80, 24, 0)
		},
	}), hubInstance, discardLogger())

	operatorHandler := websockethandlers.NewOperatorTerminalHandler(
		relay,
		stubResolver{orgID: "org-1"},
		nil,
		audit.NewNoOpLogger(),
		recordingDir,
		[]string{"*"},
		discardLogger(),
	)

	// The hub routes frames it reads off the device socket to this handler,
	// which sends binary PTY frames to the PTY sink.
	ptySink := websockethandlers.NewPTYDeviceHandler(relay, operatorHandler, discardLogger())
	hubInstance.SetDeviceFrameHandler(websockethandlers.NewDeviceFrameHandler(nil, ptySink, discardLogger()))

	rig := &e2eRig{relay: relay, hub: hubInstance}

	// Device endpoint: the server owns the read loop.
	deviceMux := http.NewServeMux()
	deviceMux.HandleFunc("/v1/device/dev-1/stream", func(w http.ResponseWriter, r *http.Request) {
		up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		client := &hub.Client{
			DeviceID: "dev-1",
			Conn:     conn,
			Send:     make(chan command.CommandFrame, 8),
			PtySend:  make(chan []byte, 32),
			Hub:      hubInstance,
			Done:     make(chan struct{}),
		}
		hubInstance.Register(client)
		go client.WritePump()

		// ReadPump blocks and dispatches each frame into the installed handler.
		client.ReadPump()
	})
	deviceSrv := httptest.NewServer(deviceMux)
	t.Cleanup(deviceSrv.Close)

	// Operator endpoint.
	operatorMux := http.NewServeMux()
	operatorMux.HandleFunc("/v1/device/dev-1/terminal", func(w http.ResponseWriter, r *http.Request) {
		c, _ := gin.CreateTestContext(w)
		c.Request = r
		c.Params = gin.Params{{Key: "imei", Value: "dev-1"}}
		c.Set(middleware.ContextKeyOperator, &operator.Operator{ID: "op-1"})
		c.Set(middleware.ContextKeyOrganizationID, "org-1")
		operatorHandler.Handle(c)
	})
	operatorSrv := httptest.NewServer(operatorMux)
	t.Cleanup(operatorSrv.Close)

	// Connect the device (the fake agent) first, so the hub knows it.
	deviceConn, _, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(deviceSrv.URL, "http")+"/v1/device/dev-1/stream", nil)
	if err != nil {
		t.Fatalf("dial device: %v", err)
	}
	t.Cleanup(func() { _ = deviceConn.Close() })

	d := &fakeDevice{conn: deviceConn, received: make(chan string, 32)}
	rig.device = d
	go d.run(t)

	time.Sleep(80 * time.Millisecond)

	// Then the operator.
	operatorConn, _, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(operatorSrv.URL, "http")+"/v1/device/dev-1/terminal", nil)
	if err != nil {
		t.Fatalf("dial operator: %v", err)
	}
	t.Cleanup(func() { _ = operatorConn.Close() })
	rig.operatorConn = operatorConn

	return rig
}

type stubResolver struct{ orgID string }

func (s stubResolver) DeviceOrganization(context.Context, string) string { return s.orgID }

// TestE2ETerminalRoundTrip drives the whole path over real WebSockets: the
// operator opens a session, the device receives OPEN, keystrokes reach the
// device, and the device's echoed output reaches the operator.
func TestE2ETerminalRoundTrip(t *testing.T) {
	rig := newE2ERig(t, t.TempDir())

	// 1. Open a session.
	openMsg, _ := json.Marshal(map[string]any{"type": "pty_open", "sessionId": "sess-1"})
	if err := rig.operatorConn.WriteMessage(websocket.TextMessage, openMsg); err != nil {
		t.Fatalf("operator open: %v", err)
	}

	if !awaitOperatorType(t, rig.operatorConn, "pty_opened", 3*time.Second) {
		t.Fatal("operator never received pty_opened")
	}

	// 2. Operator input must reach the device.
	input := hub.EncodePTYFrame(hub.ChannelPTY, hub.PTYTypeData, hub.EncodeSessionData("sess-1", []byte("ls\n")))
	if err := rig.operatorConn.WriteMessage(websocket.BinaryMessage, input); err != nil {
		t.Fatalf("operator input: %v", err)
	}

	select {
	case got := <-rig.device.received:
		if got != "ls\n" {
			t.Fatalf("device received %q, want ls\\n", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("device never received operator input")
	}

	// 3. The device echoed it; that output must reach the operator.
	if !awaitOperatorBinary(t, rig.operatorConn, "echo:ls\n", 3*time.Second) {
		t.Fatal("operator never received device output")
	}
}

// TestE2ESessionClosedOnOperatorDisconnect proves a session does not outlive the
// operator's connection.
func TestE2ESessionClosedOnOperatorDisconnect(t *testing.T) {
	rig := newE2ERig(t, t.TempDir())

	openMsg, _ := json.Marshal(map[string]any{"type": "pty_open", "sessionId": "sess-1"})
	if err := rig.operatorConn.WriteMessage(websocket.TextMessage, openMsg); err != nil {
		t.Fatalf("operator open: %v", err)
	}
	if !awaitOperatorType(t, rig.operatorConn, "pty_opened", 3*time.Second) {
		t.Fatal("operator never received pty_opened")
	}

	if rig.relay.Registry().Count() != 1 {
		t.Fatalf("sessions = %d, want 1", rig.relay.Registry().Count())
	}

	_ = rig.operatorConn.Close()

	deadline := time.After(3 * time.Second)
	for {
		if rig.relay.Registry().Count() == 0 {
			return
		}

		select {
		case <-deadline:
			t.Fatal("session survived the operator disconnect")
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// TestE2ERecordingProducesCast proves an opted-in session writes a cast file
// containing what the operator saw.
func TestE2ERecordingProducesCast(t *testing.T) {
	dir := t.TempDir()
	rig := newE2ERig(t, dir)

	openMsg, _ := json.Marshal(map[string]any{"type": "pty_open", "sessionId": "rec-1", "record": true})
	if err := rig.operatorConn.WriteMessage(websocket.TextMessage, openMsg); err != nil {
		t.Fatalf("operator open: %v", err)
	}
	if !awaitOperatorType(t, rig.operatorConn, "pty_opened", 3*time.Second) {
		t.Fatal("operator never received pty_opened")
	}

	input := hub.EncodePTYFrame(hub.ChannelPTY, hub.PTYTypeData, hub.EncodeSessionData("rec-1", []byte("whoami\n")))
	if err := rig.operatorConn.WriteMessage(websocket.BinaryMessage, input); err != nil {
		t.Fatalf("operator input: %v", err)
	}

	if !awaitOperatorBinary(t, rig.operatorConn, "echo:whoami", 3*time.Second) {
		t.Fatal("operator never received the echoed output")
	}

	// Closing the session flushes the recorder.
	closeMsg, _ := json.Marshal(map[string]any{"type": "pty_close", "sessionId": "rec-1"})
	_ = rig.operatorConn.WriteMessage(websocket.TextMessage, closeMsg)
	time.Sleep(200 * time.Millisecond)

	path := dir + "/rec-1.cast"
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cast file missing: %v", err)
	}
	if !strings.Contains(string(raw), "echo:whoami") {
		t.Fatalf("cast does not contain the session output:\n%s", raw)
	}
}

// awaitOperatorType waits for a JSON control message of the given type.
func awaitOperatorType(t *testing.T, conn *websocket.Conn, want string, timeout time.Duration) bool {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))

		msgType, raw, err := conn.ReadMessage()
		if err != nil {
			continue
		}
		if msgType != websocket.TextMessage {
			continue
		}

		var msg struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(raw, &msg) == nil && msg.Type == want {
			return true
		}
	}

	return false
}

// awaitOperatorBinary waits for a binary PTY data frame whose payload contains
// want.
func awaitOperatorBinary(t *testing.T, conn *websocket.Conn, want string, timeout time.Duration) bool {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))

		msgType, raw, err := conn.ReadMessage()
		if err != nil {
			continue
		}
		if msgType != websocket.BinaryMessage {
			continue
		}

		channel, frameType, payload, err := hub.DecodePTYFrame(raw)
		if err != nil || channel != hub.ChannelPTY || frameType != hub.PTYTypeData {
			continue
		}

		_, data, err := hub.DecodeSessionData(payload)
		if err == nil && strings.Contains(string(data), want) {
			return true
		}
	}

	return false
}
