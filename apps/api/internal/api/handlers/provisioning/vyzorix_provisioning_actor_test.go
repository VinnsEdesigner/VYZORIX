package provisioninghandler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/middleware"
	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/serviceaccount"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/user"
)

// TestActorFromResolvesEachCredential pins who gets blamed for a provisioning
// action. Getting this wrong is worse than not recording it: a token's automated
// change attributed to a person, or a person's change attributed to "system", both
// make the audit trail misleading rather than merely incomplete.
func TestActorFromResolvesEachCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("signed-in user", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set(middleware.ContextKeyUser, &user.User{ID: "u1", Email: "a@b.c"})

		got := actorFrom(c)
		if got.Kind != domain.ActorUser || got.ID != "u1" || got.Label != "a@b.c" {
			t.Fatalf("actor = %+v, want the user with email", got)
		}
	})

	t.Run("service account", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("auth_type", "service_account")
		c.Set(middleware.ContextKeyServiceAccountToken, &serviceaccount.Token{
			ID: "tok-1", Name: "ci-pipeline",
		})

		got := actorFrom(c)
		if got.Kind != domain.ActorServiceAccount || got.ID != "tok-1" || got.Label != "ci-pipeline" {
			t.Fatalf("actor = %+v, want the token, not a user", got)
		}
	})

	// The case that matters most: a service-account request whose context ALSO holds
	// a minimal user (the tenant API key middleware builds one). The token must win,
	// or an automated change is filed against a person.
	t.Run("service account wins over a minimal user", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("auth_type", "service_account")
		c.Set(middleware.ContextKeyServiceAccountToken, &serviceaccount.Token{ID: "tok-1", Name: "ci"})
		c.Set(middleware.ContextKeyUser, &user.User{ID: "u1", Email: "a@b.c"})

		got := actorFrom(c)
		if got.Kind != domain.ActorServiceAccount {
			t.Fatalf("actor = %+v, want the service account, never the user", got)
		}
	})

	// A service-account request whose token the middleware did not expose is still a
	// token, not a person and not blank.
	t.Run("service account without an exposed token", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set("auth_type", "service_account")

		got := actorFrom(c)
		if got.Kind != domain.ActorServiceAccount {
			t.Fatalf("actor = %+v, want a service-account attribution", got)
		}
		if got.ID == "" {
			t.Fatal("actor id must not be empty")
		}
	})
}
