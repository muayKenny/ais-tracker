# Requirements

**Status:** Draft — open questions below need answers before this is considered final.

## Problem Statement

We are building a system that ingests ship observations from realtime
AIS-based data sources, maintains a live model of vessel positions, and
detects when ships enter defined regions, so users can monitor meaningful
maritime activity in near real time.

## Functional Requirements

1. The system shall ingest AIS observations for ships from an upstream
   realtime streaming data source.
2. The system shall maintain the latest known location for each tracked
   ship.
3. The system shall display tracked ships on a geographically accurate
   world map.
4. The system shall ensure that, for normal display, each ship is
   represented by one current location, based on the latest valid
   observation.
5. The system shall allow a user to select a ship on the map and view its
   latest known metadata and state.
6. The system shall allow users to define watched geographic regions
   using coordinates.
7. The system shall detect when a ship enters a watched region.
8. The system shall support region-entry alerts for:
   - any ship entering a watched region
   - a specific ship entering a watched region
9. The system shall expose newly detected region-entry events to the user
   in near real time.

## Non-Functional Requirements

1. The system should prioritize correct latest-known ship state over
   maximum ingestion throughput.
2. The system should deliver ship-state and region-entry updates to
   connected clients within a few seconds of ingesting new AIS
   observations.
3. The system should support at least 1,000 concurrent frontend users for
   v1.
4. The system should remain operational if an upstream AIS poll fails
   temporarily, and should recover automatically on the next successful
   poll.
5. The system should keep infrastructure and runtime costs low enough for
   personal development and small-scale deployment.

## Out of Scope

- Satellite tracking
- Flight tracking
- Historical tracking or playback of ship movement
- Multi-source AIS data reconciliation
- SMS, email, or external notification delivery
- Multi-region or distributed cloud deployment
- Custom WebGPU or globe-based rendering
- Advanced analytics (route prediction, anomaly detection, etc.)

## Open Questions

- **Accounts/persistence for watched regions (FR6):** Do watched regions
  belong to a user account (requires auth + a DB-backed store), or are
  they client-side only (e.g. browser storage, no accounts in v1)?
- **Geofence shape (FR6):** Are watched regions arbitrary polygons, or
  simple boxes/circles? Boxes would mirror the bounding-box model
  aisstream.io already uses for the ingest subscription.
- ~~**Entry detection needs one step of history (FR7/FR8)**~~ —
  **Resolved 2026-09-01.** `vessel.Vessel.Trail` (a bounded rolling
  window of recent `Ping`s) covers this and more — the immediately-prior
  position is just `Trail[len(Trail)-2]`.
- ~~**Staleness/expiry (FR4)**~~ — **Resolved 2026-09-01.**
  `vessel.Vessel.IsStale(now)`, a derived judgment computed on read
  against a fixed 5-minute threshold — not navigational-status aware
  yet (real AIS reporting intervals vary a lot by status), a known
  simplification. Nothing evicts a stale vessel from the store; that's
  a separate, deferred concern (Phase 7).
- **Region-entry event delivery mechanism (FR9):** Push (e.g. WebSocket)
  or poll? This mirrors the general ship-state delivery mechanism
  decision but is worth confirming applies the same way to events.
