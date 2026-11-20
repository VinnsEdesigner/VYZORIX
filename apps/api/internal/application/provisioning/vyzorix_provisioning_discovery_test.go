package provisioning

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	domain "github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/provisioning"
)

// TestSurveyReportsUnmanagedDeviceDirs proves full discovery lists device
// directories that hold no manifest, which is what the wizard's step 3 needs: what
// the repository contains, not only what it would apply.
func TestSurveyReportsUnmanagedDeviceDirs(t *testing.T) {
	root := writeTree(t, map[string]string{
		"devices/managed/vyzorix.toml": validManifest,
		"devices/empty/.keep":          "",
	})

	survey, err := SurveyRepository(root, "/")
	if err != nil {
		t.Fatalf("SurveyRepository: %v", err)
	}

	if len(survey.Manifests) != 1 {
		t.Fatalf("manifests = %d, want 1", len(survey.Manifests))
	}
	if len(survey.Unmanaged) != 1 || survey.Unmanaged[0].DeviceID != "empty" {
		t.Fatalf("unmanaged = %+v, want exactly the 'empty' device", survey.Unmanaged)
	}
}

// TestSurveyDoesNotMisreportBrokenManifest is the subtle case: a device directory
// whose manifest exists but fails to parse is MANAGED — the plan reports the parse
// error. Listing it as unmanaged would tell the user the repository does not manage
// a device it does.
func TestSurveyDoesNotMisreportBrokenManifest(t *testing.T) {
	root := writeTree(t, map[string]string{
		// A symlink is refused by discovery, so this manifest has an error.
		"devices/broken/.keep": "",
	})

	if err := os.Symlink("/etc/hostname", filepath.Join(root, "devices", "broken", "vyzorix.toml")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	survey, err := SurveyRepository(root, "/")
	if err != nil {
		t.Fatalf("SurveyRepository: %v", err)
	}

	if len(survey.Manifests) != 1 || survey.Manifests[0].Err == nil {
		t.Fatalf("manifests = %+v, want one with an error", survey.Manifests)
	}
	if len(survey.Unmanaged) != 0 {
		t.Fatalf("unmanaged = %+v, want none (the device has a manifest)", survey.Unmanaged)
	}
}

// TestSurveySkipsSymlinkedDeviceDir proves a symlinked device directory is not
// enumerated, consistent with discovery refusing symlinks.
func TestSurveySkipsSymlinkedDeviceDir(t *testing.T) {
	root := writeTree(t, map[string]string{"devices/real/vyzorix.toml": validManifest})

	target := filepath.Join(root, "elsewhere")
	if err := os.MkdirAll(target, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(root, "devices", "linked")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	survey, err := SurveyRepository(root, "/")
	if err != nil {
		t.Fatalf("SurveyRepository: %v", err)
	}
	if len(survey.Unmanaged) != 0 {
		t.Fatalf("unmanaged = %+v, want none (a symlinked dir is not part of the tree)", survey.Unmanaged)
	}
}

// TestDiscoverWritesNothing is the guarantee the wizard depends on: a discovery read
// must not create a resource, a job, or touch a device.
func TestDiscoverWritesNothing(t *testing.T) {
	root := writeTree(t, map[string]string{
		"devices/device-a/vyzorix.toml": validManifest,
		"devices/unmanaged/.keep":       "",
	})

	resources := newFakeResources()
	jobs := newFakeJobs()
	dep := &fakeDeployer{}
	s := newSyncServiceForTest(t, root, resources, jobs, dep)

	discovery, err := s.Discover(context.Background(), testRepo())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(discovery.Items) != 1 || discovery.Items[0].Action != ActionCreate {
		t.Fatalf("items = %+v, want one create", discovery.Items)
	}
	if len(discovery.Unmanaged) != 1 {
		t.Fatalf("unmanaged = %+v, want one", discovery.Unmanaged)
	}

	if len(dep.calls) != 0 {
		t.Fatalf("discovery deployed %d manifests", len(dep.calls))
	}
	if resources.upserts != 0 {
		t.Fatalf("discovery wrote %d resources", resources.upserts)
	}
	if len(jobs.jobs) != 0 {
		t.Fatalf("discovery recorded %d jobs", len(jobs.jobs))
	}
}

// TestDiscoverMarksManagedDevices proves the view distinguishes a device this
// repository already owns from one it does not, which is the conflict signal step 3
// surfaces before any write.
func TestDiscoverMarksManagedDevices(t *testing.T) {
	root := writeTree(t, map[string]string{
		"devices/device-a/vyzorix.toml": validManifest,
	})

	resources := newFakeResources()
	s := newSyncServiceForTest(t, root, resources, newFakeJobs(), &fakeDeployer{})

	// First sync adopts the device.
	if _, err := s.Sync(context.Background(), testRepo(), SyncOptions{}); err != nil {
		t.Fatalf("Sync: %v", err)
	}

	discovery, err := s.Discover(context.Background(), testRepo())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(discovery.Items) != 1 {
		t.Fatalf("items = %d, want 1", len(discovery.Items))
	}
	item := discovery.Items[0]
	if item.Resource == nil || item.Resource.RepoID != testRepo().ID {
		t.Fatalf("item = %+v, want it marked as owned by this repository", item)
	}
	if item.Action != ActionUnchanged {
		t.Fatalf("action = %s, want unchanged for an already-applied manifest", item.Action)
	}
}

// TestDiscoverConflictsWithAnotherOwner proves a device another repository owns is
// reported as a conflict, never as something this repository will apply.
func TestDiscoverConflictsWithAnotherOwner(t *testing.T) {
	root := writeTree(t, map[string]string{"devices/device-a/vyzorix.toml": validManifest})

	resources := newFakeResources()
	resources.seed(&domain.Resource{
		ID: "r-other", RepoID: "repo-other", DeviceID: "device-a",
		Path: "devices/device-a/vyzorix.toml", State: domain.ResourceApplied,
	})

	s := newSyncServiceForTest(t, root, resources, newFakeJobs(), &fakeDeployer{})

	discovery, err := s.Discover(context.Background(), testRepo())
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	if len(discovery.Items) != 1 || discovery.Items[0].Action != ActionConflict {
		t.Fatalf("items = %+v, want one conflict", discovery.Items)
	}
}

// TestDiscoverRejectsTwoManifestsForOneDevice is the guard for an ambiguous layout.
//
// Discovery accepts both vyzorix.toml and vyzorix.v, so a device directory holding both
// produced two resources for one device. The second deploy then hit the one-owner index
// and failed with "device is owned by another repository" — naming no owner, because the
// owner was this same repository. Both files are now flagged with a message that says
// what is actually wrong.
func TestDiscoverRejectsTwoManifestsForOneDevice(t *testing.T) {
	root := writeTree(t, map[string]string{
		"devices/device-a/vyzorix.toml": "a = 1\n",
		"devices/device-a/vyzorix.v":    "a = 1\n",
	})

	manifests, err := Discover(root, "/")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(manifests) != 2 {
		t.Fatalf("got %d manifests, want 2 (both files are still reported)", len(manifests))
	}

	for _, m := range manifests {
		if m.Err == nil {
			t.Fatalf("%s was accepted; a device must declare exactly one manifest", m.Path)
		}
		if !strings.Contains(m.Err.Error(), "exactly one") {
			t.Fatalf("error does not explain the ambiguity: %v", m.Err)
		}
	}
}

// TestDiscoverAcceptsOneManifestPerDevice proves the duplicate rule does not fire for the
// normal layout, including two different devices each with one manifest.
func TestDiscoverAcceptsOneManifestPerDevice(t *testing.T) {
	root := writeTree(t, map[string]string{
		"devices/device-a/vyzorix.toml": "a = 1\n",
		"devices/device-b/vyzorix.v":    "b = 1\n",
	})

	manifests, err := Discover(root, "/")
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	for _, m := range manifests {
		if m.Err != nil {
			t.Fatalf("%s was rejected: %v", m.Path, m.Err)
		}
	}
}
