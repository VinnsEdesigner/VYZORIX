package api

import (
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestMain sets gin's global mode once, before any test runs.
//
// gin.SetMode writes a package-level variable with no synchronisation. Several
// tests in this package call it from helpers (newDeployFixture, engineFor) and run
// under t.Parallel(), so the write raced with itself and with the reads gin does on
// every request: the race detector reported it, and in principle it could leave the
// engine in debug mode for a test that expected otherwise.
//
// Pinning the mode here, once, on the single goroutine that runs before the test
// binary starts its parallel tests, removes the shared write entirely. Individual
// tests must not call SetMode; there is nothing left for them to change.
func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	os.Exit(m.Run())
}
