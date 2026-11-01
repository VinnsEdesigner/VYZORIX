// Package crypto provides cryptographic utilities.
package crypto

import (
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/command"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/security/password"
)

// ErrHashSecretFailed is returned when hashing a secret fails.
var ErrHashSecretFailed = errors.New("failed to hash secret: crypto/rand unavailable")

// ErrInvalidHashFormat is returned when the hash format is invalid.
var ErrInvalidHashFormat = errors.New("invalid hash format")

// CommandSigner handles HMAC signing of commands per DEVICE_REGISTRATION.md §5.
// and COMMAND_SECURITY.md §3.
type CommandSigner struct{}

// NewCommandSigner creates a new CommandSigner.
func NewCommandSigner() *CommandSigner {
	return &CommandSigner{}
}

// SignCommandFrame signs a CommandFrame in place using the device's command.
// secret. It generates a fresh nonce, sets the frame timestamp (if zero), and.
// fills the Nonce + Signature fields. This is the shared entry point used by.
// all command dispatch paths (direct execute, outbox retry, update push) so.
// every frame reaching a device is signed consistently.
//
// The secret passed in is the device's CommandSecretHash — a deterministic.
// SHA-256 derivation of the plaintext secret. Both the server and the device.
// can compute the same key without the server storing the plaintext.
func SignCommandFrame(s *CommandSigner, frame *command.CommandFrame, deviceID, secret string) error {
	if frame.Timestamp == 0 {
		frame.Timestamp = s.GenerateTimestampMs()
	}
	nonce, sig, err := s.SignCommand(frame, deviceID, secret)
	if err != nil {
		return err
	}
	frame.Nonce = nonce
	frame.Signature = sig
	return nil
}

// SignCommand generates a nonce and HMAC signature for a command frame.
// Returns the nonce, HMAC hex string, and any error.
//
// Canonical string format (per COMMAND_SECURITY.md §3):.
// {dispatchId}|{deviceId}|{command}|{timestamp_unix_ms}|{nonce}|{args}.
func (s *CommandSigner) SignCommand(frame *command.CommandFrame, deviceID, secret string) (nonce string, hmacHex string, err error) {
	// Generate 16 random bytes → 32 hex chars.
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	nonce = hex.EncodeToString(nonceBytes)

	// Build canonical string.
	canonical := BuildCanonicalString(frame, deviceID, nonce)

	// Compute HMAC-SHA512.
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write([]byte(canonical))
	hmacHex = hex.EncodeToString(mac.Sum(nil))

	return nonce, hmacHex, nil
}

// BuildCanonicalString constructs the canonical message string for HMAC computation.
// Format: {dispatchId}|{deviceId}|{command}|{timestamp_unix_ms}|{nonce}|{args}.
func BuildCanonicalString(frame *command.CommandFrame, deviceID, nonce string) string {
	// Timestamp is already in Unix milliseconds (int64).
	argsStr := string(frame.Args)
	if argsStr == "" {
		argsStr = "{}"
	}

	return frame.DispatchID + "|" + deviceID + "|" + frame.Command + "|" +
		strconv.FormatInt(frame.Timestamp, 10) + "|" + nonce + "|" + argsStr
}

// ValidateCommandHMAC validates a command frame's HMAC signature.
// Returns true if valid, false otherwise.
func (s *CommandSigner) ValidateCommandHMAC(frame *command.CommandFrame, deviceID, secret string) bool {
	// Recompute canonical string with the frame's nonce and timestamp.
	argsStr := string(frame.Args)
	if argsStr == "" {
		argsStr = "{}"
	}

	canonical := frame.DispatchID + "|" + deviceID + "|" + frame.Command + "|" +
		strconv.FormatInt(frame.Timestamp, 10) + "|" + frame.Nonce + "|" + argsStr

	// Compute expected HMAC.
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write([]byte(canonical))
	expected := hex.EncodeToString(mac.Sum(nil))

	// Constant-time comparison.
	return hmac.Equal([]byte(frame.Signature), []byte(expected))
}

// ValidateTimestamp checks if the command timestamp is within the ±30s window.
func (s *CommandSigner) ValidateTimestamp(frame *command.CommandFrame, maxDriftMs int64) bool {
	if maxDriftMs == 0 {
		maxDriftMs = 30_000 // Default ±30 seconds.
	}

	// Timestamp is already Unix milliseconds.
	nowMs := time.Now().UnixMilli()

	drift := nowMs - frame.Timestamp
	if drift < 0 {
		drift = -drift
	}

	return drift <= maxDriftMs
}

// SignCommandFrameWithKey signs a frame with the server's own key rather than a
// per-device shared secret.
//
// This is the keypair model's signing path: the server holds a private key that
// never leaves it, and every device verifies against the same public key pinned in
// the app. Nothing per-device is transmitted, which is the property the HMAC
// scheme could not have — it needed the secret delivered to each device, and the
// only channel available was FCM.
//
// The signature covers the frame's canonical string with a domain prefix, so a
// device signature can never be replayed as a server one.
func SignCommandFrameWithKey(
	signer *ServerSigner,
	frame *command.CommandFrame,
	deviceID string,
) error {
	if frame.Timestamp == 0 {
		frame.Timestamp = time.Now().UnixMilli()
	}

	// The nonce must be on the frame *before* the canonical string is built: the
	// string includes it, so signing with an empty nonce and setting it afterwards
	// produces a signature that can never verify.
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return fmt.Errorf("nonce: %w", err)
	}
	frame.Nonce = hex.EncodeToString(nonceBytes)

	sig, err := signer.SignCanonical(CommandCanonical(frame, deviceID))
	if err != nil {
		return err
	}

	frame.Signature = sig

	return nil
}

// CommandSignKind is the signature domain for a command frame. It keeps commands
// distinguishable from config and from a connect, so one cannot be replayed as
// another.
const CommandSignKind = "command"

// CommandCanonical is the flat string a command frame's signature covers.
//
// It is one function, used by both signing and verification, so the two cannot
// drift — which the HMAC path did: it had BuildCanonicalString and a second
// inline copy in ValidateCommandHMAC that could disagree.
//
// The domain prefix is included, so a signature over a command cannot be replayed
// as a signature over anything else. Every field is present and separator-delimited,
// so no two different frames produce the same string.
func CommandCanonical(frame *command.CommandFrame, deviceID string) string {
	argsStr := string(frame.Args)
	if argsStr == "" {
		argsStr = "{}"
	}

	return CommandSignKind + "|" + frame.DispatchID + "|" + deviceID + "|" + frame.Command + "|" +
		strconv.FormatInt(frame.Timestamp, 10) + "|" + frame.Nonce + "|" + argsStr
}

// ValidateCommandSignatureWithKey verifies a frame signed by the server's key.
// The device does the same check against the pinned public key.
func ValidateCommandSignatureWithKey(
	serverPublicKeyPEM string,
	frame *command.CommandFrame,
	deviceID string,
) bool {
	pub, err := DecodePublicKey(serverPublicKeyPEM)
	if err != nil {
		return false
	}

	sig, err := base64.StdEncoding.DecodeString(frame.Signature)
	if err != nil {
		return false
	}

	digest := sha256.Sum256([]byte(CommandCanonical(frame, deviceID)))

	return ecdsa.VerifyASN1(pub, digest[:], sig)
}

// ValidateConnectHMAC validates WebSocket connection HMAC per DEVICE_REGISTRATION.md §4.1.
// Format: HMAC over "CONNECT:<deviceId>:<timestamp>:<nonce>".
//
// Deprecated: the keypair model replaces this. See VerifyConnectSignature. Kept so
// an already-deployed device is not cut off by a server upgrade.
func (s *CommandSigner) ValidateConnectHMAC(deviceID, timestamp, nonce, providedHmac, secret string) bool {
	canonical := "CONNECT:" + deviceID + ":" + timestamp + ":" + nonce

	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write([]byte(canonical))
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(providedHmac), []byte(expected))
}

// ConnectSignKind is the device-signature domain for a WebSocket connect. It is
// distinct from the command and config domains so a connect signature cannot be
// replayed as anything else.
const ConnectSignKind = "connect"

// ConnectCanonical is the flat string a device signs for its WebSocket connect.
// One function, used by both sides, for the same reason CommandCanonical is.
func ConnectCanonical(deviceID, timestamp, nonce string) string {
	return ConnectSignKind + "|" + deviceID + "|" + timestamp + "|" + nonce
}

// VerifyConnectSignature verifies a device's WebSocket connect handshake using its
// registered public key.
//
// This replaces ValidateConnectHMAC. The device signs with the private key it
// generated at first boot; the server verifies against the public half it received
// at enroll. Nothing shared, so nothing to transmit.
func VerifyConnectSignature(
	devicePublicKeyPEM, deviceID, timestamp, nonce, signature string,
) bool {
	pub, err := DecodePublicKey(devicePublicKeyPEM)
	if err != nil {
		return false
	}

	sig, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false
	}

	digest := sha256.Sum256([]byte(ConnectCanonical(deviceID, timestamp, nonce)))

	return ecdsa.VerifyASN1(pub, digest[:], sig)
}

// GenerateNonce generates a cryptographically random 16-byte nonce (32 hex chars).
func (s *CommandSigner) GenerateNonce() (string, error) {
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", errors.New("failed to generate nonce")
	}

	return hex.EncodeToString(nonceBytes), nil
}

// GenerateTimestamp generates a Unix timestamp string for connect HMAC.
func (s *CommandSigner) GenerateTimestamp() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

// GenerateTimestampMs generates a Unix timestamp in milliseconds.
func (s *CommandSigner) GenerateTimestampMs() int64 {
	return time.Now().UnixMilli()
}

// HashSecret creates an Argon2id hash of the command secret for secure storage.
// Uses OWASP 2023 recommended parameters for strong protection against brute force.
func (s *CommandSigner) HashSecret(secret string) (string, error) {
	hash, err := password.HashSecret(secret)
	if err != nil {
		return "", ErrHashSecretFailed
	}
	return hash, nil
}

// VerifySecretHash verifies a secret against its Argon2id hash.
func (s *CommandSigner) VerifySecretHash(secret, hash string) bool {
	err := password.VerifySecret(secret, hash)
	if err == nil {
		return true
	}
	// Check if it's the legacy SHA512-based hash format.
	if s.isLegacyHash(hash) {
		return s.verifyLegacyHash(secret, hash)
	}

	return false
}

// isLegacyHash checks if the hash is in the legacy SHA512-based format.
func (s *CommandSigner) isLegacyHash(hash string) bool {
	// Legacy format: saltHex:hashHex (no $argon2id$ prefix).
	return len(hash) > 0 && hash[0] != '$'
}

// verifyLegacyHash verifies using the old SHA512-based hash format.
// This is kept for backward compatibility with existing stored hashes.
func (s *CommandSigner) verifyLegacyHash(secret, hash string) bool {
	parts := splitHash(hash)
	if len(parts) != 2 {
		return false
	}

	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}

	mac := sha512.New()
	mac.Write(salt)
	mac.Write([]byte(secret))
	expected := mac.Sum(nil)

	return hmac.Equal([]byte(hex.EncodeToString(expected)), []byte(parts[1]))
}

func splitHash(hash string) []string {
	for i := 0; i < len(hash); i++ {
		if hash[i] == ':' {
			return []string{hash[:i], hash[i+1:]}
		}
	}

	return nil
}
