package ingest

import (
	"strings"
	"time"

	"github.com/kenshef/ais-tracker/apps/backend/internal/vessel"
)

// Shape converts already-validated, already-decoded raw data into the
// domain shape: the vessel's identity (mmsi, name) and one observation
// (ping). pos must come from a prior call to r.PositionReport() for this
// same message. Identity is returned separately from Ping rather than
// bundled in, since Ping is meant to be one entry in a Vessel's trail, not
// something that carries identity itself.
//
// Canonical lat/lon comes from pos (full precision), not MetaData (a
// rounded copy) — both are populated with the same underlying reading.
// Timestamp is when we received the message, not the upstream time_utc
// string, since its exact format hasn't been verified against more samples
// yet.
func (r rawMessage) Shape(pos rawPositionReport) (mmsi int64, name string, ping vessel.Ping) {
	mmsi = int64(r.MetaData.MMSI)
	name = strings.TrimSpace(r.MetaData.ShipName)
	ping = vessel.Ping{
		Latitude:  pos.Latitude,
		Longitude: pos.Longitude,
		Course:    pos.Cog,
		Speed:     pos.Sog,
		Heading:   headingPtr(pos.TrueHeading),
		Timestamp: time.Now(),
	}
	return
}

// headingPtr returns nil for AIS's "not available" sentinel (511),
// otherwise a pointer to the heading value.
func headingPtr(raw float64) *float64 {
	if raw == 511 {
		return nil
	}
	return &raw
}
