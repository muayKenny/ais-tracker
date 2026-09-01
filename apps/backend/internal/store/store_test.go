package store

import (
	"sync"
	"testing"

	"github.com/kenshef/ais-tracker/apps/backend/internal/vessel"
)

// TestUpsertAndSnapshotConcurrent is the actual proof behind Phase 2's
// roadmap checkpoint: "provable with a concurrent unit test (go test
// -race)." Many goroutines hammer Upsert on the same MMSI while also
// calling Snapshot concurrently.
func TestUpsertAndSnapshotConcurrent(t *testing.T) {
	s := New()

	const mmsi = int64(368373020)
	const writers = 8
	const writesPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(writers)
	for i := 0; i < writers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < writesPerGoroutine; j++ {
				s.Upsert(mmsi, "TEST VESSEL", vessel.Ping{Latitude: 1, Longitude: 2})
				_ = s.Snapshot()
			}
		}()
	}
	wg.Wait()

	snap := s.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("expected 1 tracked vessel, got %d", len(snap))
	}
	if snap[0].MMSI != mmsi {
		t.Fatalf("expected MMSI %d, got %d", mmsi, snap[0].MMSI)
	}
	if len(snap[0].Trail) != maxTrailLength {
		t.Fatalf("expected trail capped at %d, got %d", maxTrailLength, len(snap[0].Trail))
	}
}
