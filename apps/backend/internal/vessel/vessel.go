package vessel

import "time"

// Ping is one observation of a vessel at an instant — the system's own
// domain shape for a single AIS message, no aisstream.io awareness or
// wire-format field names. Ingestion is responsible for producing these;
// nothing else in the system should need to know where the data
// originally came from.
type Ping struct {
	Latitude  float64
	Longitude float64
	Course    float64
	Speed     float64
	Heading   *float64 // nil when not available
	Timestamp time.Time
}

// Vessel is the tracked entity itself, identified by MMSI. Trail is a
// bounded, in-memory rolling window of recent Pings (most recent last) —
// not a persisted or unbounded history. It exists to support both entry
// detection (comparing the last two entries) and rendering a vessel's
// recent movement, without becoming the historical-playback log that was
// explicitly ruled out in requirements.md.
type Vessel struct {
	MMSI  int64
	Name  string
	Trail []Ping
}

// StaleAfter is a single fixed threshold for now — not navigational-status
// aware, even though real AIS reporting intervals vary a lot (anchored
// vessels ping every ~3 minutes, moving ones every ~2-10 seconds). Vessel
// doesn't currently carry NavigationalStatus, so a per-status threshold
// isn't possible without widening Ping first. Revisit if that turns out to
// matter in practice.
const StaleAfter = 5 * time.Minute

// LastPing returns the most recent entry in Trail, if any.
func (v Vessel) LastPing() (Ping, bool) {
	if len(v.Trail) == 0 {
		return Ping{}, false
	}
	return v.Trail[len(v.Trail)-1], true
}

// IsStale reports whether this vessel hasn't been heard from recently,
// relative to now. It's a derived judgment computed on read, not stored
// state — nothing evicts a stale vessel from the Store; that's a
// deliberately separate, deferred concern (see roadmap.md Phase 7).
func (v Vessel) IsStale(now time.Time) bool {
	last, ok := v.LastPing()
	if !ok {
		return true
	}
	return now.Sub(last.Timestamp) > StaleAfter
}
