# Roadmap

**Current phase:** Phase 1 — Ingest Layer
**Last updated:** 2026-08-31

This is the living plan for the project, phased by platform layer —
backend built out fully, layer by layer, with frontend last — rather than
an end-to-end vertical slice through every layer at once. See
[requirements.md](./requirements.md) for the full functional/non-functional
requirements this roadmap is built against, and
[system-design.md](./system-design.md) for the pipeline these phases walk
through.

Each phase also names its **Platform decisions** — the concrete
technology/architecture calls that phase locks in, not just tasks. Some
are already made (carried over from the system-design strawman); others
stay open until that phase actually starts.

Conventions:
- Each phase has a **Status** (`Not started` / `In progress` / `Done`),
  updated as work crosses into/out of it.
- Task checkboxes get checked off in the same commit that does the work.
- **Notes** under each phase capture anything discovered or decided while
  working the phase (schema quirks, scope changes) — dated, short.
- Once a phase starts, its tasks get broken down into GitHub Issues under
  a matching Milestone for day-to-day tracking. This file stays the
  narrative/status layer, not the ticket system.

---

## Phase 0: System Design

**Status:** In progress
**Goal:** Settle the abstract shape of the system — pipeline roles, data
flow, and storage strawman — before building anything, so Phase 1 has a
concrete architecture to prove out rather than being exploratory from
scratch.

**Tasks:**
- [x] Define the abstract pipeline (external source → ingestion → current
      state → distribution → presentation)
- [x] Name the overall pattern and why (digital twin / last-write-wins
      store / pub-sub distribution, not event sourcing)
- [x] Strawman storage and concurrency decisions
- [ ] Review/revise strawman decisions

**Checkpoint:** [system-design.md](./system-design.md) captures the
pipeline shape and the key architecture decisions, agreed on before
Phase 1 work starts.

**Notes:**
- 2026-08-30: Considered event sourcing; ruled out because AIS position
  reports are snapshots, not deltas — current state doesn't need to be a
  projection of a replayable log. See system-design.md for the full
  reasoning.
- 2026-08-31: Switched from an end-to-end vertical spike to a
  platform-layered plan — fully build and prove out each backend layer
  before moving to the next, frontend last. Reasoning: this is also a
  vehicle for learning Go/backend design deeply, and layer-by-layer is
  easier to cognitively hold than a thin slice across everything at
  once. Frontend is deliberately last because it's the one layer that's
  already familiar — the learning value (and the risk of building the
  wrong thing) is concentrated in the backend layers, and every backend
  contract can be verified without a UI (curl, a raw WebSocket client,
  unit tests).

---

## Phase 1: Ingest Layer

**Status:** In progress
**Goal:** Reliably turn a raw aisstream.io message into a trusted,
validated `Vessel` domain value — or explicitly discard it. No storage,
no distribution, no UI.

**Platform decisions:**
- WebSocket client: `gorilla/websocket` (already in use)
- Validation: `go-playground/validator`, tags alongside the existing
  `json:"..."` tags on the same structs (already in use)
- Wire type vs. domain type split: raw/unrefined types stay private to
  `internal/ingest`; the clean `Vessel` type lives in its own
  `internal/vessel` package so other layers depend on the domain shape,
  not on aisstream's wire format

**Tasks:**
- [x] Connect to aisstream.io reliably
- [x] Parse messages into typed structs (`rawMessage` / `rawMetaData`)
- [x] Validate `MessageType` / `MMSI` / lat-lon, conditional on message
      type (`SubscriptionConfirmation` doesn't carry position data)
- [ ] Shape validated raw data into the domain `Vessel` struct — resolve
      MMSI as `int64` (not `float64`), trim `ShipName`, pick canonical
      lat/lon source, convert sentinel values (511 heading, -128
      rate-of-turn) to "missing" rather than literal numbers
- [ ] Discard (log + skip) messages that fail shaping, same as
      validation failures already do

**Checkpoint:** Given a raw aisstream message, the ingest layer produces
either a valid `Vessel` or an explicit discard — provable with unit
tests against captured real samples, no UI required.

**Notes:**
- 2026-08-31: Confirmed via real captured messages that `MessageType`
  dispatch is required — `SubscriptionConfirmation` arrives once per
  connection regardless of the `PositionReport` filter. aisstream.io
  docs confirm 25 total message types exist; current requirements only
  need `PositionReport` (MetaData already includes ShipName on every
  message, satisfying FR5 without needing `ShipStaticData`).
- 2026-08-31: Real data quirks found — `TrueHeading:511` and
  `RateOfTurn:-128` are AIS spec sentinel values for "not available,"
  not literal readings; `ShipName` arrives space-padded and needs
  trimming; lat/lon appear twice (full precision under
  `Message.PositionReport`, rounded under `MetaData`) — canonical source
  still to be picked during shaping.

---

## Phase 2: Current State

**Status:** Not started
**Goal:** Maintain the live, queryable snapshot of vessel state.
**Maps to:** FR2, FR4

**Platform decisions:**
- Concurrency: `sync.RWMutex`-guarded map keyed by MMSI — one writer
  (ingest), many readers. No channel-owned actor pattern; not justified
  at this scale.
- Storage: in-memory only, no persistence. Consistent with vessel state
  being explicitly non-historical (see system-design.md).

**Tasks:**
- [ ] Concurrent-safe vessel store keyed by MMSI
- [ ] Upsert on new `Vessel` data
- [ ] Keep one previous position per vessel (needed for entry detection,
      Phase 5)
- [ ] Define vessel TTL/staleness behavior (open question — see
      requirements.md)
- [ ] Expose a `Snapshot()` / `GetAll()` read API
- [ ] Basic observability/logging

**Checkpoint:** For any tracked vessel, a `Snapshot()` call returns its
correct latest known valid position — provable with a concurrent unit
test (`go test -race`), no network or UI needed.

**Notes:** _(none yet)_

---

## Phase 3: Distribution

**Status:** Not started
**Goal:** Get current state out of the process to any connected
consumer.

**Platform decisions:**
- Transport: WebSocket (mirrors the aisstream.io feed itself; push, not
  poll)
- Fan-out: a broadcast hub — one goroutine owns the set of connected
  clients, ingest writes an update, hub fans it out. Standard pub/sub
  shape, no persistence in the pipe.

**Tasks:**
- [ ] WebSocket endpoint: send full snapshot on connect
- [ ] Broadcast hub: push each update to all connected clients
- [ ] Basic reconnect/backpressure handling

**Checkpoint:** A raw WebSocket client (`wscat`, or a browser devtools
console) connecting to the endpoint receives a snapshot, then live
updates — verified without any frontend UI.

**Notes:** _(none yet)_

---

## Phase 4: Watched Regions (backend)

**Status:** Not started
**Goal:** Regions can be defined and evaluated via the backend alone —
no creation UI yet, that's Phase 8.

**Platform decisions:**
- Persistence: SQLite, not Postgres — zero-ops, fits the single-box/
  low-cost deployment NFR, no multi-writer concurrency needed at this
  scale.
- Region shape: boxes/circles for v1, not arbitrary polygons, mirroring
  the bounding-box model aisstream.io's own subscription already uses.
- Accounts: still an open question (see requirements.md) — resolve
  before persistence work starts, since it decides whether regions are
  scoped to a user or global to the deployment.

**Tasks:**
- [ ] Server-side region model
- [ ] Persist regions (SQLite)
- [ ] CRUD API (create/list/edit/delete) — testable via `curl`
- [ ] Validate region coordinates

**Checkpoint:** A region can be created, listed, and deleted via the API
alone — no UI required to prove it works.

**Notes:** _(none yet)_

---

## Phase 5: Entry Detection

**Status:** Not started
**Goal:** Detect transitions, not just presence.

Core logic:
```
previous position outside region
current position inside region
=> region entry event
```

**Platform decisions:**
- Detection state: reuses Phase 2's one-previous-position tracking —
  no separate history/log needed (see system-design.md's delta-vs-
  snapshot reasoning).

**Tasks:**
- [ ] Evaluate position against regions using the previous/current pair
      from Phase 2
- [ ] Support any-ship alerts
- [ ] Support specific-ship alerts
- [ ] Avoid duplicate alerts while a ship remains inside a region
- [ ] Define re-entry behavior
- [ ] Store recent events
- [ ] Test edge cases

**Checkpoint:** When a vessel crosses into a watched region, the system
emits exactly one correct entry event — provable with unit tests, no UI
needed.

**Notes:** _(none yet — this is probably the most important backend
correctness phase)_

---

## Phase 6: Event Delivery

**Status:** Not started
**Goal:** Push region-entry events to any connected consumer, same
mechanism Phase 3 built for state.

**Platform decisions:**
- Reuses Phase 3's broadcast hub/WebSocket transport rather than a
  second, separate channel.

**Tasks:**
- [ ] Extend the broadcast hub to also carry events
- [ ] Missed-event recovery / reconnection semantics
- [ ] Basic rate limiting/backpressure

**Checkpoint:** A raw WebSocket client sees a region-entry event within a
few seconds of a real crossing — verified without a frontend.

**Notes:** _(none yet)_

---

## Phase 7: Hardening

**Status:** Not started
**Goal:** Make the backend reliable enough for v1, unattended.

**Platform decisions:**
- Deployment target: single small VPS/box, Docker Compose — matches the
  low-cost NFR; no managed/multi-region infrastructure.

**Tasks:**
- [ ] Upstream reconnect/retry (the real gap flagged back in Phase 1 —
      any `ReadJSON` error currently kills ingest permanently)
- [ ] Tolerate failed polls/stream interruptions
- [ ] Logging and metrics
- [ ] Error handling
- [ ] Load test assumptions
- [ ] Deploy setup
- [ ] Cost review
- [ ] Rollback plan

**Checkpoint:** The system can run unattended, recover from upstream
failures, and support expected v1 load.

**Notes:** _(none yet)_

---

## Phase 8: Frontend

**Status:** Not started
**Goal:** Build the UI on top of a fully proven backend. Deliberately
last — this is the layer already well understood, so it should move
fast once every backend contract it depends on already works.

**Platform decisions:**
- Map library: not yet decided (Leaflet vs. MapLibre GL vs. other) —
  open until this phase starts.

**Tasks:**
- [ ] World map view
- [ ] Vessel markers, updated from the Phase 3 WebSocket stream
- [ ] Stale/active visual state
- [ ] Select vessel / metadata panel
- [ ] Connection status
- [ ] Region creation UI (coordinate input or draw-on-map)
- [ ] List/edit/delete regions (consumes Phase 4's API)
- [ ] Event feed UI (consumes Phase 6's event stream)
- [ ] Frontend performance with many markers
- [ ] Basic filtering, if needed

**Checkpoint:** A user can open the app, see live vessel positions,
select one, define and manage watched regions, and see region-entry
events — the full FR1–9 experience, on a backend that was already
independently proven at every layer.

**Notes:** _(none yet)_
