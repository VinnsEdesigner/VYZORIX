package deploy

import (
	"regexp"
	"strings"

	libv "github.com/VinnsEdesigner/libv/go"
)

// The structural no-secret rule (VYZORIX_DEPLOY_CONTRACT.md §5.2).
//
// The shipped check scanned every line for the words secret, token, key, hmac
// and password. That cannot survive a config that carries apps: the schema's own
// keys — the `secret NAME` node, the `apps` container — contain those words by
// definition, so the rule refused the document it exists to protect. This walk
// is the replacement: schema keys are exempt by path, and the values that could
// actually carry a secret are inspected.

// secretKeyPattern matches a key that claims to hold a secret. It is checked
// only against env keys, which is the field the rule exists for.
var secretKeyPattern = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key|private[_-]?key|credential)`)

// value patterns for the shapes a secret takes. Each is anchored or bounded so
// an ordinary value does not trip it: a short hostname is not a base64 run, and
// a URL with no credentials is not a user:pass URL.
var (
	pemBlockPattern = regexp.MustCompile(`-----BEGIN [A-Z ]+-----`)
	jwtPattern      = regexp.MustCompile(`^eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}$`)
	hexRunPattern   = regexp.MustCompile(`^[0-9a-fA-F]{40,}$`)
	base64Run       = regexp.MustCompile(`^[A-Za-z0-9+/]{40,}={0,2}$`)
	userPassURL     = regexp.MustCompile(`^[a-z][a-z0-9+.-]*://[^/@\s:]+:[^/@\s]+@`)
)

// SecretScanError names the key that carried a secret.
type SecretScanError struct {
	Key    string
	Reason string
}

// Error implements error.
func (e *SecretScanError) Error() string {
	return "config must not carry a secret: " + e.Key + " " + e.Reason
}

// ScanConfigForSecrets walks a rendered config and refuses anything that looks
// like a secret value. It is run before signing, so an unsigned refusal is what
// the operator sees.
func ScanConfigForSecrets(document string) error {
	doc, err := libv.Parse(document)
	if err != nil {
		return &SecretScanError{Key: "<document>", Reason: "does not parse: " + err.Error()}
	}

	apps := doc.Root.Find("device.apps.app")

	for _, app := range apps {
		if err := scanAppNode(app); err != nil {
			return err
		}
	}

	return nil
}

// scanAppNode inspects one app's children. Only `env` values and `env` keys are
// inspected by name; `secret` values are exempt because a name is a name.
func scanAppNode(app *libv.Node) error {
	for _, child := range app.Children {
		if len(child.Tokens) < 2 {
			continue
		}

		switch child.Key() {
		case "env":
			if err := scanEnvNode(child); err != nil {
				return err
			}
		case "secret":
			if err := scanSecretName(child.Tokens[1]); err != nil {
				return err
			}
		default:
			if looksLikeSecret(child.Tokens[1]) {
				return &SecretScanError{Key: child.Key(), Reason: "holds a value that looks like a secret"}
			}
		}
	}

	return nil
}

// scanEnvNode checks an `env KEY=VALUE` node: the key must not claim to hold a
// secret, and the value must not look like one.
func scanEnvNode(node *libv.Node) error {
	key, value, found := strings.Cut(node.Tokens[1], "=")
	if !found {
		return &SecretScanError{Key: "env", Reason: "malformed, expected KEY=VALUE"}
	}

	if secretKeyPattern.MatchString(key) {
		return &SecretScanError{Key: "env." + key, Reason: "names a secret; use [app.secrets] and vyzorix secrets set"}
	}

	if looksLikeSecret(value) {
		return &SecretScanError{Key: "env." + key, Reason: "holds a value that looks like a secret"}
	}

	return nil
}

// scanSecretName refuses a `secret NAME` whose value is a secret shape rather
// than a name. A name is not forty hex characters.
func scanSecretName(name string) error {
	if looksLikeSecret(name) {
		return &SecretScanError{Key: "secret", Reason: "carries what looks like a value, not a name"}
	}

	return nil
}

// looksLikeSecret reports whether a value has the shape of a credential.
func looksLikeSecret(value string) bool {
	if value == "" {
		return false
	}

	return pemBlockPattern.MatchString(value) ||
		jwtPattern.MatchString(value) ||
		hexRunPattern.MatchString(value) ||
		base64Run.MatchString(value) ||
		userPassURL.MatchString(value)
}
