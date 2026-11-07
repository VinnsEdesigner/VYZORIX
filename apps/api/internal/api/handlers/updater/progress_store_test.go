package updater

import (
	"testing"
	"time"
)

// TestProgressStoreRecordsAndReads covers the happy path: a report is stored and
// read back with the percent derived from the bytes.
func TestProgressStoreRecordsAndReads(t *testing.T) {
	store := NewProgressStore(time.Minute)

	store.Record("dev-1", "app.apk", 25, 100)

	got, ok := store.Get("dev-1")
	if !ok {
		t.Fatal("a recorded device must be readable")
	}
	if got.Filename != "app.apk" {
		t.Errorf("filename = %q", got.Filename)
	}
	if got.Percent() != 25 {
		t.Errorf("percent = %d, want 25", got.Percent())
	}
}

// TestProgressStoreUnknownDeviceIsNotDownloading proves a device that never
// reported reads as "not downloading" rather than as an error or a zero percent.
func TestProgressStoreUnknownDeviceIsNotDownloading(t *testing.T) {
	store := NewProgressStore(time.Minute)

	if _, ok := store.Get("never-seen"); ok {
		t.Fatal("an unknown device must not report progress")
	}
}

// TestProgressStoreSweepsStaleReports is the reason the store has a TTL: a device
// that crashes mid-download would otherwise show a frozen percentage forever.
func TestProgressStoreSweepsStaleReports(t *testing.T) {
	store := NewProgressStore(time.Minute)

	// Drive time manually so the TTL is exercised without sleeping.
	now := time.Unix(0, 0)
	store.now = func() time.Time { return now }

	store.Record("dev-1", "app.apk", 50, 100)

	// Inside the TTL: live.
	now = now.Add(30 * time.Second)
	if _, ok := store.Get("dev-1"); !ok {
		t.Fatal("a report inside the TTL must stay live")
	}

	// Past the TTL: swept, and reads as not downloading.
	now = now.Add(2 * time.Minute)
	if _, ok := store.Get("dev-1"); ok {
		t.Fatal("a report past the TTL must not be live")
	}
}

// TestProgressStoreSweepRemovesStale proves the ticker sweep drops old entries so
// the map cannot grow without bound.
func TestProgressStoreSweepRemovesStale(t *testing.T) {
	store := NewProgressStore(time.Minute)
	now := time.Unix(0, 0)
	store.now = func() time.Time { return now }

	store.Record("old", "app.apk", 1, 100)
	now = now.Add(2 * time.Minute)
	store.Record("fresh", "app.apk", 1, 100)

	if dropped := store.Sweep(); dropped != 1 {
		t.Fatalf("sweep dropped %d, want 1", dropped)
	}
	if _, ok := store.Get("old"); ok {
		t.Error("the stale entry must be gone")
	}
	if _, ok := store.Get("fresh"); !ok {
		t.Error("the fresh entry must survive")
	}
}

// TestProgressStoreIgnoresEmptyDeviceID proves a report with no id is dropped
// rather than stored under a key that can never be read.
func TestProgressStoreIgnoresEmptyDeviceID(t *testing.T) {
	store := NewProgressStore(time.Minute)

	store.Record("", "app.apk", 1, 100)

	if _, ok := store.Get(""); ok {
		t.Fatal("an empty device id must not be stored")
	}
}

// TestPercentDoesNotDivideByZero proves a report without a total size reports 0,
// not a panic or a bogus number.
func TestPercentDoesNotDivideByZero(t *testing.T) {
	p := DownloadProgress{BytesLoaded: 500, TotalBytes: 0}

	if p.Percent() != 0 {
		t.Fatalf("percent = %d, want 0 for an unknown total", p.Percent())
	}
}
