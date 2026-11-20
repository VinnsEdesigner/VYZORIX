package provisioninghandler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/api/adapters/response"
	svc "github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/provisioning"
	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// TestRepositoryErrorStatus pins the status each repository failure produces.
//
// The distinction matters to a caller: a malformed body is their fault and
// fixable in place (400), while a URL the server cannot reach is a value the
// request named but the server could not resolve (422). Collapsing both into 400
// tells the user to correct their syntax when the syntax was already correct.
func TestRepositoryErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name string
		err  error
		want int
	}{
		{"unreachable remote", fmt.Errorf("%w: host did not answer", svc.ErrRemoteUnreachable), http.StatusUnprocessableEntity},
		{"invalid input", fmt.Errorf("%w: name is required", svc.ErrInvalidRepository), http.StatusBadRequest},
		{"not found", domain.ErrRepositoryNotFound, http.StatusNotFound},
		{"unexpected", fmt.Errorf("boom"), http.StatusInternalServerError},
	}

	h := &RepositoryHandler{presenter: response.NewPresenter(nil, nil, nil)}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)

			h.writeRepositoryError(c, tc.err)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}
