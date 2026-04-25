# Beautify Diagram

A "Beautify" operation re-embeds a knot diagram so it looks tidy: round-ish,
balanced, with smooth strands and no near-coincident crossings. The algorithm
is a direct port of the one in
[KnotFolio](https://github.com/kmill/knotfolio) (`src/knotgraph.mjs`,
`KnotGraph.beautify`), based on **the Tutte embedding of a barycentric
subdivision** of the diagram's underlying 4-valent planar graph.

## Input / output

Input is a `cmd/knotty.Diagram`:

```go
type Diagram struct {
    Crossings []image.Point   // 4-valent vertices in pixel coords
    Arcs      []Arc           // polyline strands between crossings
}
```

Topology comes from the dart graph already built by `newDartGraph`: at each
crossing the four incident darts are listed counter-clockwise, with positions
0 and 2 on the under-strand and 1 and 3 on the over-strand. That CCW order is
what makes the graph **planar with a chosen rotation system** — exactly what
the algorithm needs.

Output is a new `Diagram` whose `Crossings` have new `image.Point` positions
and whose `Arcs` have polylines that pass through subdivision midpoints, so a
straight-line render of the polyline already looks like a smooth curve.
Over/under attribution at every crossing is preserved verbatim.

## Algorithm

### 1. Skeleton extraction

A *skeleton* is a planar graph with a fixed cyclic dart order at every
vertex. We use the dart-id convention from `dart_graph.go`: a dart is
`+(arcID+1)` at its start crossing and `-(arcID+1)` at its end crossing.
Edges are pairs `{d, -d}`.

For the typical knot-info diagram (a single connected non-loop component) the
skeleton is identical to `dartGraph`:

- `verts[v].darts` = `dartGraph.adj[v]` (CCW order).
- One edge per `Arc`.
- One face per closed walk of "negate-dart-then-rotate-CCW-by-one".

KnotFolio also handles two edge cases that do not appear in our current
inputs but are easy to preserve:

- **Reidemeister-I loops.** A self-loop edge at a single vertex is replaced by
  inserting a degree-2 vertex on the loop, so no two parallel edges share both
  endpoints. Without this, the Tutte system has a non-unique solution.
- **Unknot components** (a circuit of pure passthroughs with no crossing).
  These are drawn as plain circles, not via Tutte.

### 2. Medial barycentric subdivision

Given a skeleton `G`, `medial(G)` is a new skeleton:

- Every vertex of `G` is kept.
- One **edge vertex** is added in the middle of every edge. After
  subdivision, the original edge `{u, v}` is replaced by two edges
  `{u, e}` and `{e, v}` going through the midpoint `e`.
- One **face vertex** is added at the centroid of every face, connected to
  every edge-midpoint on the face's boundary (degree = face length).

The new vertices' rotation systems are determined locally and the result is
again a planar graph with a consistent CCW rotation at every vertex.

We apply medial subdivision **three times**. Each pass roughly quadruples the
number of vertices and gives Tutte enough degrees of freedom to render arcs
as smooth curves rather than straight chords. After three iterations a
vertex's `type` records its provenance:

| type    | origin                                                        |
| ------- | ------------------------------------------------------------- |
| `vvv`   | original crossing (kept across all three subdivisions)        |
| `evv`   | edge midpoint added in iteration 1                            |
| `fvv`   | face centroid added in iteration 1                            |
| `ev`    | edge midpoint added in iteration 2                            |
| `fv`    | face centroid added in iteration 2                            |
| `e`     | edge midpoint added in iteration 3                            |
| `f`     | face centroid added in iteration 3                            |

After three medial subdivisions, each original arc is a chain of nine
vertices (the two endpoint crossings plus seven intermediate
edge/face-derived vertices). Connecting them by straight segments produces
the visibly curved render.

### 3. Outer-face selection

Tutte's theorem requires one face of the subdivided graph to be designated
as the *outer face* and pinned to a strictly convex polygon. We pick the
largest such face. KnotFolio's heuristic — and ours — is:

> Among vertices of type `fvv`, pick the one with the largest degree.

A type-`fvv` vertex sits at the centroid of an *original* face (one of the
faces that existed before any subdivision), and its degree equals the face's
boundary length in the subdivided graph. This is just "the original face
with the most darts". Choosing the largest face minimises the chance that
the diagram gets cramped against the boundary.

### 4. Tutte embedding

For every vertex of the subdivided graph except the chosen outer face, we
ask:

- If `v` is on the outer face's boundary, pin it on a regular polygon: place
  the boundary vertices at angles `2πi/k` for `i = 0..k-1`.
- Otherwise, require `v` to sit at the centroid of its neighbours:
  `Σ_{u ~ v} (x_u − x_v) = 0` (and the same for `y`).

That's `n − 1` linear equations in `n − 1` unknowns (the outer-face vertex
itself is excluded — it has no position; the face is "at infinity"). We
solve two independent systems, one for `x`, one for `y`, by dense
Gauss–Jordan elimination with partial pivoting. KnotFolio uses the same
straight-up dense `row_reduce` and we keep that — for reasonable inputs (≲ 20
crossings → ≲ 1500 vertices after three subdivisions) the cubic cost is well
under a second.

By **Tutte's theorem (1963)**, if the underlying planar graph is
3-connected and the outer face is convex, the resulting embedding is
straight-line planar (no edges cross). Knot diagrams' subdivided graphs
satisfy this in practice; even when 3-connectivity fails, the embedding is
still useful because the iterated subdivision spreads the vertices out
enough that obvious overlaps are avoided.

### 5. Render back to `Diagram`

- Each original crossing's new screen position is the embedded position of
  the matching `vvv`-vertex, rescaled to fit the canvas (centered, with
  margin so strands are not flush against the edge).
- Each original arc's polyline is the chain of nine embedded positions
  connecting its two endpoint crossings — collected by following the arc's
  edge-subdivision history through the three medial passes.
- Over/under, start/end, and component bookkeeping are unchanged.

The caller renders the resulting `Diagram` the same way it renders any other
diagram. In knotty we additionally rasterise the new `Diagram` straight onto
the canvas (clearing any user pencil strokes) so the user sees the result in
place of the previous picture.

## Complexity

For an `n`-crossing input:

- Vertices in the subdivided graph: roughly `64n` (each medial pass
  multiplies by `~4` in the dense regime).
- Tutte solve: dense Gauss–Jordan, `O((64n)^3) ≈ 2.6·10^5 · n^3`.
- For typical knot-info diagrams (`n ≤ 20`), this is well under one second
  in Go on a laptop. We do not bother with sparse iteration.

## What we did *not* port

- The grid-of-parts layout for diagrams with multiple connected
  components. Knot-info has only single-component knots; we lay out a single
  part centered in the canvas.
- Rendering of pure unknot circles. If we ever want to support links with
  unknotted components, draw them as circles in unused canvas regions.

## Files

- `cmd/knotty/beautify.go` — pure algorithm. Operates on `*Diagram`, no GUI
  imports.
- `cmd/knotty/beautify_test.go` — tests against known knots from the
  dataset (figure-eight, trefoil) checking that the output preserves
  topology (PD code matches up to relabelling) and stays inside the canvas.
- `cmd/knotty/main.go` — adds a "Beautify" button next to "Search" and a
  `doBeautify` handler that takes a snapshot of the canvas, runs
  `convertImage` to recover the diagram, beautifies it, and rasterises the
  result back onto the canvas.

## Reference

- W. T. Tutte, *How to draw a graph*, Proc. London Math. Soc. (3) **13**
  (1963), 743–767.
- KnotFolio: <https://github.com/kmill/knotfolio>, `src/knotgraph.mjs`
  (`skeleton`, `barycentric`, `beautify`).
