package deploy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	libv "github.com/VinnsEdesigner/libv/go"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/appsecret"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/crypto"
)

// The secret envelope's server side: set, list, unset, and the device fetch.
//
// The value is wrapped to the device's public key before it reaches storage, so
// the plaintext exists only in the request and in the device's memory. Nothing
// here logs a value, returns one, or writes one to a place a GET can reach
//.

// SecretError is a refusal naming the app and key. The CLI prints it verbatim.
type SecretError struct {
	App     string
	Key     string
	Message string
}

// Error implements error.
func (e *SecretError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("secret %s/%s: %s", e.App, e.Key, e.Message)
	}

	return fmt.Sprintf("secret %s: %s", e.App, e.Message)
}

// ErrSecretNotSet is returned when a config names a secret the device has not
// been given. It is separate from a plain SecretError so the handler can answer
// 404 rather than 400: the request is well-formed, the value simply is not there.
var ErrSecretNotSet = errors.New("secret is not set")

// ErrNoSecretKey is returned when a device has no secret key registered, so a value cannot
// be wrapped to it.
//
// It is a sentinel rather than a plain error because the condition is a property of the
// device, not a server fault: the caller asked correctly, and the answer is that this device
// cannot receive secrets yet. Without it the error fell through to a 500, telling the user
// the server had broken when the device simply had no key.
// ErrNoSecretKey is the domain sentinel, re-exported so a deploy caller names it once.
var ErrNoSecretKey = appsecret.ErrNoSecretKey

// SecretService sets, lists and removes app secrets.
type SecretService struct {
	secrets  appsecret.Repository
	keyStore DeviceSecretKeyLookup
	// now is injectable so a test can pin timestamps.
	now func() time.Time
}

// DeviceSecretKeyLookup resolves the public key a secret is wrapped to. It is
// the device's PURPOSE_DECRYPT key, distinct from its identity key.
type DeviceSecretKeyLookup interface {
	SecretPublicKey(ctx context.Context, deviceID string) (string, error)
}

// NewSecretService builds a SecretService.
func NewSecretService(secrets appsecret.Repository, keyStore DeviceSecretKeyLookup) *SecretService {
	return &SecretService{secrets: secrets, keyStore: keyStore, now: time.Now}
}

// Set wraps value to the device's key and stores it, returning the new
// generation. The plaintext is not retained anywhere.
func (s *SecretService) Set(ctx context.Context, deviceID, app, key, value string) (int64, error) {
	if err := validateSecretName(app, key); err != nil {
		return 0, err
	}

	if value == "" {
		return 0, &SecretError{App: app, Key: key, Message: "value is empty"}
	}

	publicKey, err := s.keyStore.SecretPublicKey(ctx, deviceID)
	if err != nil {
		return 0, fmt.Errorf("resolve device key for %s: %w", deviceID, err)
	}

	ciphertext, err := crypto.WrapSecret(publicKey, []byte(value))
	if err != nil {
		// The wrap failure names the device's key material, not anything the caller
		// controls, so the detail goes to the log and the caller gets the reason.
		return 0, &SecretError{App: app, Key: key, Message: "the device's key could not wrap the value"}
	}

	record := &appsecret.Record{
		DeviceID:   deviceID,
		App:        app,
		Key:        key,
		Ciphertext: ciphertext,
		UpdatedAt:  s.now(),
	}

	generation, err := s.secrets.Put(ctx, record)
	if err != nil {
		return 0, err
	}

	return generation, nil
}

// List returns the key names and generation for an app, never a value.
func (s *SecretService) List(ctx context.Context, deviceID, app string) ([]*appsecret.Record, error) {
	return s.secrets.List(ctx, deviceID, app)
}

// Unset removes a secret.
func (s *SecretService) Unset(ctx context.Context, deviceID, app, key string) error {
	if err := validateSecretName(app, key); err != nil {
		return err
	}

	return s.secrets.Delete(ctx, appsecret.Name{DeviceID: deviceID, App: app, Key: key})
}

// FetchForDevice returns the ciphertext for one key, but only when the current
// signed config names that key for that app. A device cannot ask for a secret
// its config does not reference, so the config is the gate and not the
// device's own claim.
func (s *SecretService) FetchForDevice(ctx context.Context, deviceID, app, key, currentConfig string) (*appsecret.Record, error) {
	if !configNamesSecret(currentConfig, app, key) {
		return nil, &SecretError{
			App:     app,
			Key:     key,
			Message: "the current signed config does not name this secret for this app",
		}
	}

	record, err := s.secrets.Get(ctx, appsecret.Name{DeviceID: deviceID, App: app, Key: key})
	if errors.Is(err, appsecret.ErrNotFound) {
		return nil, fmt.Errorf("%w: %s/%s", ErrSecretNotSet, app, key)
	}

	if err != nil {
		return nil, err
	}

	return record, nil
}

// configNamesSecret reports whether the signed config names `key` as a secret
// of `app`. It parses the document with the format's own parser and reads the
// app's `secret NAME` nodes, rather than matching the text: a substring search
// would match a comment or another app's node.
func configNamesSecret(document, app, key string) bool {
	doc, err := libv.Parse(document)
	if err != nil {
		return false
	}

	for _, appNode := range doc.Root.Find("device.apps.app") {
		if len(appNode.Tokens) < 2 || appNode.Tokens[1] != app {
			continue
		}

		for _, child := range appNode.Children {
			if child.Key() == "secret" && len(child.Tokens) > 1 && child.Tokens[1] == key {
				return true
			}
		}
	}

	return false
}

// validateSecretName applies the manifest's own app-name rule, so a secret can only be
// stored under an app a manifest could declare.
//
// Without the app check a secret could be filed under a name that can never appear in a
// manifest, which makes it unreachable: the device's config is the gate for a fetch, and
// no config will ever name that app. It would sit in the table forever, looking set.
func validateSecretName(app, key string) error {
	if strings.TrimSpace(app) == "" {
		return &SecretError{Message: "app is required"}
	}

	if !appNamePattern.MatchString(app) || len(app) > maxAppNameLen {
		return &SecretError{
			Message: fmt.Sprintf("app %q must match %s and be at most %d characters", app, appNameRe, maxAppNameLen),
		}
	}

	if strings.TrimSpace(key) == "" {
		return &SecretError{App: app, Message: "key is required"}
	}

	if strings.ContainsAny(key, "= \t\n") {
		return &SecretError{App: app, Key: key, Message: "key must not contain whitespace or '='"}
	}

	return nil
}
