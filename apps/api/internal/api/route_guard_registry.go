package api

import (
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/permission"
)

// guardStore names which grant store a route guard consults. A route declares
// its context by which group it is registered in; the store records that so a
// contract test can assert the defaults cover every guard and that the two
// axes stay orthogonal.
type guardStore string

const (
	// guardStoreOrg consults the caller's membership in the selected org.
	guardStoreOrg guardStore = "org"
	// guardStoreInstance consults the caller's deployment-wide InstanceRole.
	guardStoreInstance guardStore = "instance"
)

// scopeGuard is one (store, action, scope) a registered route requires. It is
// recorded when the guard is built, so the registry reflects the code rather
// than a hand-maintained list that can drift.
type scopeGuard struct {
	Store  guardStore
	Action permission.Action
	Scope  string
}

// recordGuard appends a route's requirement to the server's guard registry.
// Registration happens once, at startup, before the server serves traffic, so
// no lock is needed.
func (s *Server) recordGuard(store guardStore, action permission.Action, scope string) {
	s.scopeGuards = append(s.scopeGuards, scopeGuard{Store: store, Action: action, Scope: scope})
}

// ScopeGuards returns a copy of the registered route guards. It exists for the
// authorization contract test: the test walks what was actually registered
// instead of restating it.
func (s *Server) ScopeGuards() []scopeGuard {
	out := make([]scopeGuard, len(s.scopeGuards))
	copy(out, s.scopeGuards)

	return out
}
