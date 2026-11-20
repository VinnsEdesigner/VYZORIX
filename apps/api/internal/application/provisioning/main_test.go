package provisioning

import (
	"os"
	"testing"
)

// TestMain lets the test binary stand in for the server binary.
//
// The git fetcher configures itself as a credential helper by re-invoking its own
// executable, so an end-to-end clone against an authenticated remote needs the
// test binary to answer that call exactly as api_main does.
func TestMain(m *testing.M) {
	if RunCredentialHelper(os.Args[1:]) {
		os.Exit(0)
	}

	os.Exit(m.Run())
}
