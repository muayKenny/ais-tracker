package ingest

import (
	"context"
	"log"

	"github.com/gorilla/websocket"
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

func Connect(ctx context.Context, apiKey string) error {
	conn, _, err := websocket.DefaultDialer.Dial("wss://stream.aisstream.io/v0/stream", nil)

	if err != nil {
		return err
	}
	defer conn.Close()

	sub := subscriptionMessage{
		APIKey: apiKey,
		BoundingBoxes: [][][]float64{
			DEFAULT_BOUNDING_BOX,
		},
		FilterMessageTypes: []string{"PositionReport"},
	}

	if err := conn.WriteJSON(sub); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var raw rawMessage
		if err := conn.ReadJSON(&raw); err != nil {
			return err
		}

		if err := raw.Validate(); err != nil {
			log.Printf("discarding invalid message: %v", err)
			continue
		}

		if raw.MessageType != "PositionReport" {
			log.Printf("got message: type=%s (no position payload)", raw.MessageType)
			continue
		}

		pos, err := raw.PositionReport()
		if err != nil {
			log.Printf("discarding message: failed to decode position report: %v", err)
			continue
		}

		log.Printf("got message: type=%s meta=%+v position=%+v", raw.MessageType, raw.MetaData, pos)
	}
}
