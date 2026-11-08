package crypto

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

// TestGenerateSecretEnvelopeVector writes a cross-language test vector: a device
// keypair, a plaintext, and that plaintext wrapped to the key with this
// package's own [WrapSecret].
//
// It exists because the envelope's failure mode is silent — a secret that
// decrypts to garbage is worse than one that refuses — and because the two
// languages disagree by default on one parameter: Go's rsa.EncryptOAEP uses the
// supplied hash for both the OAEP digest and MGF1, while Java's
// OAEPWithSHA-256AndMGF1Padding leaves MGF1 at SHA-1 unless told otherwise. A
// round-trip test inside one language cannot see that. This test and the Kotlin
// harness on the other side of it can.
//
// It is a no-op unless SECRET_VECTOR_OUT names a directory, so it costs CI
// nothing and cannot fail a normal run. When the variable is set it writes:
//
//	private.pem       PKCS#8 PEM of the device key (test material, never real)
//	public.pem        PKIX PEM of the same key
//	plaintext.b64     the secret, base64
//	ciphertext.b64    that secret wrapped to the key, base64
//	algorithm.txt     SecretEnvelopeAlgorithm, the name both sides compare
func TestGenerateSecretEnvelopeVector(t *testing.T) {
	out := os.Getenv("SECRET_VECTOR_OUT")
	if out == "" {
		t.Skip("SECRET_VECTOR_OUT not set; vector generation is opt-in")
	}

	priv, err := GenerateSecretKeyPair()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	privPEM, err := EncodeRSAPrivateKeyPEM(priv)
	if err != nil {
		t.Fatalf("encode private key: %v", err)
	}

	pubPEM, err := EncodeRSAPublicKeyPEM(&priv.PublicKey)
	if err != nil {
		t.Fatalf("encode public key: %v", err)
	}

	// A value shaped like the secrets this carries in practice, so the vector
	// exercises the real length rather than a one-byte toy.
	plaintext := []byte("postgres://vyzorix:s3cr3t-pw@db.internal:5432/app?sslmode=require")

	ciphertext, err := WrapSecret(pubPEM, plaintext)
	if err != nil {
		t.Fatalf("wrap: %v", err)
	}

	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatalf("create vector dir: %v", err)
	}

	files := map[string]string{
		"private.pem":    privPEM,
		"public.pem":     pubPEM,
		"plaintext.b64":  base64.StdEncoding.EncodeToString(plaintext),
		"ciphertext.b64": ciphertext,
		"algorithm.txt":  SecretEnvelopeAlgorithm,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(out, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	t.Logf("wrote secret envelope vector to %s", out)
}
