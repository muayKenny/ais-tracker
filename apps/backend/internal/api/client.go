package api

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

// Viewport is the geographic rectangle a client is currently looking at,
// as reported by the frontend's map.
type Viewport struct {
	West  float64 `json:"west"`
	South float64 `json:"south"`
	East  float64 `json:"east"`
	North float64 `json:"north"`
}

// BoundingBox converts to aisstream's format: [[northLat, westLon],
// [southLat, eastLon]] — latitude first, opposite of the lng/lat ordering
// MapLibre and GeoJSON use.
//
// Values are clamped: a zoomed-out or antimeridian-crossing map readily
// reports longitudes outside [-180, 180] and latitudes past the Mercator
// limits, which aisstream won't accept.
func (v Viewport) BoundingBox() [][]float64 {
	return [][]float64{
		{clamp(v.North, -90, 90), clamp(v.West, -180, 180)},
		{clamp(v.South, -90, 90), clamp(v.East, -180, 180)},
	}
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// clientMessage is anything a client sends us. Currently only viewport
// updates; the type tag exists so adding more later doesn't require
// guessing at shape.
type clientMessage struct {
	Type     string    `json:"type"`
	Viewport *Viewport `json:"viewport,omitempty"`
}

// Client is one connected WebSocket client, as far as Hub is concerned.
// send is its own outbound buffer — Broadcast pushes into it, and
// writeLoop (below) drains it and writes to the actual socket. Buffered,
// not unbuffered, so a client mid-write to its socket doesn't cause every
// broadcast in that window to be dropped.
type Client struct {
	conn *websocket.Conn
	send chan Update

	// viewport is written by this client's readLoop and read when the Hub
	// aggregates boxes. Both happen under the Hub's mutex rather than a
	// second lock here — see Hub.SetViewport / Hub.BoundingBoxes.
	viewport *Viewport
}

// newClient returns a Client ready to use, wrapping the given connection.
// A zero-value Client{} would have a nil send channel, and sending on a
// nil channel blocks forever — same "unsafe zero value" reasoning as
// Store needing New().
func newClient(conn *websocket.Conn) *Client {
	return &Client{
		conn: conn,
		send: make(chan Update, 16),
	}
}

// writeLoop drains c.send and writes each update to the socket as JSON.
// Runs until c.send is closed (by readLoop's cleanup) or a write fails.
// Meant to run on its own goroutine, one per connected client.
func (c *Client) writeLoop() {
	for update := range c.send {
		if err := c.conn.WriteJSON(update); err != nil {
			log.Printf("client write failed: %v", err)
			return
		}
	}
}

// readLoop reads client messages (currently just viewport updates) and
// detects disconnection — a failed read is the signal to clean up. Owns
// that cleanup: unregistering, closing send (which lets writeLoop exit its
// range naturally), and closing the connection.
func (c *Client) readLoop(hub *Hub) {
	defer func() {
		hub.Unregister(c)
		close(c.send)
		c.conn.Close()
	}()

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var msg clientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("discarding unparseable client message: %v", err)
			continue
		}

		if msg.Type == "viewport" && msg.Viewport != nil {
			hub.SetViewport(c, *msg.Viewport)
		}
	}
}
