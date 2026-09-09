package api

import (
	"sync"
	"testing"

	"github.com/kenshef/ais-tracker/apps/backend/internal/vessel"
)

// TestHubConcurrentRegisterBroadcast hammers Register/Unregister/Broadcast
// from many goroutines at once, under -race.
func TestHubConcurrentRegisterBroadcast(t *testing.T) {
	h := NewHub()

	const clients = 8
	const broadcasts = 50

	var wg sync.WaitGroup

	// Clients registering, briefly existing, then unregistering.
	wg.Add(clients)
	for i := 0; i < clients; i++ {
		go func() {
			defer wg.Done()
			c := newClient(nil) // fine here — this test never touches c.conn
			h.Register(c)

			// Drain c.send so Broadcast never blocks on a full buffer
			// while this client is "connected".
			done := make(chan struct{})
			go func() {
				for range c.send {
				}
				close(done)
			}()

			for j := 0; j < broadcasts; j++ {
				h.Broadcast(Update{MMSI: 1, Name: "TEST", Ping: vessel.Ping{}})
			}

			// Unregister before closing send — Broadcast and Unregister
			// share h.mu, so once Unregister returns, no in-flight or
			// future Broadcast call can still be holding a reference to
			// this client to send into. Closing first would leave a
			// window where a concurrent Broadcast could send on an
			// already-closed channel and panic.
			h.Unregister(c)
			close(c.send)
			<-done
		}()
	}

	// A separate broadcaster running concurrently with clients
	// registering/unregistering.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < broadcasts; j++ {
			h.Broadcast(Update{MMSI: 2, Name: "OTHER", Ping: vessel.Ping{}})
		}
	}()

	wg.Wait()
}
