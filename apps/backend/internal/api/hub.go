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

	// OnViewportChange, if set, fires whenever any client's viewport
	// changes or a client with a viewport disconnects. Keeps the Hub from
	// needing to know anything about ingest — main.go wires the two.
	OnViewportChange func()
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*Client]struct{})}
}

// SetViewport records what a client is currently looking at.
func (h *Hub) SetViewport(c *Client, v Viewport) {
	h.mu.Lock()
	c.viewport = &v
	h.mu.Unlock()

	if h.OnViewportChange != nil {
		h.OnViewportChange()
	}
}

// BoundingBoxes returns one box per client that has reported a viewport,
// in aisstream's format. Plural by design: with one client this is a
// single box, with many it's one each — aisstream accepts multiple boxes
// natively, so no enclosing-rectangle math that would over-fetch.
func (h *Hub) BoundingBoxes() [][][]float64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	boxes := make([][][]float64, 0, len(h.clients))
	for c := range h.clients {
		if c.viewport != nil {
			boxes = append(boxes, c.viewport.BoundingBox())
		}
	}
	return boxes
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
	hadViewport := c.viewport != nil
	delete(h.clients, c)
	h.mu.Unlock()

	// A departing client's box should stop being subscribed to.
	if hadViewport && h.OnViewportChange != nil {
		h.OnViewportChange()
	}
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
