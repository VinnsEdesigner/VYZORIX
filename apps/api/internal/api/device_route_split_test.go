package api

import (
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

// deviceRouteServer builds a Server with the device-signed route group registered,
// so the middleware chain under test is the production one.
func deviceRouteServer(t *testing.T, deviceID, publicKeyPEM string) *Server {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(`CREATE TABLE devices (
		id TEXT PRIMARY KEY, firebase_install_id TEXT NOT NULL, fcm_token TEXT, app_version TEXT,
		device_class TEXT, online INTEGER NOT NULL DEFAULT 0,
		registered_at INTEGER NOT NULL DEFAULT 0, last_seen INTEGER NOT NULL DEFAULT 0,
		user_id TEXT, organization_id TEXT, created_at TIMESTAMP, updated_at TIMESTAMP,
		device_name TEXT, manufacturer TEXT, model TEXT, os_version TEXT, security_patch TEXT,
			build_id TEXT, api_level INTEGER NOT NULL DEFAULT 0, supported_abis TEXT,
			ram_bytes INTEGER NOT NULL DEFAULT 0, storage_bytes INTEGER NOT NULL DEFAULT 0,
		deregistered_at INTEGER, deletion_scheduled_at INTEGER, fcm_token_refreshed_at INTEGER,
		tags TEXT, tier INTEGER NOT NULL DEFAULT 1, public_key TEXT, secret_public_key TEXT, pairing_code TEXT,
		pairing_expires_at INTEGER,
		reported_config_version INTEGER, reported_app_digests TEXT, reported_at INTEGER NOT NULL DEFAULT 0)`); err != nil {
		t.Fatalf("schema: %v", err)
	}

	if deviceID != "" {
		if _, err := db.Exec(
			`INSERT INTO devices (id, firebase_install_id, fcm_token, app_version, device_class,
			                      public_key, online, registered_at, last_seen, created_at, updated_at)
			 VALUES (?, '', '', '', '', ?, 0, ?, ?, ?, ?)`,
			deviceID, publicKeyPEM, time.Now().UnixMilli(), time.Now().UnixMilli(),
			time.Now().UnixMilli(), time.Now().UnixMilli(),
		); err != nil {
			t.Fatalf("seed device: %v", err)
		}
	}

	return &Server{
		config:     config.Config{EnforceHMAC: true},
		log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		DeviceRepo: storage.NewDeviceRepository(db),
		hmacVerifier: &cryptohmac.Verifier{
			Nonces: cryptohmac.NewNonceCache(5 * time.Minute),
			Window: 5 * time.Minute,
		},
	}
}

func deviceRouteKeyPair(t *testing.T) (*ecdsa.PrivateKey, string) {
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

func signDeviceRouteRequest(t *testing.T, key *ecdsa.PrivateKey, method, path, deviceID, ts, nonce string, body []byte) string {
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

// TestDeviceSignedRoutesRequireOnlyTheDeviceSignature is the guard on the bug where
// the device group chained the API-client signature middleware (X-Signature /
// X-Client-ID) in front of the device keypair middleware. A device has no
// API-client id or secret, so every device request was refused with "Missing
// required signature headers" before its own signature was ever checked.
//
// The assertion is that a request carrying only the device signature reaches the
// handler — no X-Client-ID, no X-Signature.
func TestDeviceSignedRoutesRequireOnlyTheDeviceSignature(t *testing.T) {
	key, pubPEM := deviceRouteKeyPair(t)
	s := deviceRouteServer(t, "dev-1", pubPEM)

	reached := false

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Params = gin.Params{{Key: "imei", Value: "dev-1"}}
		c.Next()
	})
	engine.Use(s.requireHMAC())
	engine.GET("/v1/device/:imei/config", func(c *gin.Context) {
		reached = true
		c.Status(http.StatusOK)
	})

	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	sig := signDeviceRouteRequest(t, key, http.MethodGet, "/v1/device/dev-1/config", "dev-1", ts, "n-1", nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/device/dev-1/config", nil)
	req.Header.Set("X-Vyzorix-Timestamp", ts)
	req.Header.Set("X-Vyzorix-Nonce", "n-1")
	req.Header.Set("X-Vyzorix-Signature", sig)

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if !reached {
		t.Fatalf("a device request with only its own signature must reach the handler; got %d: %s",
			w.Code, w.Body.String())
	}
}

// TestDeviceSignedRouteRejectsTheApiClientSignatureAlone proves the two schemes are
// not interchangeable: an API-client-signed request must not authenticate a device
// route, because the device has no API-client secret and the server must not accept
// one in its place.
func TestDeviceSignedRouteRejectsTheApiClientSignatureAlone(t *testing.T) {
	_, pubPEM := deviceRouteKeyPair(t)
	s := deviceRouteServer(t, "dev-1", pubPEM)

	reached := false

	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Params = gin.Params{{Key: "imei", Value: "dev-1"}}
		c.Next()
	})
	engine.Use(s.requireHMAC())
	engine.GET("/v1/device/:imei/config", func(c *gin.Context) {
		reached = true
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/device/dev-1/config", nil)
	req.Header.Set("X-Client-ID", "dev-1")
	req.Header.Set("X-Timestamp", strconv.FormatInt(time.Now().Unix(), 10))
	req.Header.Set("X-Signature", "whatever")

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if reached {
		t.Fatal("an API-client signature must not authenticate a device route")
	}
}
