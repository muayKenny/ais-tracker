package api

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/kenshef/ais-tracker/apps/backend/internal/store"
)

// upgrader is stateless/config-only and safe for concurrent use, so it's
// shared across every request rather than recreated per-connection.
//
// CheckOrigin defaults to true (allow any origin) for now: production
// serves the frontend from this same binary (same-origin, see
// system-design.md), so the check wouldn't do anything there anyway — but
// leaving gorilla's strict default in place would break local dev, where
// the frontend runs on Vite's own dev server port. Worth tightening if
// this were ever exposed beyond a single deployment you control.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// NewHandler returns an http.HandlerFunc that upgrades incoming requests
// to WebSocket connections. Each new client gets the current snapshot
// immediately, then is registered with hub and kept alive by its own
// read/write goroutines until it disconnects.
func NewHandler(hub *Hub, vesselStore *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade failed: %v", err)
			return
		}

		client := newClient(conn)

		for _, v := range vesselStore.Snapshot() {
			ping, ok := v.LastPing()
			if !ok {
				continue
			}
			select {
			case client.send <- Update{MMSI: v.MMSI, Name: v.Name, Ping: ping}:
			default:
			}
		}

		hub.Register(client)
		go client.writeLoop()
		client.readLoop(hub)
	}
}
