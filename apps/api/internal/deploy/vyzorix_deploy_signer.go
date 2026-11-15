package deploy

import (
	libv "github.com/VinnsEdesigner/libv/go"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/configscan"
)

// DocumentSigner signs a rendered document and returns the signed text. A
// function, not a concrete signer, so the server has exactly one signing key.
type DocumentSigner func(document string) (string, error)

// CompileAndSign compiles a manifest and signs the result.
func CompileAndSign(m *Manifest, in Injection, sign DocumentSigner) (string, error) {
	cfg, err := Compile(m, in)
	if err != nil {
		return "", err
	}

	return SignConfig(cfg, sign)
}

// SignConfig renders a compiled config, refuses it if it carries a secret, signs
// it, and re-parses the result with the format's own parser. The scan runs
// before the signature, so an unsigned refusal is what a user sees.
func SignConfig(cfg *Config, sign DocumentSigner) (string, error) {
	if sign == nil {
		return "", ErrNoSigner
	}

	document, err := RenderConfig(*cfg)
	if err != nil {
		return "", err
	}

	if scanErr := configscan.Scan(document); scanErr != nil {
		return "", scanErr
	}

	signed, err := sign(document)
	if err != nil {
		return "", err
	}

	// A signature over text the parser rejects is a signature over nothing the
	// device can read. And a document with no signature line would be stored and
	// served as though it were signed, which is the failure the trust model exists
	// to prevent.
	parsed, err := libv.Parse(signed)
	if err != nil {
		return "", err
	}

	if !parsed.Signed {
		return "", ErrNotSigned
	}

	return signed, nil
}
