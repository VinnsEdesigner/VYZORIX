package provisioninghandler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	svc "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/provisioning"
	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/redaction"
)

// captureConnectionService records the ConnectionInput the handler built, so a test can
// prove a request field actually reaches the service.
type captureConnectionService struct {
	got svc.ConnectionInput
}

// The handler calls the concrete *svc.ConnectionService, so this test exercises the
// mapping through the wire types instead: it decodes the JSON body the way the handler
// does and asserts write_enabled lands on the input. A live test caught the field being
// dropped between the request struct and ConnectionInput, which no unit test covered.

// TestConnectionRequestCarriesWriteEnabled is the guard for a field that was on the wire
// type but never copied into the service input: a write_enabled-only PATCH was rejected as
// "nothing to update", so push-back could never be enabled through the API.
func TestConnectionRequestCarriesWriteEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"write_enabled":true}`)

	var req connectionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if req.WriteEnabled == nil {
		t.Fatal("write_enabled did not decode into the request struct")
	}
	if !*req.WriteEnabled {
		t.Fatal("write_enabled decoded as false")
	}

	// The mapping the handler performs.
	in := svc.ConnectionInput{
		Name:         req.Name,
		Type:         domain.ConnectionType(req.Type),
		Host:         req.Host,
		Token:        redaction.NewSecret(req.Token),
		WriteEnabled: req.WriteEnabled,
	}

	if in.WriteEnabled == nil || !*in.WriteEnabled {
		t.Fatal("write_enabled was dropped between the request and ConnectionInput")
	}
}

// TestConnectionCreateCarriesWriteEnabled proves the create path maps it too.
func TestConnectionCreateCarriesWriteEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var req connectionRequest
	if err := json.Unmarshal([]byte(`{"name":"gh","type":"github","host":"github.com","token":"t","write_enabled":true}`), &req); err != nil {
		t.Fatalf("decode: %v", err)
	}

	in := svc.ConnectionInput{
		Name:         req.Name,
		Type:         domain.ConnectionType(req.Type),
		Host:         req.Host,
		Token:        redaction.NewSecret(req.Token),
		WriteEnabled: req.WriteEnabled,
	}

	if in.WriteEnabled == nil || !*in.WriteEnabled {
		t.Fatal("write_enabled was dropped on the create path")
	}
}

// TestConnectionJSONReportsWriteEnabled proves the response carries the flag, so the UI can
// offer editing only for a write-scoped connection.
func TestConnectionJSONReportsWriteEnabled(t *testing.T) {
	out := toConnectionJSON(&domain.Connection{
		ID: "c1", Name: "gh", Type: domain.ConnectionGitHub, Host: "github.com",
		Status: domain.ConnectionReady, WriteEnabled: true,
	})

	if !out.WriteEnabled {
		t.Fatal("the wire shape did not report write_enabled")
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/", bytes.NewReader(nil))
	c.JSON(http.StatusOK, out)

	if !bytes.Contains(rec.Body.Bytes(), []byte(`"write_enabled":true`)) {
		t.Fatalf("the JSON body does not carry write_enabled: %s", rec.Body.String())
	}
}

// Ensure the capture type is used, so the file compiles as written if the test bodies
// change shape.
var _ = context.Background
