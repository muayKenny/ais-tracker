package ingest

import (
	"context"
	"log"
	"reflect"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/kenshef/ais-tracker/apps/backend/internal/vessel"
)

// resubscribeInterval throttles subscription updates. aisstream allows at
// most one per second per connection and *closes the connection* if you
// exceed it, so this is enforced structurally rather than left to callers.
//
// Deliberately above 1s rather than exactly 1s: sitting on the documented
// ceiling meant consecutive updates landed ~1.000s apart, which aisstream
// treated as too fast and dropped the connection. Treat the limit as a
// ceiling to stay under, not a target to hit.
const resubscribeInterval = 2 * time.Second

type subscriptionMessage struct {
	APIKey             string        `json:"APIKey"`
	BoundingBoxes      [][][]float64 `json:"BoundingBoxes"`
	FilterShipMMSI     []string      `json:"FiltersShipMMSI,omitempty"`
	FilterMessageTypes []string      `json:"FilterMessageTypes,omitempty"`
}

var DEFAULT_BOUNDING_BOX = [][]float64{
	{25.835302, -80.207729},
	{25.602700, -79.879297},
}

// PingMessage is one shaped observation, ready to hand to a consumer:
// identity (MMSI, Name) plus the Ping itself.
type PingMessage struct {
	MMSI int64
	Name string
	Ping vessel.Ping
}

// Connection is what Connect returns: a channel of pings, plus a way to
// find out why it stopped once Pings closes. Shape mirrors bufio.Scanner —
// range over Pings, then call Err() once the loop ends.
type Connection struct {
	Pings <-chan PingMessage
	err   error

	// desired holds the latest requested bounding boxes; the subscriber
	// goroutine drains it on a ticker. Latest-wins rather than queued —
	// if the view moves three times in a second, only the final position
	// matters, and intermediate ones are worthless.
	mu      sync.Mutex
	desired [][][]float64
	dirty   bool
	sent    [][][]float64
}

// Err returns the reason Pings closed (nil if it closed because ctx was
// cancelled cleanly). Only meaningful after Pings has been fully drained —
// err is always set before the channel is closed, and Go's memory model
// guarantees that write is visible once a receiver observes the close, so
// no separate lock is needed here.
func (c *Connection) Err() error {
	return c.err
}

// Subscribe asks for the upstream subscription to cover boxes instead of
// whatever it covers now. Non-blocking and latest-wins: calling it faster
// than resubscribeInterval is safe, only the most recent value is sent.
// An empty set is ignored — that would mean subscribing to nothing, which
// silently kills the feed rather than doing anything useful.
func (c *Connection) Subscribe(boxes [][][]float64) {
	if len(boxes) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.desired = boxes
	c.dirty = true
}

// takeDesired returns the pending subscription, if any, and clears the
// dirty flag. Returns false when nothing is pending *or* when the pending
// value matches what was already sent — panning away and back produces an
// identical box, and spending rate-limit budget to re-send it is pure
// waste.
func (c *Connection) takeDesired() ([][][]float64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.dirty {
		return nil, false
	}
	c.dirty = false

	if reflect.DeepEqual(c.desired, c.sent) {
		return nil, false
	}
	c.sent = c.desired
	return c.desired, true
}

// Connect opens a connection to aisstream.io and returns a Connection
// whose Pings channel receives one PingMessage per valid position report.
// The dial and subscription happen synchronously — a failure here returns
// immediately. Once connected, reading continues on an internal goroutine
// until ctx is cancelled or the connection fails, at which point Pings is
// closed and Err() reports why.
func Connect(ctx context.Context, apiKey string) (*Connection, error) {
	conn, _, err := websocket.DefaultDialer.Dial("wss://stream.aisstream.io/v0/stream", nil)
	if err != nil {
		return nil, err
	}

	sub := subscriptionMessage{
		APIKey: apiKey,
		BoundingBoxes: [][][]float64{
			DEFAULT_BOUNDING_BOX,
		},
		FilterMessageTypes: []string{"PositionReport"},
	}

	if err := conn.WriteJSON(sub); err != nil {
		conn.Close()
		return nil, err
	}

	pings := make(chan PingMessage)
	c := &Connection{Pings: pings}

	// Closed when the read loop exits, so the subscriber goroutine below
	// shuts down too rather than writing to a dead connection forever.
	done := make(chan struct{})

	// The subscriber owns *all* writes to conn after the initial
	// subscription above. gorilla/websocket permits one concurrent reader
	// and one concurrent writer, so this running alongside the read loop
	// is fine — what would break is a second goroutine also writing.
	go func() {
		ticker := time.NewTicker(resubscribeInterval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				boxes, ok := c.takeDesired()
				if !ok {
					continue
				}

				update := subscriptionMessage{
					APIKey:             apiKey,
					BoundingBoxes:      boxes,
					FilterMessageTypes: []string{"PositionReport"},
				}
				if err := conn.WriteJSON(update); err != nil {
					log.Printf("resubscribe failed: %v", err)
					return
				}
				log.Printf("resubscribed to %d bounding box(es): %v", len(boxes), boxes)
			}
		}
	}()

	go func() {
		defer conn.Close()
		defer close(pings)
		defer close(done)

		for {
			select {
			case <-ctx.Done():
				c.err = ctx.Err()
				return
			default:
			}

			var raw rawMessage
			if err := conn.ReadJSON(&raw); err != nil {
				c.err = err
				return
			}

			if err := raw.Validate(); err != nil {
				log.Printf("discarding invalid message: %v", err)
				continue
			}

			if raw.MessageType != "PositionReport" {
				continue
			}

			pos, err := raw.PositionReport()
			if err != nil {
				log.Printf("discarding message: failed to decode position report: %v", err)
				continue
			}

			mmsi, name, ping := raw.Shape(pos)

			select {
			case pings <- PingMessage{MMSI: mmsi, Name: name, Ping: ping}:
			case <-ctx.Done():
				c.err = ctx.Err()
				return
			}
		}
	}()

	return c, nil
}
