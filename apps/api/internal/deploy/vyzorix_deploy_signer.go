package deploy

import (
	libv "github.com/VinnsEdesigner/libv/go"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/configscan"
)

// The signing path. A document is rendered, scanned for secrets, then signed;
// the scan runs before the signature so an unsigned refusal is what an operator
// sees.

// DocumentSigner signs a rendered document and returns the signed text. A
// function, not a concrete signer, so the server has exactly one signing key.
type DocumentSigner func(document string) (string, error)

// CompileAndSign compiles a manifest, renders it, refuses a config carrying a
// secret, and signs the result.
func CompileAndSign(m *Manifest, in Injection, sign DocumentSigner) (string, error) {
	if sign == nil {
		return "", errNoSigner
	}

	cfg, err := Compile(m, in)
	if err != nil {
		return "", err
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

	// The signed document is re-parsed with the format's own parser before it is
	// returned: a signature over text the parser rejects is a signature over
	// nothing the device can read.
	if _, err := libv.Parse(signed); err != nil {
		return "", err
	}

	return signed, nil
}
