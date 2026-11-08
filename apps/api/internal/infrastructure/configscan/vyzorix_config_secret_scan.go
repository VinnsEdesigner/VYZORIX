// Package configscan is the structural no-secret check for a rendered vyzorix.v
// config (VYZORIX_DEPLOY_CONTRACT.md §5.2). A config is transmitted and signed,
// so it must never carry a secret value; it may only name one.
//
// This is the single check both config paths use — registration and deploy. An
// earlier line scanner refused the word "secret" anywhere, which refuses the
// schema's own `secret NAME` nodes: the field that exists so a config can name a
// secret without carrying one.
package configscan

import (
	"regexp"
	"strings"

	libv "github.com/VinnsEdesigner/libv/go"
)

// secretKeyPattern matches a key that claims to hold a secret. Bare `key` is
// deliberately absent: an env var named KEY is ordinary configuration, so only
// the qualified forms are matched.
var secretKeyPattern = regexp.MustCompile(`(?i)(password|passwd|secret|token|hmac|api[_-]?key|private[_-]?key|credential)`)

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

// Error names the node that carried a secret.
type Error struct {
	Key    string
	Reason string
}

// Error implements error.
func (e *Error) Error() string {
	return "config must not carry a secret: " + e.Key + " " + e.Reason
}

// Scan walks a rendered config and refuses a value that looks like a credential,
// at every depth. It parses with the format's own parser, so it sees the same
// nodes the device will.
func Scan(document string) error {
	doc, err := libv.Parse(document)
	if err != nil {
		return &Error{Key: "<document>", Reason: "does not parse: " + err.Error()}
	}

	return scanNode(doc.Root, false)
}

// scanNode inspects a node's value and recurses. inApp is true inside an `app`
// node, the only place the schema's `secret NAME` node is legal.
func scanNode(node *libv.Node, inApp bool) error {
	if err := scanNodeValue(node, inApp); err != nil {
		return err
	}

	childInApp := inApp || node.Key() == "app"

	for _, child := range node.Children {
		if err := scanNode(child, childInApp); err != nil {
			return err
		}
	}

	return nil
}

// scanNodeValue applies the rules to one node's own value. A `secret` node is
// the schema's name-only field, but only inside an app; anywhere else a `secret`
// key is an attempt to carry one and is refused like any other secret-naming key.
func scanNodeValue(node *libv.Node, inApp bool) error {
	key := node.Key()

	if key == "env" {
		if len(node.Tokens) < 2 {
			return nil
		}

		return scanEnvValue(node.Tokens[1])
	}

	if key == "secret" && inApp {
		// A name, not a value: only a strong shape is a mistake here.
		if len(node.Tokens) > 1 && strongSecretShape.MatchString(strings.Join(node.Tokens[1:], " ")) {
			return &Error{Key: "secret", Reason: "carries what looks like a value, not a name"}
		}

		return nil
	}

	if secretKeyPattern.MatchString(key) {
		return &Error{Key: key, Reason: "names a secret; a config may name one, not hold one"}
	}

	// The whole token run, not just Tokens[1]: a PEM header has spaces, so libv
	// splits it and the marker is not in the first token alone.
	if len(node.Tokens) > 1 && strongSecretShape.MatchString(strings.Join(node.Tokens[1:], " ")) {
		return &Error{Key: key, Reason: "holds a value that looks like a secret"}
	}

	return nil
}

// scanEnvValue checks an `env KEY=VALUE` pair: the key must not name a secret,
// and the value must not look like one.
func scanEnvValue(token string) error {
	key, value, found := strings.Cut(token, "=")
	if !found {
		return &Error{Key: "env", Reason: "malformed, expected KEY=VALUE"}
	}

	if secretKeyPattern.MatchString(key) {
		return &Error{Key: "env." + key, Reason: "names a secret; use [app.secrets] and vyzorix secrets set"}
	}

	if strongSecretShape.MatchString(value) || envSecretShape.MatchString(value) {
		return &Error{Key: "env." + key, Reason: "holds a value that looks like a secret"}
	}

	return nil
}
