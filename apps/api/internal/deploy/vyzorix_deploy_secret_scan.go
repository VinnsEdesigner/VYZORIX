package deploy

import (
	"regexp"
	"strings"

	libv "github.com/VinnsEdesigner/libv/go"
)

// secretKeyPattern matches a key that claims to hold a secret.
var secretKeyPattern = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key|private[_-]?key|credential)`)

// strongSecretShape matches a value that is a credential regardless of context:
// a PEM block, a JWT, or a URL carrying user:pass. Bare hex and base64 are
// deliberately absent — an artifact's sha256 is 64 hex characters and would
// trip them.
var strongSecretShape = regexp.MustCompile(
	`-----BEGIN [A-Z ]+-----` +
		`|^eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}$` +
		`|^[a-z][a-z0-9+.-]*://[^/@\s:]+:[^/@\s]+@`,
)

// envSecretShape adds the shapes that are only a secret in an env value, where
// a digest is not expected.
var envSecretShape = regexp.MustCompile(`^[0-9a-fA-F]{40,}$|^[A-Za-z0-9+/]{40,}={0,2}$`)

// SecretScanError names the node that carried a secret.
type SecretScanError struct {
	Key    string
	Reason string
}

// Error implements error.
func (e *SecretScanError) Error() string {
	return "config must not carry a secret: " + e.Key + " " + e.Reason
}

// ScanConfigForSecrets walks a rendered config and refuses a value that looks
// like a credential. Schema keys are exempt by path — the `secret NAME` node and
// the `apps` container contain those words by design — but every value is
// inspected, including nested ones.
func ScanConfigForSecrets(document string) error {
	doc, err := libv.Parse(document)
	if err != nil {
		return &SecretScanError{Key: "<document>", Reason: "does not parse: " + err.Error()}
	}

	for _, app := range doc.Root.Find("device.apps.app") {
		if err := scanNode(app); err != nil {
			return err
		}
	}

	return nil
}

// scanNode inspects a node's value and recurses into its children.
func scanNode(node *libv.Node) error {
	if err := scanNodeValue(node); err != nil {
		return err
	}

	for _, child := range node.Children {
		if err := scanNode(child); err != nil {
			return err
		}
	}

	return nil
}

// scanNodeValue applies the rules to one node's own value. `env` is split into
// KEY=VALUE; `secret` holds a name, so only a strong shape there is a mistake.
func scanNodeValue(node *libv.Node) error {
	if len(node.Tokens) < 2 {
		return nil
	}

	switch node.Key() {
	case "env":
		return scanEnvValue(node.Tokens[1])
	case "secret":
		if strongSecretShape.MatchString(node.Tokens[1]) {
			return &SecretScanError{Key: "secret", Reason: "carries what looks like a value, not a name"}
		}
	default:
		if strongSecretShape.MatchString(node.Tokens[1]) {
			return &SecretScanError{Key: node.Key(), Reason: "holds a value that looks like a secret"}
		}
	}

	return nil
}

// scanEnvValue checks an `env KEY=VALUE` pair: the key must not name a secret,
// and the value must not look like one.
func scanEnvValue(token string) error {
	key, value, found := strings.Cut(token, "=")
	if !found {
		return &SecretScanError{Key: "env", Reason: "malformed, expected KEY=VALUE"}
	}

	if secretKeyPattern.MatchString(key) {
		return &SecretScanError{Key: "env." + key, Reason: "names a secret; use [app.secrets] and vyzorix secrets set"}
	}

	if strongSecretShape.MatchString(value) || envSecretShape.MatchString(value) {
		return &SecretScanError{Key: "env." + key, Reason: "holds a value that looks like a secret"}
	}

	return nil
}
