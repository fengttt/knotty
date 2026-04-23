# Computing components, PD, and DT from a diagram

This note covers how knotfolio turns a combinatorial diagram into three
numeric pieces of information: **number of link components**, the
**PD (planar-diagram) notation**, and the **DT (Dowker-Thistlethwaite)
notation**. The source of truth is `src/knotgraph.mjs` in knotfolio; the
companion `convert_to_diagram.md` in this directory covers how the
diagram itself is reconstructed from pixels.

Throughout, a "diagram" is an embedded 4-valent planar multigraph with
crossings marked over/under — i.e., what you get at the end of
`convertImage` in `cmd/knotty/convert.go`.

## Dart graph: the representation

Knotfolio stores the diagram as three parallel arrays:

- `verts[i]`   — the plane coordinates of vertex `i`.
- `edges[e]`   — `[v1, v2, component_color]`. A straight segment between
  two vertices. `component_color` lets colored strokes survive the
  pipeline (links are colored one component per stroke).
- `adjs[v]`    — the **ordered** list of **darts** incident to `v`,
  in counter-clockwise order.

A **dart** is a signed edge id: `+(e+1)` means "edge `e`, with vertex
`v` as its start", `-(e+1)` means "edge `e`, with vertex `v` as its
end". Every edge has exactly two darts (one at each endpoint). So a
dart uniquely identifies an (edge, end) pair; its sign tells you which
end.

Vertex types:
- **P** (degree 2) — a path/waypoint. Passes straight through.
- **X** (degree 4) — a crossing. The CCW dart order is
  `[u_in, o_out, u_out, o_in]` where even indices `{0, 2}` are on the
  under-strand and odd indices `{1, 3}` are on the over-strand. The
  two under darts are on the same line (180° apart geometrically);
  same for the two over darts.

Helpers you'll see used everywhere:
- `dart_start(d)` / `dart_end(d)` — the vertex a dart points from / to.
- `opp_dart(d) = -d` — flip the dart to point the other direction
  along the same edge.
- `next_dart(d)` / `prev_dart(d)` — rotate CCW/CW one step in the adj
  list at `dart_start(d)`.
- `through_dart(d)` — **cross the edge, continue the same strand on
  the far side.** For P vertices (degree 2) this is
  `next_dart(opp(d))`. For X vertices it's
  `next_dart(next_dart(opp(d)))` — i.e., step by 2 in the adj list of
  the far vertex, which picks out the diagonally-opposite dart on the
  same strand.
- `dart_circuit(d)` — the closed orbit of `d` under `through_dart`.
  A circuit represents one link component, traversed in one direction.

The `ensure_orientation()` method walks each circuit and flips edges
as needed so that, within a circuit, every `dart_start(d) == edge[0]`.
After that, positive darts all point "with" the chosen circuit
orientation and negative darts point against it.

## Number of components

One circuit per component. Each edge contributes exactly two darts
(`+e` and `-e`), and every dart is in exactly one circuit — so if we
union-find darts by circuit we can just iterate edges and count how
many circuits we hit:

```
seen = set()
components = 0
for e in 0 .. |edges|-1:
    if e in seen: continue
    components += 1
    for d in dart_circuit(e+1):
        seen.add(|d| - 1)     # mark the underlying edge
```

(Knotfolio's `num_components` has a tiny quirk — it stores `edge_i` in
`seen_darts` rather than the edge index derived from `d`. The two are
the same thing for the starter of each circuit, so the count is still
correct.)

## PD notation

A PD presentation labels each **arc** (maximal over-strand between
successive under-crossings) with an integer `1..N` and records every
crossing as a 4-tuple of arc labels. Knotfolio's `get_pd(oriented)`
does it in two passes:

**Pass 1 — number the arcs.** Process circuits in ascending order of
`edges[*][2]` (component color) so the numbering is deterministic and
component-grouped. For a circuit with no crossings (a loose unknot
loop), emit one `P[a, a]` entry and give every dart on it the same arc
id. Otherwise:

1. Rotate the circuit so the first dart is at a crossing.
2. Walk the circuit. Every time you hit a crossing vertex, **bump the
   arc counter**. Store the current `arc_id` against every dart
   visited (both `+d` and `-d`, since the arc id is edge-based).

The "bump at every crossing" is subtly wrong if you read it as
"bump when passing **through** a crossing" — an arc is broken only at
the **under-crossings**. But because the circuit passes each crossing
twice (once over, once under), and the bumps happen on both visits,
the numbering lines up: consecutive under-crossings along the strand
produce the `1 → 2`, `2 → 3`, ... arc boundaries, while the over
visits generate "phantom" increments that are invisible from the
outside.

Wait — actually that's not right either. Let me re-read knotfolio.
It **does** bump on every crossing visit. The trick: what matters is
that every dart-occurrence at a crossing starts a fresh arc. Since an
over-strand has both its halves landing on crossings (it enters and
exits a crossing in one continuous stroke), bumping at every X-visit
still assigns one arc id per over-strand segment between consecutive
under-crossings along the circuit. The "extra" arc labels at over
crossings coincide with the labels already assigned to the far ends
of the over segment via the same walk — so the PD ends up consistent.
The deciding fact: there are `2c` darts at crossings around a
`c`-crossing diagram; the walk issues `2c` fresh arc ids — but only
`2c` arcs is exactly right for a diagram with `c` crossings (each
crossing emits 4 arc-incidences, each arc has 2 endpoints, so
`#arcs * 2 = #crossing_incidences = 4c`, i.e., `#arcs = 2c`).

**Pass 2 — emit the PD entries.** For each X vertex, look up the 4
darts' arc ids. Rotate the 4-tuple so the incoming under-strand dart
(the `a`-position) is at index 0: knotfolio picks this by checking
that `adj[2]` is oriented into the vertex (`!dart_oriented(adj[2])`),
and rotates by 2 if it isn't. Then:

- Unoriented mode: emit `X[a, b, c, d]`.
- Oriented mode: emit `Xp` (positive crossing, both over darts point
  out along the orientation — "right-handed") or `Xm` (negative) based
  on whether `adj[1]` is oriented.

The "a goes to c" convention means, in the standard picture

```
      c   b
       \ /
        X
       / \
      d   a
```

the under-strand runs `a → c` (enters bottom-right, exits top-left).
`b` and `d` are the two ends of the over-strand; `Xp` vs `Xm` says
whether that over-strand runs `d → b` (positive) or `b → d` (negative).

## DT notation

DT only makes sense for a **single-component** diagram (a knot, not a
link). Knotfolio returns `null` otherwise.

The classical DT code, for a `c`-crossing knot, is a sequence of `c`
even integers obtained as follows:

1. Orient the knot and walk around it, labeling the **under-strand
   entries at each crossing** `1, 2, 3, ..., 2c`. (Equivalently, label
   **every** crossing incidence along the strand; each crossing gets
   two labels, one odd and one even — a theorem, not a coincidence.)
2. For each odd number `2k-1`, record the even number `m_k` at the
   **same crossing**. Sign is used to record over/under orientation at
   the odd entry: `+m_k` if the strand at the odd visit is under,
   `-m_k` if it is over. (Conventions vary; knotfolio uses
   `+` for under and flips if needed so entry 0 is positive.)

Knotfolio's `get_dt()` produces a **canonical** DT code by trying many
starting points and orientations and taking the lexicographically
smallest:

```
circuit = dart_circuit(1).filter(d => X at dart_start(d))
n = circuit.length          # == 2 * crossing_number

for k in 0 .. n-1:
    # index each filtered dart as (position - k) mod n
    # so we effectively start the circuit at dart `circuit[k]`
    build one DT code as above
    # if the first entry is negative, flip all signs
    append to codes

# now try the reverse orientation
circuit = dart_circuit(-1).filter(...)
for k in 0 .. n-1: same as above

return min(codes)  # lex smallest
```

The pairing step — given a filtered dart `d` at crossing `v`, find the
other circuit-dart at the same crossing — is done with `next_dart` /
`prev_dart`. At `v` the four darts of the X alternate over-strand and
under-strand in CCW order, so `next_dart(d)` and `prev_dart(d)` are
the two perpendicular-strand darts. Exactly one of them is in the
filtered circuit (it's the one whose index is in `dart_crossing`; the
other points the wrong way).

Why alternating odd/even: it's a classical theorem. Walk the strand;
at every crossing you visit, the **other** visit to that crossing
must come an odd number of steps later, because the two visits bound
a closed sub-loop whose self-intersection with the rest of the
diagram has even parity modulo 2. Equivalently, the sub-arc between
the two visits crosses every other crossing an even number of times,
hence contributes even to the step count.

## Implementation notes for knotty's Go port

In knotty, the output of `convertImage` (see `cmd/knotty/diagram.go`)
is a `Diagram { Crossings []Point, Arcs []Arc }`. Arcs are polylines
between crossings with over/under per endpoint — the P vertices
knotfolio keeps have already been absorbed into the polyline. That is
enough to rebuild the dart graph:

- Vertices = crossings. Each has degree 4.
- Each arc contributes two darts: `+arc_id` at `arc.Start.Crossing`
  and `-arc_id` at `arc.End.Crossing`. Over/under at each dart comes
  from the arc's endpoint `Over` flag.
- Self-loop arcs (an arc whose two endpoints land at the same
  crossing) show up as two darts on the same vertex. Polyline has ≥ 3
  points so the directions are still well-defined.

To order the four darts at a crossing counter-clockwise, compute the
direction of each arc **as it leaves the crossing** (use the next
polyline point for the `+` dart and the previous-to-last for the `-`
dart) and sort by angle. Flip the y-axis since pixel coordinates are
y-down, so "CCW on screen" is "CCW in math coordinates". Then rotate
the 4-tuple so `adj[0]` is on the under strand.

With that adj in hand, `through_dart` is
`adj[dart_end(d)][(adj_index(opp(d)) + 2) % 4]` and circuits /
components / PD / DT all follow directly.
