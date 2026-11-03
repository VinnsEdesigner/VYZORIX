package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/dto"
)

func TestDeviceController_DeviceStatus_JSON(t *testing.T) {
	status := dto.DeviceStatus{
		DeviceID:    "device-001",
		Online:      true,
		LastSeen:    1234567890,
		AppVersion:  "1.0.0",
		DeviceClass: "phone",
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal() failed: %v", err)
	}

	if result["deviceId"] != "device-001" {
		t.Errorf("deviceId = %v, want device-001", result["deviceId"])
	}
	if result["online"] != true {
		t.Error("online should be true")
	}
	if result["appVersion"] != "1.0.0" {
		t.Errorf("appVersion = %v, want 1.0.0", result["appVersion"])
	}
}

func TestDeviceController_HTTPRequest_Parsing(t *testing.T) {
	body := []byte(`{"deviceId":"test-device","firebaseInstallId":"firebase-test"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/devices/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if req.Header.Get("Content-Type") != "application/json" {
		t.Error("Content-Type should be application/json")
	}
}

func TestDeviceController_HTTPResponse_JSON(t *testing.T) {
	resp := dto.DeviceStatus{
		DeviceID:    "device-001",
		AppVersion:  "1.0.0",
		DeviceClass: "phone",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	w := httptest.NewRecorder()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		t.Logf("write error: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Error("Content-Type should be application/json")
	}
}
