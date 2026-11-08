package configscan

import "testing"

// The scanner is the invariant both config paths rest on: a config is
// transmitted, so it must never carry a secret value. Each case is asserted by
// the outcome, and the two halves — naming accepted, carrying refused — are
// tested together, because a rule that refuses too much is as wrong as one that
// refuses too little.

func TestRefusesASecretValue(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"pem in a value":        "v device d1\nvv note -----BEGIN PRIVATE KEY-----\n",
		"jwt in a value":        "v device d1\nvv note eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U\n",
		"user:pass url":         "v device d1\nvv note https://user:pw@host/x\n",
		"secret-naming key":     "v device d1\nvv token abc123\n",
		"hmac key":              "v device d1\nvv hmac sha256 deadbeef\n",
		"password key":          "v device d1\nvv password hunter2\n",
		"nested token key":      "v device d1\nvv cloud\nvvv api_token xyz\n",
		"secret key not in app": "v device d1\nvv secret abc123\n",
		"hex run in env":        "v device d1\nvv env K=9f2c1b3a4d5e6f708192a3b4c5d6e7f8091a2b3c4\n",
		"secret-shaped env key": "v device d1\nvv env API_TOKEN=short\n",
	}

	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := Scan(doc); err == nil {
				t.Fatalf("%s must be refused", name)
			}
		})
	}
}

func TestAllowsANameAndOrdinaryConfig(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"a secret name inside an app": "v device d1\nvv apps\nvvv app vfd\nvvvv secret DATABASE_URL\n",
		"an artifact digest":          "v device d1\nvv apps\nvvv app vfd\nvvvv artifact engine\nvvvvv sha256 9f2c000000000000000000000000000000000000000000000000000000000000\n",
		"an ordinary env value":       "v device d1\nvv apps\nvvv app vfd\nvvvv env MI_PORT=8080\n",
		"the signature line":          "v device d1\nv sig ecdsa-p256 MEYCIQ==\n",
		"a short token-like value":    "v device d1\nvv apps\nvvv app vfd\nvvvv env MODE=token\n",
	}

	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := Scan(doc); err != nil {
				t.Fatalf("%s must be accepted: %v", name, err)
			}
		})
	}
}

func TestRefusesAMalformedDocument(t *testing.T) {
	t.Parallel()

	if err := Scan("not a document with a depth jump\nvvv deep\n"); err == nil {
		t.Fatal("a malformed document must be refused, not silently accepted")
	}
}
