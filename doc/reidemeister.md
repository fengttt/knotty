# Reidemeister-Move Tool

A new toolbar mode that lets the user lasso a region of the diagram with
a free-hand closed curve. On release, the algorithm inspects what's
inside the lasso and, if a legal Reidemeister move is available there,
rewrites the `Diagram` to perform it.

The three moves we want to support, plus their inverse "creation" forms:

- **R1 simplification** — remove a kink (a single self-crossing).
- **R2 simplification** — remove a poke-through bigon (two crossings
  bounding a lens between two arcs).
- **R3** — slide one strand across the crossing of the other two; this
  is the "shift" move that rewires three crossings forming a triangle.
- **R1 / R2 creation** — the inverses, used to *introduce* a kink or a
  poke-through into the diagram.

The work is staged in four phases. Phases 1 + 2 are the cleanup / tidy
phase, which is the typical reason a user reaches for the tool. Phases
3 and 4 finish the toolkit.

## Status / convention notes

In this codebase the `dartGraph` (`cmd/knotty/info.go`) holds the
crossing-level adjacency: at every crossing `v`, `adj[v]` is a CCW-
ordered list of four signed darts; positions 0 and 2 are the
under-strand darts, positions 1 and 3 are the over-strand. A signed
dart `+(arcID+1)` lives at the start of arc `arcID`; `-(arcID+1)` lives
at its end. Arc polylines are stored in `Diagram.Arcs[i].Polyline` with
the first point at the start crossing and the last at the end crossing.
Reidemeister rewrites work by mutating `Diagram.Crossings`,
`Diagram.Arcs`, and the implicit dart adjacency — not by editing the
canvas pixels. After every rewrite we re-render the whole diagram via
`renderDiagram`.

The render path expects 13-point polylines (the resampled control
polygon used by Chaikin smoothing). When a rewrite produces new arcs
we generate fresh 13-point polylines for them — initially as straight
chords between the two endpoint crossings — and rely on the user
running Beautify if they want a nicer layout.

---

## Phase 1 — Toolbar mode and lasso UI

**Goal**: a new toolbar button enters "Reidemeister" mode; in that mode
a press-and-drag paints a free-form curve on the canvas; on release the
curve auto-closes (last point ↔ first point) and is exposed to the move
detector. Until phase 2 lands, "no-op": just drop the lasso and clear
the overlay.

### Toolbar

- New button between Move and Pencil. Glyph: `U+E155 gesture` from
  Material Symbols Outlined (a hand-drawn squiggle, which reads as
  "draw a free-form curve"). Add it to the subset with
  `pyftsubset --unicodes=U+E166,U+E65F,U+E6D0,U+E155`.
- Tool constant `ToolReidemeister = 3` in `scaled_image.go`.

### Lasso state

- Add `lassoPath []image.Point` and `lassoActive bool` fields on
  `scaledImage`.
- In `handleDrawing`, branch on `s.Tool == ToolReidemeister`: instead of
  painting onto `Image`, append the cursor point to `lassoPath` and
  request a full-frame redraw via the existing `OnDiagramChanged`
  hook (we extend it to fire on lasso updates, or use a sibling
  `OnLassoChanged`).
- On press: clear `lassoPath`, set `lassoActive = true`, push first
  point.
- On drag: append cursor point if it's at least 2 px from the last
  recorded point (cheap arc-length filter to keep `lassoPath` bounded).
- On release: snap `lassoPath` closed (push the first point at the
  end), invoke `(*game).doReidemeister(lassoPath)`, then clear the
  lasso state.

### Overlay

- `scaledImage.Render` paints `lassoPath` as a thin dashed/translucent
  stroke layered on top of the canvas. Same source-image-pixel
  coordinate transform as the debug overlay.
- After release, fade the lasso out over ~150 ms (or just clear it
  immediately — fade can come later).

### `doReidemeister` stub

Lives in `cmd/knotty/main.go`. Phase 1: receives the closed lasso path,
logs how many points and the bounding box, returns. Phases 2-4 fill in
the detection-and-rewrite branches.

### Tests

- Pure function `closedPolygonContainsPoint(poly, p) bool` (point-in-
  polygon by even-odd rule, used by detectors). Test on a few
  hand-rolled polygons.
- Tool switch: setting `Tool = ToolReidemeister` makes `handleDrawing`
  route into the lasso-collection branch instead of the pencil branch
  (table test).

---

## Phase 2 — R1 and R2 simplification

**Goal**: when the user lassos a kink or a poke-through bigon, perform
the simplifying rewrite.

### Detection — what's "inside" the lasso

Helper `enclosed` returns:

- the set of crossings whose `image.Point` lies inside the lasso
  polygon;
- the set of arcs that have at least one polyline point strictly inside
  the lasso polygon AND at least one strictly outside (these are the
  arcs that "enter and leave" through the lasso boundary);
- the set of arcs entirely inside the lasso (whose endpoints are both
  inside-the-lasso crossings).

We use the existing `closedPolygonContainsPoint` for the point-in-
polygon test.

A useful intermediate result: for each crossing inside the lasso, list
the four darts and classify each as `inside` (the dart's edge stays in
the lasso — its full polyline is inside) or `outside` (it leaves the
lasso through the boundary). This lets the detectors check
"how many of the crossing's darts go to the outside?".

### R1 detection

A kink at crossing `v` is identified by:

- exactly one crossing inside the lasso, namely `v`;
- exactly one arc inside the lasso (the kink's self-loop), with both
  endpoints at `v`;
- the kink-arc's two darts are at adjacent CCW positions in `adj[v]`
  (positions {0,1}, {1,2}, {2,3}, or {3,0}); the *other two* darts go
  to neighbours outside the lasso.

When all three hold, `v` is an R1 kink and we can remove it.

### R1 rewrite

Let `loop` be the self-loop arc and `tail` be the arc that connects
the *other* two darts of `v` (they must connect through `v` — they're
the same physical strand passing through). To remove the kink:

1. Identify the two "tail" darts of `v`. They are the two darts NOT in
   `loop`. Call them `dIn` (where the strand enters `v`) and `dOut`
   (where it exits, on the opposite side).
2. There is exactly one arc whose end is at `dIn` and another whose
   start is at `dOut` — call them `arcIn` and `arcOut`. (They might be
   the same arc if `arcIn == arcOut`; the strand is a closed loop. We
   handle that as a degenerate "remove the whole component" case for
   correctness.)
3. Splice `arcIn` and `arcOut` into a single arc that no longer passes
   through `v`. The new arc's polyline is `arcIn.Polyline` (with its
   last point dropped) concatenated with `arcOut.Polyline` (resampled
   afterwards to 13 points). Its `Start` is `arcIn.Start`, its `End`
   is `arcOut.End`.
4. Delete `v` from `Diagram.Crossings`, delete `loop` from
   `Diagram.Arcs`, delete `arcIn` and `arcOut` and append the new
   spliced arc.
5. Decrement crossing references in every remaining `Endpoint`:
   crossing indices > `v` shift down by 1. Same for arc indices.

### R2 detection

A poke-through bigon is identified by:

- exactly two crossings inside the lasso, `v` and `w`;
- exactly two arcs whose polyline lies entirely inside the lasso, both
  connecting `v` and `w`. Call them `arcA` and `arcB` (each has
  endpoints {`v`,`w`}; together they form the boundary of the bigon).
- *Crucially*: at `v`, `arcA`'s dart is on the over-strand and `arcB`'s
  dart is on the under-strand (or vice versa); and at `w`, the same
  pairing reverses — `arcA` is now on the under-strand and `arcB` on
  the over-strand. This is the "poke-through" pattern (one strand
  alternates over/under within the bigon). Two crossings that are
  *both* "A over B" form a doubly linked pass, NOT an R2 candidate.

If both crossings carry the alternation, the bigon is removable.

### R2 rewrite

1. Identify the two "outside" darts of `v` (the two not on `arcA` or
   `arcB`); they belong to two arcs that enter the lasso, call them
   `arcL_v` (the over-strand neighbour) and `arcU_v` (the under-strand
   neighbour). Same at `w`: `arcL_w` and `arcU_w`.
2. After removal, the over-strand at `v` should connect directly
   through to the over-strand at `w` (since the over-arc was the one
   that "poked through"). Splice `arcL_v` ↔ `arcL_w` into one arc.
   Similarly splice `arcU_v` ↔ `arcU_w` into another. We end up with
   two new arcs replacing four old arcs.
3. Delete crossings `v` and `w`, delete `arcA`, `arcB`, `arcL_v`,
   `arcL_w`, `arcU_v`, `arcU_w`. Append the two spliced arcs.
4. Re-index: all endpoints whose crossing was > max(`v`,`w`) shift down
   by 2; whose crossing was between `v` and `w` shift down by 1; arc
   indices similarly.
5. Resample each new spliced polyline to 13 points.

### Failure modes

- Lasso encloses 0 or > 2 crossings: not an R1/R2 candidate. Surface a
  short message in `propsArea` ("no R1/R2 move found") and clear the
  lasso.
- Lasso encloses 1 crossing but no arc-inside-lasso (or 2 crossings
  with bigon mismatch on over/under): same.
- Diagram becomes empty (0 crossings) after R1 removal: that's the
  unknot represented by a single `P`-arc loop. Phase 2 doesn't model
  that explicitly — clear the canvas and label it "Unknot".

### Tests

- R1 removal on the right-handed and left-handed kink injected into a
  trefoil. Output PD code must equal the trefoil's PD with that
  crossing removed (i.e. one less crossing, valid 4-valent graph).
- R2 removal on a deliberate poke-through inserted between two arcs of
  4_1. Output PD has 2 fewer crossings; the two surviving strands
  rejoin without changing the underlying knot type (verifiable via
  `NumComponents`).
- Negative cases: lasso a triangle (3 crossings, no R1/R2 match), lasso
  outside the diagram, lasso a bigon with same over-strand twice — all
  should leave the Diagram untouched.

---

## Phase 3 — R3

**Goal**: when the user lassos a triangle of three crossings, slide one
strand across the opposite crossing.

### R3 detection

- Exactly three crossings inside the lasso, `v0`, `v1`, `v2`.
- Exactly three arcs entirely inside the lasso, each connecting a
  different pair of crossings (so the three arcs form a triangle).
- The over/under pattern around the triangle determines which strand
  is "in the middle". In a canonical R3 setup, one strand passes over
  both of the other two strands at its two crossings, another strand
  passes under both, and the third alternates. The over-everything
  strand is the one we can slide. Equivalently: out of the three
  crossings, exactly one is "the strand we want to slide is the over-
  strand at this crossing" — actually all three crossings give us
  data; we pick the strand whose over/under pattern allows the slide.

The detector pseudocode:

```
for each strand (= union of two arcs sharing a "passes-through"
                  vertex; here, the strand running through the
                  triangle that has two crossings on it):
    if strand passes over (or under) all three crossings on its path,
       it's the "outside" strand and the slide is illegal — skip;
    if strand passes over the two crossings it touches inside the
       triangle, it can be slid to the other side;
return the strand that satisfies the slide condition.
```

### R3 rewrite

Geometrically, sliding strand `S` across crossing `c01` (the crossing
of the *other* two strands `S1` and `S2`) means:

1. The two crossings on `S` (call them `c_S1` where `S` meets `S1`,
   and `c_S2` where `S` meets `S2`) move to the opposite side of
   `c01`.
2. The over/under information at `c_S1` and `c_S2` is preserved
   (`S` was on top — it stays on top).
3. The arcs need to be rerouted: the arc between `c_S1` and `c_S2`
   that *was* on the inside of the triangle is now on the outside,
   and the strand `S` now passes between `S1` and `S2` on the *other*
   side of `c01`.

Mechanically:

1. Compute new positions for `c_S1` and `c_S2`. The standard R3 move
   shifts each one across `c01`: `c_S1' = 2·c01 − c_S1` and
   `c_S2' = 2·c01 − c_S2` is the simplest geometric recipe (reflect
   through `c01`). After the rewrite, the user can re-Beautify for a
   tidier layout.
2. Update `Diagram.Crossings[c_S1]` and `Diagram.Crossings[c_S2]` to
   the new points.
3. Re-route the three arcs that bordered the triangle: their endpoints
   change adjacency at `c_S1`, `c_S2`, `c01`. Concretely, the dart
   that pointed "into the triangle" at `c_S1` now points "out of the
   triangle", and the dart on the now-outside arc swaps with the dart
   on the previously-outside arc. The dart-graph rewiring is the
   crux of this phase and we'll write it as an explicit rewrite of
   `(arc.Start.Crossing, arc.End.Crossing, arc.Start.Over,
   arc.End.Over)` for the six darts at the triangle.
4. Resample each affected polyline to 13 points (straight chord
   between the new endpoints).

### Failure modes

- Lasso encloses 3 crossings but the inside arcs aren't a triangle:
  not an R3 — message and bail.
- Triangle exists but no strand satisfies the slide condition (e.g.
  all three pass over their respective crossings, which means we have
  a "triangle of three over-passes" — geometrically impossible in a
  real diagram, but defensively check).

### Tests

- Construct a small synthetic 3-crossing R3 region (e.g. inside a
  larger knot); apply R3 and assert the new diagram has the same
  crossing count, the same number of components, and matching invariants
  (Jones / Alexander, once those land in the algorithms package).
- Idempotency-ish: applying R3 twice in the right way should return to
  the original; lock that in with a test.
- Negative cases: triangles that aren't R3 candidates leave the diagram
  untouched.

---

## Phase 4 — R1 and R2 creation

**Goal**: when the user lassos an *empty* region adjacent to a strand,
introduce a kink (R1) or a poke-through (R2). This is the inverse of
phase 2.

### R1 creation (insert a kink)

The user's lasso is roughly a disk crossed exactly once by the strand.
Detection:

- Lasso encloses 0 crossings.
- Lasso boundary is crossed by exactly one arc, which enters and
  leaves the lasso once. That arc segment will become the kink
  carrier.

Rewrite:

1. Pick a midpoint on the strand-segment that's inside the lasso
   (e.g. the arc-length midpoint of the polyline restricted to inside-
   the-lasso points).
2. Insert a new crossing `v` at that midpoint.
3. The new self-loop "loop arc" has both endpoints at `v` and a tiny
   circular polyline. Default 13-point polyline tracing a small circle
   inscribed in the lasso, sized to fit comfortably (radius ~ 1/4 of
   the lasso's bounding-box diameter).
4. Split the original arc at the chosen midpoint: the part before
   becomes one half-arc ending at `v` (entering on, say, dart position
   0); the part after becomes another half-arc starting at `v` (exiting
   on dart position 2). The loop arc occupies positions 1 and 3.
   Whether the loop is over-strand or under-strand at `v` is a UX
   choice — default to the right-handed kink (positive crossing); the
   user can do another R1 on it if they want the left-handed sign.

### R2 creation (insert a poke-through)

The user's lasso is roughly a disk crossed exactly twice — by two
distinct arcs.

Detection:

- Lasso encloses 0 crossings.
- Lasso boundary is crossed by exactly two arcs, each entering and
  leaving once.

Rewrite:

1. For each of the two arcs, pick the polyline midpoint inside the
   lasso. These two midpoints will become the two new crossings, `v`
   and `w`.
2. Re-route: split each of the two arcs at its midpoint, giving four
   half-arcs. Wire them through `v` and `w` so that one of the two
   strands "pokes over" the other (one over at `v`, under at `w`; and
   vice versa for the other strand). User picks which strand pokes via
   a tiny tooltip-style chooser, or — simpler default — the strand
   with the larger lasso-interior arc-length pokes.
3. Insert the two new "interior" arcs that connect `v` and `w` through
   the bigon (one going clockwise around the bigon, one counter-
   clockwise).
4. Both new bigon-interior arcs get straight 13-point polylines
   between `v` and `w`.

### Failure modes

- Lasso intersects more than two arc-segments, or fewer than one:
  ambiguous / not creation-ready. Bail.
- Lasso encloses any crossings: that's a simplification scenario, not
  creation. Phase 2's path took priority earlier; here we just bail.

### Tests

- Create an R1 kink on a straight arc of the unknot, then run phase 2's
  R1 simplification on it: must return to the original arc count and
  zero-crossing diagram.
- Create an R2 poke-through between two parallel arcs, then phase-2-
  simplify: must round-trip exactly.
- Negative: lasso that crosses three arcs leaves the diagram alone.

---

## Cross-phase concerns

**Move dispatching.** Phase 2's `doReidemeister` runs all available
detectors in priority order: R1 → R2 → R3 → R1-creation → R2-creation.
The first one that matches fires. If nothing matches, surface a short
message and leave the diagram alone.

**Polyline sanity.** Every rewrite ends by re-running
`resampleDiagramArcs(d, attachedArcPoints)` so the renderer's
assumption of small smooth control polygons holds.

**Undo.** The Undo button currently only rolls back Beautify. Phase 1
should snapshot the canvas (and pre-rewrite Diagram, deep-copied) into
`undoSnap` + a parallel `undoDiagram` field so a misfired Reidemeister
move is undoable too.

**File layout.** Add `cmd/knotty/reidemeister.go` for the
detectors and rewrites. Add `cmd/knotty/reidemeister_test.go` for the
test cases. The lasso UI lives in `scaled_image.go` next to the
existing pencil/eraser/move handling.
