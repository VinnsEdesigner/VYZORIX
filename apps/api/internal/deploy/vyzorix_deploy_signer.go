package deploy

import (
	libv "github.com/VinnsEdesigner/libv/go"
)

// The signing path. A document is rendered, scanned for secrets, then signed;
// the scan runs before the signature so an unsigned refusal is what an operator
// sees, and the render is a pure function of the config so the same manifest
// always signs to the same bytes (contract §5.1, §14).

// DocumentSigner signs a rendered document and returns the signed text. It is a
// function rather than a concrete signer so the server has exactly one signing
// key: passing a second type here would let the config and the command frames be
// signed by different keys.
type DocumentSigner func(document string) (string, error)

// CompileAndSign compiles a manifest, renders it, refuses a config that carries
// a secret, and signs the result.
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

	if scanErr := ScanConfigForSecrets(document); scanErr != nil {
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
