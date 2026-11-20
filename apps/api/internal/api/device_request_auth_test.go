package api

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/pem"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/config"
	cryptohmac "github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/crypto"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/storage"

	"github.com/gin-gonic/gin"
)

// deviceAuthServer builds a Server whose requireHMAC chain is the production one:
// a real device repository holding a device with a registered public key, and a
// verifier with a nonce cache and window.
func deviceAuthServer(t *testing.T, deviceID, publicKeyPEM string) *Server {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// ":memory:" gives every pooled connection its own database, so the seed insert
	// and the repository's later read would not see each other. One connection.
	db.SetMaxOpenConns(1)

	_, err = db.Exec(`CREATE TABLE devices (
		id TEXT PRIMARY KEY, firebase_install_id TEXT, fcm_token TEXT, app_version TEXT,
		device_class TEXT, online INTEGER NOT NULL DEFAULT 0,
		registered_at INTEGER NOT NULL DEFAULT 0, last_seen INTEGER NOT NULL DEFAULT 0,
		user_id TEXT, organization_id TEXT, created_at TIMESTAMP, updated_at TIMESTAMP,
		device_name TEXT, manufacturer TEXT, model TEXT, os_version TEXT, security_patch TEXT,
			build_id TEXT, api_level INTEGER NOT NULL DEFAULT 0, supported_abis TEXT,
			ram_bytes INTEGER NOT NULL DEFAULT 0, storage_bytes INTEGER NOT NULL DEFAULT 0,
		deregistered_at INTEGER, deletion_scheduled_at INTEGER, fcm_token_refreshed_at INTEGER,
		tags TEXT, tier INTEGER NOT NULL DEFAULT 1, public_key TEXT, secret_public_key TEXT, pairing_code TEXT,
		pairing_expires_at INTEGER,
		reported_config_version INTEGER, reported_app_digests TEXT, reported_at INTEGER NOT NULL DEFAULT 0)`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	repo := storage.NewDeviceRepository(db)
	if deviceID != "" {
		if _, err := db.Exec(
			`INSERT INTO devices (id, firebase_install_id, fcm_token, app_version, device_class,
			                      public_key, online, registered_at, last_seen, created_at, updated_at)
			 VALUES (?, '', '', '', '', ?, 0, ?, ?, ?, ?)`,
			deviceID, publicKeyPEM, time.Now().UnixMilli(), time.Now().UnixMilli(), time.Now(), time.Now(),
		); err != nil {
			t.Fatalf("seed device: %v", err)
		}
	}

	return &Server{
		config:     config.Config{EnforceHMAC: true},
		log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		DeviceRepo: repo,
		hmacVerifier: &cryptohmac.Verifier{
			Nonces: cryptohmac.NewNonceCache(5 * time.Minute),
			Window: 5 * time.Minute,
		},
	}
}

// deviceKeyPair returns a P-256 keypair and the PEM public half.
func deviceKeyPair(t *testing.T) (*ecdsa.PrivateKey, string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	return key, string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// signDeviceRequest reproduces crypto.DeviceCanonicalString and signs it, the way
// the device must.
func signDeviceRequest(t *testing.T, key *ecdsa.PrivateKey, method, path, deviceID, ts, nonce string, body []byte) string {
	t.Helper()

	bodyHash := sha256.Sum256(body)
	canonical := method + "\n" + path + "\n" + deviceID + "\n" + ts + "\n" + nonce + "\n" +
		base64.StdEncoding.EncodeToString(bodyHash[:])

	digest := sha256.Sum256([]byte(canonical))
	sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	return base64.StdEncoding.EncodeToString(sig)
}

// runRequireHMAC drives a request through the real middleware and reports whether
// the request reached the handler.
func runRequireHMAC(t *testing.T, s *Server, deviceID string, req *http.Request) bool {
	t.Helper()

	reached := false

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		// Pin the path param the middleware reads.
		c.Params = gin.Params{{Key: "imei", Value: deviceID}}
		c.Next()
	})
	engine.Use(s.requireHMAC())
	engine.POST("/v1/device/:imei/command", func(c *gin.Context) {
		reached = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	return reached
}

func TestRequireHMAC_AcceptsAValidDeviceSignature(t *testing.T) {
	key, pubPEM := deviceKeyPair(t)
	s := deviceAuthServer(t, "dev-1", pubPEM)

	body := []byte(`{"command":"SHELL_EXEC"}`)
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	nonce := "nonce-1"
	sig := signDeviceRequest(t, key, http.MethodPost, "/v1/device/dev-1/command", "dev-1", ts, nonce, body)

	req := httptest.NewRequest(http.MethodPost, "/v1/device/dev-1/command", bytes.NewReader(body))
	req.Header.Set("X-Vyzorix-Timestamp", ts)
	req.Header.Set("X-Vyzorix-Nonce", nonce)
	req.Header.Set("X-Vyzorix-Signature", sig)

	if !runRequireHMAC(t, s, "dev-1", req) {
		t.Fatal("a correctly signed device request must be accepted")
	}
}

func TestRequireHMAC_RejectsATamperedBody(t *testing.T) {
	key, pubPEM := deviceKeyPair(t)
	s := deviceAuthServer(t, "dev-1", pubPEM)

	signed := []byte(`{"command":"SHELL_EXEC"}`)
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	nonce := "nonce-2"
	sig := signDeviceRequest(t, key, http.MethodPost, "/v1/device/dev-1/command", "dev-1", ts, nonce, signed)

	// The body on the wire differs from the one signed.
	tampered := []byte(`{"command":"FACTORY_RESET"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/device/dev-1/command", bytes.NewReader(tampered))
	req.Header.Set("X-Vyzorix-Timestamp", ts)
	req.Header.Set("X-Vyzorix-Nonce", nonce)
	req.Header.Set("X-Vyzorix-Signature", sig)

	if runRequireHMAC(t, s, "dev-1", req) {
		t.Fatal("a tampered body must be rejected")
	}
}

func TestRequireHMAC_RejectsAnUnknownDevice(t *testing.T) {
	key, pubPEM := deviceKeyPair(t)
	// The repository holds dev-1, not dev-2.
	s := deviceAuthServer(t, "dev-1", pubPEM)

	body := []byte(`{}`)
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	nonce := "nonce-3"
	sig := signDeviceRequest(t, key, http.MethodPost, "/v1/device/dev-2/command", "dev-2", ts, nonce, body)

	req := httptest.NewRequest(http.MethodPost, "/v1/device/dev-2/command", bytes.NewReader(body))
	req.Header.Set("X-Vyzorix-Timestamp", ts)
	req.Header.Set("X-Vyzorix-Nonce", nonce)
	req.Header.Set("X-Vyzorix-Signature", sig)

	if runRequireHMAC(t, s, "dev-2", req) {
		t.Fatal("a request for an unknown device must be rejected")
	}
}

func TestRequireHMAC_RejectsAStaleTimestamp(t *testing.T) {
	key, pubPEM := deviceKeyPair(t)
	s := deviceAuthServer(t, "dev-1", pubPEM)

	body := []byte(`{}`)
	// Well outside the 5 minute window.
	ts := strconv.FormatInt(time.Now().Add(-time.Hour).UnixMilli(), 10)
	nonce := "nonce-4"
	sig := signDeviceRequest(t, key, http.MethodPost, "/v1/device/dev-1/command", "dev-1", ts, nonce, body)

	req := httptest.NewRequest(http.MethodPost, "/v1/device/dev-1/command", bytes.NewReader(body))
	req.Header.Set("X-Vyzorix-Timestamp", ts)
	req.Header.Set("X-Vyzorix-Nonce", nonce)
	req.Header.Set("X-Vyzorix-Signature", sig)

	if runRequireHMAC(t, s, "dev-1", req) {
		t.Fatal("a stale timestamp must be rejected")
	}
}

func TestRequireHMAC_RejectsAReplayedNonce(t *testing.T) {
	key, pubPEM := deviceKeyPair(t)
	s := deviceAuthServer(t, "dev-1", pubPEM)

	body := []byte(`{}`)
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	nonce := "nonce-5"
	sig := signDeviceRequest(t, key, http.MethodPost, "/v1/device/dev-1/command", "dev-1", ts, nonce, body)

	makeReq := func() *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/v1/device/dev-1/command", bytes.NewReader(body))
		req.Header.Set("X-Vyzorix-Timestamp", ts)
		req.Header.Set("X-Vyzorix-Nonce", nonce)
		req.Header.Set("X-Vyzorix-Signature", sig)
		return req
	}

	if !runRequireHMAC(t, s, "dev-1", makeReq()) {
		t.Fatal("the first request must be accepted")
	}
	if runRequireHMAC(t, s, "dev-1", makeReq()) {
		t.Fatal("a replayed nonce must be rejected")
	}
}
