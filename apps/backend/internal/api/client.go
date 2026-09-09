package api

import (
	"log"

	"github.com/gorilla/websocket"
)

// Client is one connected WebSocket client, as far as Hub is concerned.
// send is its own outbound buffer — Broadcast pushes into it, and
// writeLoop (below) drains it and writes to the actual socket. Buffered,
// not unbuffered, so a client mid-write to its socket doesn't cause every
// broadcast in that window to be dropped.
type Client struct {
	conn *websocket.Conn
	send chan Update
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

// readLoop's only real job is detecting disconnection — this hub is
// push-only, so we don't care what a client sends us, just when reading
// fails (closed connection, network error), which is the signal to clean
// up. Owns the cleanup: unregistering, closing send (which lets writeLoop
// exit its range naturally), and closing the connection.
func (c *Client) readLoop(hub *Hub) {
	defer func() {
		hub.Unregister(c)
		close(c.send)
		c.conn.Close()
	}()

	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}
