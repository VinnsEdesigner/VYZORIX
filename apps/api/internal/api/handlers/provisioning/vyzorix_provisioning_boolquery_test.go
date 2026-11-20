package provisioninghandler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestBoolQueryFailsClosed is the guard for a dangerous default.
//
// dry_run was parsed as `c.Query("dry_run") == "true"`, so "?dry_run=1" or "?dry_run=TRUE"
// read as false and ran a REAL sync when the caller asked for a preview. A preview request
// that silently applies to the fleet is the worst possible misread, so an unrecognised
// value is now rejected rather than treated as false.
func TestBoolQueryFailsClosed(t *testing.T) {
	cases := []struct {
		query   string
		wantVal bool
		wantOK  bool
	}{
		{"", false, true},
		{"?dry_run=true", true, true},
		{"?dry_run=TRUE", true, true},
		{"?dry_run=1", true, true},
		{"?dry_run=false", false, true},
		{"?dry_run=0", false, true},
		{"?dry_run=yes", false, false},
		{"?dry_run=maybe", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/"+tc.query, nil)

			val, ok := boolQuery(c, "dry_run")
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && val != tc.wantVal {
				t.Fatalf("value = %v, want %v", val, tc.wantVal)
			}
		})
	}
}
