package api

import (
	"context"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/appsecret"
)

// secretReadinessAdapter narrows the app-secret repository to the name-only read the
// deploy service's apply-time check needs.
//
// It returns names and never ciphertext: the repository's List already omits the value,
// and this adapter keeps that property explicit at the boundary so the deploy package
// cannot accidentally depend on a richer read.
type secretReadinessAdapter struct {
	repo appsecret.Repository
}

// newSecretReadinessAdapter builds the adapter. A nil repository yields a nil adapter, so
// the caller skips wiring rather than installing a check that silently passes.
func newSecretReadinessAdapter(repo appsecret.Repository) *secretReadinessAdapter {
	if repo == nil {
		return nil
	}

	return &secretReadinessAdapter{repo: repo}
}

// SetSecretNames returns the names of the secrets stored for an app on a device.
func (a *secretReadinessAdapter) SetSecretNames(ctx context.Context, deviceID, app string) ([]string, error) {
	if a == nil || a.repo == nil {
		return nil, nil
	}

	records, err := a.repo.List(ctx, deviceID, app)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(records))
	for _, r := range records {
		names = append(names, r.Key)
	}

	return names, nil
}
