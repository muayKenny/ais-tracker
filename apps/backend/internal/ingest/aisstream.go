package ingest

import (
	"encoding/json"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// rawMessage is aisstream.io's message envelope, decoded as close to the
// wire format as possible. MessageType determines how Message should be
// interpreted; it's left undecoded (json.RawMessage) until we know which
// shape to decode it into. MetaData has been present on every message
// type seen so far.
type rawMessage struct {
	MessageType string          `json:"MessageType" validate:"required"`
	Message     json.RawMessage `json:"Message"`
	MetaData    rawMetaData     `json:"MetaData"`
}

type rawMetaData struct {
	MMSI       float64 `json:"MMSI" validate:"required"`
	MMSIString float64 `json:"MMSI_String"`
	ShipName   string  `json:"ShipName"`
	Latitude   float64 `json:"latitude" validate:"gte=-90,lte=90"`
	Longitude  float64 `json:"longitude" validate:"gte=-180,lte=180"`
	TimeUTC    string  `json:"time_utc"`
}

// Validate checks MessageType is present on every message, and only checks
// MetaData's fields (MMSI, lat/lon) when the message actually carries
// position data. Other message types (e.g. SubscriptionConfirmation) don't
// populate MetaData at all, so validating it unconditionally would reject
// every connection's handshake message.
func (r rawMessage) Validate() error {
	if err := validate.Var(r.MessageType, "required"); err != nil {
		return err
	}

	if r.MessageType != "PositionReport" {
		return nil
	}

	return validate.Struct(r.MetaData)
}

// rawPositionReportEnvelope mirrors Message's shape specifically for
// MessageType == "PositionReport" — the single key nests the real payload.
type rawPositionReportEnvelope struct {
	PositionReport rawPositionReport `json:"PositionReport"`
}

// rawPositionReport is the full position payload, undecided on canonical
// lat/lon yet (MetaData carries a rounded copy of the same values — see
// roadmap.md Phase 1 notes). Numbers stay float64 here; sentinel "not
// available" values (511 heading, -128 rate-of-turn) aren't converted to
// nil yet either — both are shaping-step concerns, not parsing ones.
type rawPositionReport struct {
	Cog                       float64 `json:"Cog"`
	CommunicationState        float64 `json:"CommunicationState"`
	Latitude                  float64 `json:"Latitude"`
	Longitude                 float64 `json:"Longitude"`
	MessageID                 float64 `json:"MessageID"`
	NavigationalStatus        float64 `json:"NavigationalStatus"`
	PositionAccuracy          bool    `json:"PositionAccuracy"`
	Raim                      bool    `json:"Raim"`
	RateOfTurn                float64 `json:"RateOfTurn"`
	RepeatIndicator           float64 `json:"RepeatIndicator"`
	Sog                       float64 `json:"Sog"`
	Spare                     float64 `json:"Spare"`
	SpecialManoeuvreIndicator float64 `json:"SpecialManoeuvreIndicator"`
	Timestamp                 float64 `json:"Timestamp"`
	TrueHeading               float64 `json:"TrueHeading"`
	UserID                    float64 `json:"UserID"`
	Valid                     bool    `json:"Valid"`
}

// PositionReport decodes Message into the position payload. Only call
// this when MessageType == "PositionReport" — every other message type
// has a different (currently unmodeled) shape under Message.
func (r rawMessage) PositionReport() (rawPositionReport, error) {
	var envelope rawPositionReportEnvelope
	if err := json.Unmarshal(r.Message, &envelope); err != nil {
		return rawPositionReport{}, err
	}
	return envelope.PositionReport, nil
}
