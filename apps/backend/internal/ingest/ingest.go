package ingest

import (
	"context"
	"log"

	"github.com/gorilla/websocket"

	"github.com/kenshef/ais-tracker/apps/backend/internal/vessel"
)

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
}

// Err returns the reason Pings closed (nil if it closed because ctx was
// cancelled cleanly). Only meaningful after Pings has been fully drained —
// err is always set before the channel is closed, and Go's memory model
// guarantees that write is visible once a receiver observes the close, so
// no separate lock is needed here.
func (c *Connection) Err() error {
	return c.err
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

	go func() {
		defer conn.Close()
		defer close(pings)

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
