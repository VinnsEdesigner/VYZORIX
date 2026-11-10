// Package crypto provides cryptographic utilities.
package crypto

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/command"
)

// CommandSigner handles HMAC signing of commands.
// and.
type CommandSigner struct{}

// NewCommandSigner creates a new CommandSigner.
func NewCommandSigner() *CommandSigner {
	return &CommandSigner{}
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

	// URL-safe base64, unpadded. The signature travels as a query parameter (an
	// upgrade cannot set headers), and standard base64 emits '+', '/' and '=', all
	// of which a query string treats specially — '+' decodes as a space, so a
	// standard-base64 signature would be silently corrupted on the way in. The
	// device encodes the same way.
	sig, err := base64.RawURLEncoding.DecodeString(signature)
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
