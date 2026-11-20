package wire

import (
	"context"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/organization"
)

// orgGetter is the organisation read the name adapter needs.
type orgGetter interface {
	GetOrganization(ctx context.Context, orgID string) (*organization.Organization, error)
}

// orgNameAdapter resolves an organisation's name from its id for the provisioning sync
// engine, so an org file's directory can be checked against the repository's own
// organisation.
type orgNameAdapter struct {
	orgs orgGetter
}

// newOrgNameAdapter builds the adapter. A nil lookup yields a nil adapter, so the caller
// skips wiring rather than installing a check that always passes.
func newOrgNameAdapter(orgs orgGetter) *orgNameAdapter {
	if orgs == nil {
		return nil
	}

	return &orgNameAdapter{orgs: orgs}
}

// OrgName returns the organisation's name.
func (a *orgNameAdapter) OrgName(ctx context.Context, orgID string) (string, error) {
	org, err := a.orgs.GetOrganization(ctx, orgID)
	if err != nil {
		return "", err
	}

	return org.Name, nil
}
