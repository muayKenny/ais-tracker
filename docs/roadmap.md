# Roadmap

**Current phase:** Phase 8 — Frontend (design underway; Phases 4–7 deliberately
deferred, see note below)
**Last updated:** 2026-09-09

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

**Status:** Done
**Goal:** Reliably turn a raw aisstream.io message into a trusted,
validated domain value — or explicitly discard it. No storage, no
distribution, no UI.

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
- [x] Shape validated raw data into the domain shape — MMSI resolved as
      `int64` (not `float64`), `ShipName` trimmed, canonical lat/lon
      picked (full-precision `PositionReport`, not the rounded `MetaData`
      copy), sentinel values (511 heading) converted to `nil`
- [x] No failure mode left to discard at this stage — shaping only runs
      on already-validated data, so it can't itself produce bad output

**Checkpoint:** Given a raw aisstream message, the ingest layer produces
either a valid observation or an explicit discard — proven by running
against the real live feed, no UI required.

**Notes:**
- 2026-08-31: Confirmed via real captured messages that `MessageType`
  dispatch is required — `SubscriptionConfirmation` arrives once per
  connection regardless of the `PositionReport` filter. aisstream.io
  docs confirm 25 total message types exist; current requirements only
  need `PositionReport` (MetaData already includes ShipName on every
  message, satisfying FR5 without needing `ShipStaticData`).
- 2026-08-31: Split the domain shape in two — `vessel.Ping` (one
  observation: lat/lon/course/speed/heading/timestamp, no identity) and
  `vessel.Vessel` (the entity: MMSI, Name, a bounded `Trail []Ping`).
  `Shape()` now returns `(mmsi, name, ping)` rather than one combined
  struct, since a Ping is meant to be one entry in a Vessel's trail, not
  something that carries identity itself.
- 2026-08-31: Real data quirks found — `TrueHeading:511` and
  `RateOfTurn:-128` are AIS spec sentinel values for "not available,"
  not literal readings; `ShipName` arrives space-padded and needs
  trimming; lat/lon appear twice (full precision under
  `Message.PositionReport`, rounded under `MetaData`) — canonical source
  still to be picked during shaping.
- 2026-09-01: Reworked `ingest.Connect`'s API from a callback
  (`OnPingFunc`, threaded through `Connect` → `runIngest` → whoever
  passed it in) to returning a `*Connection` with a `Pings` channel and
  an `Err()` method. Reason: the callback made the actual wiring
  ("who handles a ping") untraceable by reading the code — you had to
  chase an opaque parameter through three files to find `st.Upsert`.
  The channel version makes it explicit and visible: `for p :=
  range conn.Pings { vesselStore.Upsert(...) }`, read top to bottom in
  `main.go`, no hidden indirection. `Err()` mirrors `bufio.Scanner`'s
  shape — read until the channel closes, then check why.

---

## Phase 2: Current State

**Status:** In progress
**Goal:** Maintain the live, queryable snapshot of vessel state.
**Maps to:** FR2, FR4

**Platform decisions:**
- Concurrency: `sync.RWMutex`-guarded map keyed by MMSI — one writer
  (ingest), many readers. No channel-owned actor pattern; not justified
  at this scale.
- Storage: in-memory only, no persistence. Consistent with vessel state
  being explicitly non-historical (see system-design.md).
- Trail, not just one previous position: bounded rolling window
  (`maxTrailLength`, currently 20) per vessel — subsumes entry
  detection's "previous position" need and supports rendering a
  vessel's recent movement, without becoming the persisted/unbounded
  history that's out of scope.

**Tasks:**
- [x] Concurrent-safe vessel store keyed by MMSI (`internal/store`)
- [x] Upsert on new `Ping` data
- [x] Keep a bounded trail per vessel (subsumes "one previous position"
      for entry detection, Phase 5)
- [x] Define vessel TTL/staleness behavior — `Vessel.IsStale(now)`,
      computed on read against a fixed threshold; active eviction from
      the Store deferred to Phase 7 (see note)
- [x] Expose a `Snapshot()` read API
- [ ] Basic observability/logging (store-level; ingest already logs
      each ping)

**Checkpoint:** For any tracked vessel, a `Snapshot()` call returns its
correct latest known valid position — proven with a concurrent unit
test under `go test -race` (8 goroutines hammering `Upsert` on the same
MMSI while `Snapshot` reads concurrently), no network or UI needed.

**Notes:**
- 2026-08-31: `appendTrail` always allocates a fresh backing array
  rather than growing a vessel's `Trail` slice in place. `Snapshot`
  copies a `Vessel`'s `Trail` slice header without deep-copying its
  array — if `Upsert` reused that array on a later write, a goroutine
  holding an older snapshot could race against it on the same memory,
  even though the two look like separate values. Building it this way
  means a snapshot's backing array is never written to again once
  it's been handed out.
- Staleness/TTL resolved as `IsStale(now)` — a derived judgment, not
  stored state. Deliberately a single fixed threshold (5 min), not
  navigational-status aware, since `Ping` doesn't carry
  `NavigationalStatus` yet — real AIS reporting intervals vary a lot by
  status, so this is a known simplification, not an oversight.
  Eviction from the Store (bounding its memory over a long run) is a
  separate, deferred concern — Phase 7's job, not Phase 2's, since
  nothing needs it yet at this project's scale/runtime.
- Fixed a real bug while here: `Upsert` was unconditionally overwriting
  `Vessel.Name` on every ping, including with a blank string when a
  message's `ShipName` happened to be empty — silently erasing a
  previously known good name. Now only overwrites when non-empty.

---

## Phase 3: Distribution

**Status:** Done (core loop). Reconnect/backpressure hardening still
lives in Phase 7, not done here.
**Goal:** Get current state out of the process to any connected
consumer.

**Platform decisions:**
- Transport: WebSocket (mirrors the aisstream.io feed itself; push, not
  poll)
- Fan-out: a broadcast hub — mutex-guarded set of `*Client`, one
  goroutine per connected client (write loop + read loop, the latter
  only used to detect disconnection), `Broadcast` skips a client whose
  buffer is full rather than blocking on it.
- Package: `internal/api` (not `internal/realtime` — a deliberate,
  discussed call to also house future CRUD, e.g. Phase 4's regions
  endpoints, in the same package rather than splitting client-facing
  code across two).
- Broadcast payload: latest ping only (MMSI, name, one `vessel.Ping`),
  not the full `Vessel`/trail — a default, not a fully confirmed
  decision, chosen to avoid re-sending trail history a client should
  already have.

**Tasks:**
- [x] WebSocket endpoint (`/ws`): sends full snapshot on connect
      (`vesselStore.Snapshot()` + `Vessel.LastPing()`, one `Update` per
      currently-tracked vessel)
- [x] Broadcast hub: push each update to all connected clients
- [ ] Basic reconnect/backpressure handling (client buffer overflow is
      handled — see above — server-side reconnect after ingest failure
      is Phase 7's job, still open)

**Checkpoint:** A raw WebSocket client connecting to the endpoint
receives a snapshot, then live updates — **verified for real**, not just
tested in isolation: connected via `wscat -c ws://localhost:8080/ws`
against the live server and watched real vessel `Update` messages
stream in, including `"Heading":null` on vessels whose raw
`TrueHeading` was the AIS 511 sentinel — confirming Phase 1's
sentinel-to-nil handling is correct end to end, not just in isolation.

**Notes:**
- 2026-09-01: `hub.go`/`client.go`/`handler.go` all race-tested
  (`TestHubConcurrentRegisterBroadcast`, `go test -race`) — 8 goroutines
  registering/unregistering/broadcasting concurrently, clean.
- 2026-09-01: `ingest` still has no test coverage for the `Connection`/
  `Err()` ordering (reasoning-only, relies on Go's memory model
  guarantee that a channel close happens-after whatever was written
  before it) — a known, deliberately deferred gap, not an oversight.
- 2026-09-09: Decided to skip straight to Phase 8 (Frontend) rather
  than working Phases 4–7 in order. Reasoning: Ingestion → Current
  State → Distribution is already enough to render real, live data —
  the portfolio/career payoff (see project practice-framing memory) is
  concentrated in the rendering work, and Phases 4–7 (regions, entry
  detection, event delivery, hardening) aren't prerequisites for that.
  They're not abandoned, just deliberately out of sequence — worth
  returning to once the frontend's real.

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

**Status:** In progress (design)
**Goal:** Build the UI on top of a fully proven backend. Started ahead
of Phases 4–7 on purpose (see Phase 3 note) — Ingestion, Current State,
and Distribution are enough real data to build against.

**Platform decisions:**
- Renderer: **MapLibre GL**, not a custom three.js/WebGPU engine.
  Reasoning, decided after real back-and-forth: MapLibre is the actual
  industry-standard choice for exactly this kind of app (real-time
  tracking, GPU-batched rendering, not per-point DOM markers), it has
  **clustering built directly into its GeoJSON source** (`cluster:
  true`), and it already supports a globe projection — it solves both
  the "thousands of points without slowing the browser" and "does this
  look meaningful zoomed out" problems from Phase 0, for free, proven
  at production scale. Deliberately *not* building a custom WebGPU
  renderer for this project: that skill is already demonstrated
  elsewhere, and duplicating it here would cost real project-completion
  risk (a from-scratch rendering engine is a materially bigger,
  higher-risk scope than this project's other phases) for a
  redundant portfolio signal. MapLibre itself is a legitimate resume
  signal — knowing when to use proven tooling instead of reinventing
  it is a real engineering judgment, not a lesser one.
- Concurrency/performance layer: a **Web Worker owns the WebSocket
  connection** to `/ws` — receives and parses messages off the main
  thread, and batches/coalesces updates over a short window before
  handing a batch to the main thread (`postMessage`), rather than
  triggering a MapLibre `source.setData(...)` call per individual
  ping. This is where the Worker/concurrency skill set actually
  applies here — MapLibre's own rendering is a black box you don't
  get to inject into, but the live-data pipeline feeding it is fair
  game, and batching is the real performance-sensitive operation once
  many vessels update live.
- Serving: same Go binary serves the built `dist/` — see
  system-design.md's strawman decisions.

**Tasks:**
- [ ] Thinnest possible slice first: render one ship's position from a
      real WebSocket connection on a MapLibre map — same "walking
      skeleton" approach used for the backend phases, no worker,
      clustering, or globe projection yet
- [ ] World map/globe view (MapLibre globe projection)
- [ ] Vessel rendering as a clustered GeoJSON source, updated from the
      Phase 3 WebSocket stream
- [ ] Worker-based WebSocket client with update batching
- [ ] Stale/active visual state
- [ ] Select vessel / metadata panel
- [ ] Connection status
- [ ] Region creation UI (coordinate input or draw-on-map)
- [ ] List/edit/delete regions (consumes Phase 4's API, once it exists)
- [ ] Event feed UI (consumes Phase 6's event stream, once it exists)
- [ ] Basic filtering, if needed

**Checkpoint:** A user can open the app and see live vessel positions
rendered from the real backend, at scale, smoothly. Region
creation/event-feed UI checkpoints wait on Phases 4/6 actually
existing.

**Notes:**
- 2026-09-09: Considered a custom three.js/React-Three-Fiber/WebGPU
  renderer first (globe, `InstancedMesh`, hand-built clustering).
  Reconsidered after weighing it against MapLibre GL specifically —
  see Platform decisions above for the full reasoning. This also
  resolves the contradiction with requirements.md's Out of Scope list
  ("Custom WebGPU or globe-based rendering") — no longer a
  contradiction, since that's genuinely no longer the plan.
