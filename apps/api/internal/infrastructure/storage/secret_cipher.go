package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// secretCipher encrypts reversible secrets at rest with AES-256-GCM.
//
// It exists for secrets the server must USE later, not just compare — a Slack
// bot token has to be sent back to Slack on every notification, so it cannot be
// hashed the way the webhook secret is. The key is derived from the session
// secret so no new configuration is required.
type secretCipher struct {
	key []byte
}

func newSecretCipher(secret string) *secretCipher {
	sum := sha256.Sum256([]byte(secret))
	return &secretCipher{key: sum[:]}
}

// encrypt returns hex(nonce || ciphertext), or the empty string for empty input.
func (c *secretCipher) encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return hex.EncodeToString(gcm.Seal(nonce, nonce, []byte(plaintext), nil)), nil
}

// decrypt reverses encrypt. A value that is not valid hex, or does not decrypt,
// is returned as-is: it predates encryption (stored in plaintext), so the caller
// still gets a usable token rather than an error.
func (c *secretCipher) decrypt(stored string) string {
	if stored == "" {
		return ""
	}
	raw, err := hex.DecodeString(stored)
	if err != nil {
		return stored
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return stored
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return stored
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return stored
	}
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return stored
	}
	return string(plaintext)
}
