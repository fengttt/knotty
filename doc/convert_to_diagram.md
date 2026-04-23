# Knotfolio — "Convert to diagram" algorithm

Here's the algorithm that turns a photo/sketch of a knot into a diagram in knotfolio. It's a two-stage pipeline: first get a clean B&W image, then extract the combinatorial knot diagram.

## Stage 0 — Image import (`KnotImageImportView.mjs`, `paint` method)

Classic computer-vision cleanup, interactive sliders for every parameter:

1. Draw the cropped image into a temp canvas.
2. Small **stack blur** (radius = `blur`) — denoise.
3. Grayscale (average RGB), invert if the user checked "invert" (for chalkboard-style pictures).
4. Compute a second, heavily blurred copy with radius = `adaptive` (~20 px). This is the **local mean**.
5. **Adaptive threshold**: pixel → black if `(gray − local_mean)/255 ≤ threshold`, else white. (Adaptive, not global, so it handles uneven lighting in photos.)

Result: a clean 1-bit raster. On "Accept" this becomes a `KnotRasterView` — its buffer is an `Int8Array` where each pixel holds `0` (empty), a small positive integer (component "color", 1–n), or `-1` (error marker).

## Stage 1 — Painting (user-supplied structure)

Important: the program does **not** infer over/under from pixels. The user paints with a brush that:

- Uses **distinct colors for distinct link components**.
- For each over-stroke, automatically leaves a **gap** in any existing ink it crosses (`PAINT_GAP` pixels clear around the brush core).

So "go over" strokes look continuous; "go under" strokes are broken into pieces by those gaps. This is the signal the conversion will exploit.

## Stage 2 — Convert to diagram (`KnotRasterView.convert()`)

### 2a. Morphological thinning (`clean_up`)

Reduce every painted stroke to a 1-pixel-wide skeleton.

For each colored pixel, look at its 3×3 neighborhood; fill implied diagonal corners (so orthogonal neighbors imply a diagonal). Then walk the 8 neighbors counterclockwise and count:
- `pcount` — number of same-color neighbors
- `ccount` — number of state transitions

Delete the pixel if `pcount == 0` (isolated) or (`ccount == 2` and `pcount` in a band) — equivalent to "removing it wouldn't disconnect its component and it isn't an endpoint". Iterate until nothing changes (two passes, tight band 3–4 then looser 2–6). Finish with a one-pass "tip" removal (pixels with a single same-color neighbor).

### 2b. Spur deletion

Walk from every endpoint up to `SPUR_LENGTH` pixels; if you reach a junction in that budget, erase the spur. This cleans noise hairs left by thinning.

### 2c. Sanity check

Any surviving pixel with **>2 same-color neighbors** is a junction → user error ("understrand fused to overstrand"). Mark and bail out.

### 2d. Endpoint matching (the clever part)

Every pixel with exactly one same-color neighbor is an endpoint. By construction each color must have an even number of them — they come in pairs, one pair per under-crossing of that component.

For each color, match the endpoints into pairs:

1. For every candidate pair `(p, q)`:
   - Walk the straight line `p→q` over a **thickened** copy of the image, counting how many *other* strands it crosses (`pcount`).
   - If it crosses 0 other strands (`pcount ≤ 1`) → reject (backtracking, not a real over-strand gap).
   - Otherwise `count = min(2, pcount−1)`, score = `max(0, (dist − DIAGRAM_LINE_WIDTH·count) / count)`. Shorter spans that cross exactly one strand win.
2. **Greedy min-weight matching** on those scores.
3. **2-opt** pass: for every pair of chosen edges, try swapping the endpoint pairing if either alternative has lower total score. Repeat until stable.
4. Must be a perfect matching; cross-component matched edges must not geometrically intersect each other (that would be a self-intersecting under-strand).

### 2e. Build the planar graph

- **Walk each skeleton component** starting from its endpoints (and then any remaining closed loops, for unknots). Each walk produces a sequence of vertices and edges tagged `(v1, v2, color, over=true)`.
- **Inject the matched segments** as straight under-strand edges with `over=false`. When a matched segment passes through an existing vertex or crosses an existing edge, split both at the intersection — those intersections *are* the crossings of the diagram.

Now you have a set of vertices and edges where every crossing is a 4-valent vertex and every waypoint is 2-valent.

### 2f. Build the combinatorial rotation system

For each vertex, collect the incident "darts" (directed half-edges, sign = direction).
- 2 darts → a `P` node (pass-through).
- 4 darts → an `X` node (crossing). Sort its 4 darts by the `pseudo_angle` to each neighbor so they go counterclockwise around the vertex (this gives the **planar rotation system**). Rotate the list so the under-dart is first (PD convention).

Consistency checks at each `X`:
- Opposite darts alternate over/under (transversality).
- Opposite pairs share a color (the over-strand continues straight across; ditto the under).

### 2g. Orient and hand off

`KnotGraph.ensure_orientation()` picks consistent orientations per component, then the graph is wrapped in a `KnotDiagramView` — from here on you have a PD code and can run Alexander / Jones / etc.

## What makes it tractable

Two design choices shoulder most of the load:

- **Over/under is encoded by the user's paintbrush gaps, not recovered from image intensity.** You don't need to detect shading or line-stop cues — every under-strand is already a literal break in the raster. Conversion = gluing the breaks back across the correct over-strand.
- **Components are encoded by color.** Endpoint matching is done **per color**, which keeps the matching problem small (each color has only a handful of endpoints) and keeps distinct link components from getting tangled together in the matcher.

The hard combinatorial work is just (1) thinning to a skeleton, (2) a small weighted perfect-matching problem per color, (3) standard planar-graph-from-polyline arithmetic, (4) sorting the 4 darts at each crossing by angle.
