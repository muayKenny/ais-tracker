package store

import (
	"sync"

	"github.com/kenshef/ais-tracker/apps/backend/internal/vessel"
)

// maxTrailLength bounds each vessel's Trail — a rolling window, not a
// persisted history. Picked arbitrarily for now; revisit once entry
// detection and trail rendering actually need a specific length.
const maxTrailLength = 20

// Store is the Current State role from system-design.md: an in-memory,
// concurrent-safe map of every vessel currently being tracked, keyed by
// MMSI. One writer (ingestion, via Upsert), many readers (Snapshot).
type Store struct {
	mu      sync.RWMutex
	vessels map[int64]*vessel.Vessel
}

func New() *Store {
	return &Store{
		vessels: make(map[int64]*vessel.Vessel),
	}
}

// Upsert records a new ping for the given vessel, creating it if this is
// the first time it's been seen.
func (s *Store) Upsert(mmsi int64, name string, ping vessel.Ping) {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.vessels[mmsi]
	if !ok {
		v = &vessel.Vessel{MMSI: mmsi}
		s.vessels[mmsi] = v
	}
	// aisstream doesn't guarantee ShipName is populated on every message —
	// don't let a blank name overwrite one we already know.
	if name != "" {
		v.Name = name
	}
	v.Trail = appendTrail(v.Trail, ping)
}

// Snapshot returns every currently-tracked vessel. Safe to keep reading
// after the call returns, even while Upsert keeps running concurrently —
// each returned Vessel is a value copy, and appendTrail never mutates a
// Trail's backing array in place, so a snapshot's array is never written
// to again after it's handed out.
func (s *Store) Snapshot() []vessel.Vessel {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]vessel.Vessel, 0, len(s.vessels))
	for _, v := range s.vessels {
		out = append(out, *v)
	}
	return out
}

// appendTrail always returns a freshly allocated slice rather than
// extending trail's backing array in place. This matters because Snapshot
// copies a Vessel's Trail slice header without deep-copying its backing
// array — if Upsert reused that array on a later write, a reader holding
// an older snapshot could race against it on the same memory.
func appendTrail(trail []vessel.Ping, ping vessel.Ping) []vessel.Ping {
	start := 0
	if len(trail)+1 > maxTrailLength {
		start = len(trail) + 1 - maxTrailLength
	}

	next := make([]vessel.Ping, 0, maxTrailLength)
	next = append(next, trail[start:]...)
	next = append(next, ping)
	return next
}
