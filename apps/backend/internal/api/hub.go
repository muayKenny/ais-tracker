package api

import (
	"sync"

	"github.com/kenshef/ais-tracker/apps/backend/internal/vessel"
)

// Update is what gets broadcast to every connected client — one vessel's
// latest observation, not its full accumulated trail. Defaulted here
// (not explicitly decided) to avoid repeating trail history a client
// should already have from earlier updates; easy to change to a full
// vessel.Vessel later if that turns out wrong.
type Update struct {
	MMSI int64
	Name string
	Ping vessel.Ping
}

// Hub tracks every currently-connected Client and fans updates out to all
// of them. Same mutex-guarded-set shape as Store, holding connections
// instead of vessel data.
type Hub struct {
	mu      sync.Mutex
	clients map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*Client]struct{})}
}

// Register adds a client to the broadcast set.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
}

// Unregister removes a client from the broadcast set. Safe to call more
// than once for the same client.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
}

// Broadcast pushes update into every currently-registered client's own
// channel. A client whose buffer is already full is skipped rather than
// blocked on — one slow or stuck client shouldn't stall delivery to
// everyone else.
func (h *Hub) Broadcast(update Update) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for c := range h.clients {
		select {
		case c.send <- update:
		default:
		}
	}
}
