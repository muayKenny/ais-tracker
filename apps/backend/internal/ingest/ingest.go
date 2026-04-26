package ingest

import (
	"context"
	"log"
	"time"

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
	conn, _, err := websocket.DefaultDialer.Dial("wss://stream.aistream.io/vo/stream", nil)

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

		var msg map[string]any
		if err := conn.ReadJSON(&msg); err != nil {
			return err
		}

		log.Printf("got message: %+v", msg)
		_ = time.Now()
	}
}
