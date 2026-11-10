package deploy

import (
	"strings"
	"sync"
)

// The app catalog: the names of known apps, seeded at boot from
// the provisioning file's `catalog_apps` section. A manifest naming an app in the
// catalog is ordinary; one naming anything else is permitted but marked custom,
// so the console can flag an app nobody reviewed. The catalog is a review aid,
// not a gate: it never refuses a deploy.
//
// It is written once at boot, before the server serves, and read on every deploy.
// The lock is what makes that ordering an invariant rather than a hope.
type Catalog struct {
	known map[string]struct{}
	mu    sync.RWMutex
}

// NewCatalog builds a catalog from the names the provisioning file declares.
// Blank names are dropped: a name that matches nothing is a typo, not a rule.
func NewCatalog(names []string) *Catalog {
	c := &Catalog{known: make(map[string]struct{}, len(names))}
	c.Add(names)

	return c
}

// Add records names as known. It is the boot-time seeding path.
func (c *Catalog) Add(names []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, name := range names {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			c.known[trimmed] = struct{}{}
		}
	}
}

// Known reports whether the app is in the catalog.
func (c *Catalog) Known(name string) bool {
	if c == nil {
		return false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.known[name]

	return ok
}

// Mark classifies each app as known or custom, in the order given. A nil
// catalog marks everything custom, which is the honest answer when no
// provisioning file declared a catalog.
func (c *Catalog) Mark(names []string) []AppMarking {
	markings := make([]AppMarking, 0, len(names))

	for _, name := range names {
		markings = append(markings, AppMarking{Name: name, Custom: !c.Known(name)})
	}

	return markings
}

// AppMarking is one app's catalog classification.
type AppMarking struct {
	Name   string
	Custom bool
}
