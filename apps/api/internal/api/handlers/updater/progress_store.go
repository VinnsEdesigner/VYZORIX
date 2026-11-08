package updater

import (
	"sync"
	"time"
)

// DownloadProgress is one device's in-flight OTA download.
type DownloadProgress struct {
	UpdatedAt   time.Time
	DeviceID    string
	Filename    string
	BytesLoaded int64
	TotalBytes  int64
}

// Percent is the download's completion, 0..100. A zero TotalBytes (the size was
// not sent) reports 0 rather than dividing by zero.
func (p DownloadProgress) Percent() int {
	if p.TotalBytes <= 0 {
		return 0
	}
	pct := int(p.BytesLoaded * 100 / p.TotalBytes)
	if pct > 100 {
		return 100
	}

	return pct
}

// ProgressStore keeps the latest download progress per device.
//
// In-memory and deliberately not durable: this answers "how far along is this
// device's update right now", which is meaningless after a restart, and a device
// re-reports on its next tick. That is the same lifetime as the PTY session
// registry, and for the same reason — a live thing, not a record.
//
// A device that stops reporting (crashed, killed, offline) would otherwise leave a
// stale "downloading 40%" entry forever, so records are swept by age: a read older
// than [ttl] is reported as not-downloading rather than as frozen progress.
type ProgressStore struct {
	byID map[string]DownloadProgress
	now  func() time.Time
	ttl  time.Duration
	mu   sync.Mutex
}

// NewProgressStore creates a store. A zero ttl takes the default.
func NewProgressStore(ttl time.Duration) *ProgressStore {
	if ttl <= 0 {
		ttl = DefaultProgressTTL
	}

	return &ProgressStore{
		byID: make(map[string]DownloadProgress),
		ttl:  ttl,
		now:  time.Now,
	}
}

// Record stores the latest progress for a device, replacing any prior entry.
//
// Progress is monotonic in practice but not enforced: a device that restarts its
// download legitimately reports a lower byte count, and refusing that would leave
// the server showing a number the device has moved past.
func (s *ProgressStore) Record(deviceID, filename string, loaded, total int64) {
	if deviceID == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.byID[deviceID] = DownloadProgress{
		DeviceID:    deviceID,
		Filename:    filename,
		BytesLoaded: loaded,
		TotalBytes:  total,
		UpdatedAt:   s.now(),
	}
}

// Get returns the live progress for a device. The second result is false when the
// device has no current download — either it never reported one, or the last report
// is older than the TTL and has been swept.
func (s *ProgressStore) Get(deviceID string) (DownloadProgress, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.byID[deviceID]
	if !ok {
		return DownloadProgress{}, false
	}

	if s.now().Sub(entry.UpdatedAt) > s.ttl {
		// Stale: the device stopped reporting. Drop it and report nothing rather
		// than a frozen percentage.
		delete(s.byID, deviceID)

		return DownloadProgress{}, false
	}

	return entry, true
}

// Sweep removes every entry older than the TTL and returns how many were dropped.
// Called on a ticker so a store full of abandoned downloads does not grow forever.
func (s *ProgressStore) Sweep() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := s.now().Add(-s.ttl)
	dropped := 0

	for id, entry := range s.byID {
		if entry.UpdatedAt.Before(cutoff) {
			delete(s.byID, id)
			dropped++
		}
	}

	return dropped
}

// DefaultProgressTTL is how long a report stays live without a refresh.
//
// Two minutes: long enough that a slow 64 MB download on a bad link is not swept
// between its (frequent) reports, short enough that a device which stops reporting
// stops showing as downloading.
const DefaultProgressTTL = 2 * time.Minute
