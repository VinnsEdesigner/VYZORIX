package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/responses"
)

// HeaderDeployCredential is the header a CI client presents a device-scoped
// deploy credential in. It is its own header
// rather than a bearer token so a credential can never be confused with an
// user API key, which reaches more than one device.
const HeaderDeployCredential = "X-Vyzorix-Deploy-Credential"

// ContextKeyDeployCredentialDevice is the device a deploy credential is bound to.
const ContextKeyDeployCredentialDevice = "deploy_credential_device"

// ContextKeyDeployCredentialOrg is the organization a deploy credential is bound
// to, taken from the credential rather than from a request header.
const ContextKeyDeployCredentialOrg = "deploy_credential_org"

// ContextKeyDeployCredentialAuthor is the user who minted the credential. It
// is the `memberId` a credential-initiated deploy records, so the config still
// names a human even when the deploy came from CI.
const ContextKeyDeployCredentialAuthor = "deploy_credential_author"

// DeployCredentialIdentity is what a resolved credential tells the request.
type DeployCredentialIdentity struct {
	DeviceID  string
	OrgID     string
	CreatedBy string
}

// DeployCredentialResolver resolves a presented credential to the identity it
// names. The deploy package's CredentialService satisfies the shape; the function
// is injected so this middleware does not import the feature.
type DeployCredentialResolver func(ctx context.Context, secret string) (DeployCredentialIdentity, bool)

// IsDeployPath reports whether a request path belongs to the deploy surface. The
// deploy credential is only ever valid there, so the other authenticators defer to
// it only for these paths: deferring on the header alone would let an unknown
// header value suppress the session and API-key checks on every tenant route.
func IsDeployPath(path string) bool {
	return path == "/v1/deploy" || strings.HasPrefix(path, "/v1/deploy/")
}

// DeployCredentialAuth authenticates a request that presents a deploy credential
// and binds it to the one device the credential names. It is scoped to the
// /v1/deploy group, so a credential can never widen past that group.
//
// The organization context is set from the credential, not from a request header:
// the credential is the authority on which tenant and which device may be
// touched, so a client cannot point it at another org by sending a header.
func DeployCredentialAuth(resolve DeployCredentialResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := c.GetHeader(HeaderDeployCredential)
		if secret == "" {
			// No deploy credential: another authenticator owns this request.
			c.Next()

			return
		}

		identity, ok := resolve(c.Request.Context(), secret)
		if !ok {
			responses.RespondStructuredAbort(c, http.StatusUnauthorized, "deploy credential is not valid")

			return
		}

		// The path may name a device; it must be the credential's device.
		if pathDevice := c.Param("imei"); pathDevice != "" && pathDevice != identity.DeviceID {
			responses.RespondStructuredAbort(c, http.StatusForbidden,
				"deploy credential is scoped to a different device")

			return
		}

		c.Set("auth_type", "deploy_credential")
		c.Set(ContextKeyDeployCredentialDevice, identity.DeviceID)
		c.Set(ContextKeyDeployCredentialOrg, identity.OrgID)
		c.Set(ContextKeyDeployCredentialAuthor, identity.CreatedBy)
		c.Set(ContextKeyOrganizationID, identity.OrgID)
		c.Next()
	}
}

// DeployCredentialFromContext returns the identity a deploy credential
// authenticated, or ok=false when the request was not credential-authenticated.
func DeployCredentialFromContext(c *gin.Context) (DeployCredentialIdentity, bool) {
	if c.GetString("auth_type") != "deploy_credential" {
		return DeployCredentialIdentity{}, false
	}

	return DeployCredentialIdentity{
		DeviceID:  c.GetString(ContextKeyDeployCredentialDevice),
		OrgID:     c.GetString(ContextKeyDeployCredentialOrg),
		CreatedBy: c.GetString(ContextKeyDeployCredentialAuthor),
	}, true
}
