# Roadmap

**Current phase:** Phase 1 — Architecture Spike
**Last updated:** 2026-08-28

This is the living plan for the project, phased as vertical slices. See
[requirements.md](./requirements.md) for the full functional/non-functional
requirements this roadmap is built against.

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

## Phase 1: Architecture Spike

**Status:** Not started
**Goal:** Prove the core technical path works.

Build a thin vertical slice: AIS source → ingest service → latest vessel
store → frontend map update. No accounts, no alerts, no polished UI yet.

**Questions to answer:**
- [ ] Can we connect to the AIS source reliably?
- [ ] What does the message schema look like?
- [ ] How noisy is the data?
- [ ] How do we identify vessels?
- [ ] How do we validate lat/lon?
- [ ] Can we update a map in near real time?
- [ ] What storage/cache makes sense?

**Checkpoint:** Working prototype with live ships on a map.

**Notes:** _(none yet)_

---

## Phase 2: Live Vessel State

**Status:** Not started
**Goal:** Turn raw observations into a clean current-state model.
**Maps to:** FR2, FR3, FR4, FR5

**Tasks:**
- [ ] Parse AIS messages
- [ ] Validate coordinates/timestamps/MMSI
- [ ] Discard bad/stale observations
- [ ] Maintain `latest_position_by_vessel`
- [ ] Keep one previous position for entry detection
- [ ] Define vessel TTL/staleness
- [ ] Expose current state API
- [ ] Add basic observability/logging

**Checkpoint:** For any tracked vessel, the system can show one latest
known valid position and metadata.

**Notes:** _(none yet)_

---

## Phase 3: Realtime Frontend

**Status:** Not started
**Goal:** Users can see the live model.

**Tasks:**
- [ ] World map view
- [ ] Vessel markers
- [ ] Marker update stream
- [ ] Stale/active visual state
- [ ] Select vessel
- [ ] Metadata panel
- [ ] Connection status
- [ ] Basic filtering if needed

**Checkpoint:** A user can open the app, see live vessel positions,
select one, and understand its latest state.

**Notes:** _(none yet)_

---

## Phase 4: Watched Regions

**Status:** Not started
**Goal:** Users can define regions.

Scoped to boxes or circles for v1, not arbitrary polygons, unless the job
requires it.

**Tasks:**
- [ ] Region creation UI
- [ ] Coordinate input or draw-on-map interaction
- [ ] Persist regions (depends on accounts/persistence decision — see
      requirements.md open questions)
- [ ] List/edit/delete regions
- [ ] Server-side region model
- [ ] Validation of coordinates

**Checkpoint:** A user can define and manage watched regions that the
backend can evaluate.

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

**Tasks:**
- [ ] Keep previous vessel position
- [ ] Evaluate position against regions
- [ ] Support any-ship alerts
- [ ] Support specific-ship alerts
- [ ] Avoid duplicate alerts while ship remains inside
- [ ] Define re-entry behavior
- [ ] Store recent events
- [ ] Test edge cases

**Checkpoint:** When a vessel crosses into a watched region, the system
emits one correct entry event.

**Notes:** _(none yet — this is probably the most important backend
correctness phase)_

---

## Phase 6: Realtime Event Delivery

**Status:** Not started
**Goal:** Users see entry events quickly.

**Tasks:**
- [ ] WebSocket/SSE event stream
- [ ] Event feed UI
- [ ] Reconnect behavior
- [ ] Missed-event recovery
- [ ] Client subscription model
- [ ] Basic rate limiting/backpressure

**Checkpoint:** Connected users see vessel updates and region-entry
events within a few seconds.

**Notes:** _(none yet)_

---

## Phase 7: Hardening

**Status:** Not started
**Goal:** Make it reliable enough for v1.

**Tasks:**
- [ ] Upstream reconnect/retry
- [ ] Tolerate failed polls/stream interruptions
- [ ] Logging and metrics
- [ ] Error handling
- [ ] Load test assumptions
- [ ] Frontend performance with many markers
- [ ] Deploy setup
- [ ] Cost review
- [ ] Rollback plan

**Checkpoint:** The system can run unattended, recover from upstream
failures, and support expected v1 load.

**Notes:** _(none yet)_
