package main

import (
	"fmt"
	"image"
	"log"
	"math"
)

// doReidemeister consumes a closed lasso polygon (in source-image pixel
// coordinates, last point == first point) and tries to perform a legal
// Reidemeister move on the attached Diagram. Detectors are tried in
// priority order — R1 first (single-crossing kink), then R2 (two-
// crossing poke-through bigon). Phases 3 and 4 (R3, creation) plug in
// here later.
func (g *game) doReidemeister(closed []image.Point) {
	if g.imageWidget == nil {
		return
	}
	d := g.imageWidget.Diagram
	if d == nil {
		g.propsArea.SetText("reidemeister: no diagram attached (Search/Beautify/Convert first)\n")
		return
	}
	if len(closed) < 4 {
		return
	}

	// Snapshot the canvas for Undo before we mutate the diagram.
	g.snapshotCanvas()

	log.Printf("reidemeister: lasso closed, %d points, diagram has %d crossings, %d arcs",
		len(closed)-1, len(d.Crossings), len(d.Arcs))

	if r1, ok := detectR1(d, closed); ok {
		log.Printf("reidemeister: R1 hit crossing=%d loop=%d carrier1=%d carrier2=%d",
			r1.crossing, r1.loop, r1.carrier1, r1.carrier2)
		applyR1(d, r1)
		resampleDiagramArcs(d, attachedArcPoints)
		renderDiagram(g.imageWidget.Image, d, canvasBG)
		g.propsArea.SetText(fmt.Sprintf("R1: removed kink at crossing %d\n", r1.crossing))
		return
	}
	if r2, ok := detectR2(d, closed); ok {
		log.Printf("reidemeister: R2 hit v=%d w=%d arcA=%d arcB=%d "+
			"vOver=%d vUnd=%d wOver=%d wUnd=%d",
			r2.v, r2.w, r2.arcA, r2.arcB,
			r2.vOverArc, r2.vUndArc, r2.wOverArc, r2.wUndArc)
		applyR2(d, r2)
		resampleDiagramArcs(d, attachedArcPoints)
		renderDiagram(g.imageWidget.Image, d, canvasBG)
		g.propsArea.SetText(fmt.Sprintf(
			"R2: removed bigon (crossings %d, %d)\n", r2.v, r2.w))
		return
	}
	if r3, ok := detectR3(d, closed); ok {
		log.Printf("reidemeister: R3 hit movable=(%d,%d) pivot=%d "+
			"arcs movable=%d a-pivot=%d b-pivot=%d",
			r3.a, r3.b, r3.pivot, r3.arcAB, r3.arcAP, r3.arcBP)
		// Snapshot the outside-of-lasso polyline points and the 6
		// boundary intersections BEFORE the rewrite, so we can verify
		// that R3 leaves everything outside the lasso untouched.
		preOutside := snapshotOutsidePolyline(d, closed)
		preBoundary := snapshotBoundaryCrossings(d, closed)
		for ai, ent := range preBoundary {
			log.Printf("R3-pre  A%d boundary @ %v (k=%d)", ai, ent.bp, ent.k)
		}
		applyR3(d, r3, closed)
		// NO resampleDiagramArcs here — uniform-arc-length resampling
		// would replace the carefully-preserved outside-the-lasso
		// polyline points of every exterior arc that R3 touched. R3
		// is a strictly-inside-the-lasso edit, so the outside has to
		// stay byte-for-byte identical.
		postBoundary := snapshotBoundaryCrossings(d, closed)
		for ai, ent := range postBoundary {
			log.Printf("R3-post A%d boundary @ %v (k=%d)", ai, ent.bp, ent.k)
		}
		verifyOutsideUnchanged(d, closed, preOutside)
		renderDiagram(g.imageWidget.Image, d, canvasBG)
		g.propsArea.SetText(fmt.Sprintf(
			"R3: slid strand at C%d/C%d across C%d\n", r3.a, r3.b, r3.pivot))
		return
	}

	diag := reidemeisterDiagnose(d, closed)
	log.Print("reidemeister: no R1/R2 found.\n", diag)
	g.propsArea.SetText(diag)
}

// reidemeisterDiagnose builds a multi-line description of what the
// detector saw inside the lasso and why no move matched. Used to
// surface the failure to the user when neither R1 nor R2 fires.
//
// Includes the lasso's bounding box and the position of every
// crossing (with inside-flag), so we can tell whether the user's
// lasso is missing the underlying Diagram coordinates entirely
// (e.g. the loaded picture's crossings sit at different pixel
// positions than the user expects).
func reidemeisterDiagnose(d *Diagram, lasso []image.Point) string {
	bbox := lassoBBox(lasso)
	var insideC []int
	var b []byte
	b = append(b, "RDX: no R1/R2 found.\n"...)
	b = append(b, fmt.Sprintf("RDX: lasso bbox x=[%d,%d] y=[%d,%d] (%d pts)\n",
		bbox.Min.X, bbox.Max.X, bbox.Min.Y, bbox.Max.Y, len(lasso)-1)...)
	b = append(b, fmt.Sprintf("RDX: %d crossings, %d arcs total\n",
		len(d.Crossings), len(d.Arcs))...)
	for i, c := range d.Crossings {
		inside := closedPolygonContainsPoint(lasso, c)
		flag := "OUT"
		if inside {
			flag = "IN "
			insideC = append(insideC, i)
		}
		b = append(b, fmt.Sprintf("RDX:   C%d %s @ (%d,%d)\n", i, flag, c.X, c.Y)...)
	}
	for i, a := range d.Arcs {
		any, all := arcInLassoStats(lasso, a.Polyline)
		state := "out"
		if all {
			state = "FULL"
		} else if any {
			state = "part"
		}
		ptsIn := 0
		for _, p := range a.Polyline {
			if closedPolygonContainsPoint(lasso, p) {
				ptsIn++
			}
		}
		b = append(b, fmt.Sprintf("RDX:   A%d %s C%d(%s)→C%d(%s) %d/%d pts in\n",
			i, state, a.Start.Crossing, overTag(a.Start.Over),
			a.End.Crossing, overTag(a.End.Over), ptsIn, len(a.Polyline))...)
	}
	_ = insideC
	return string(b)
}

// lassoBBox returns the axis-aligned bounding rectangle of the lasso
// polygon, in source-image pixel coordinates.
func lassoBBox(lasso []image.Point) image.Rectangle {
	if len(lasso) == 0 {
		return image.Rectangle{}
	}
	r := image.Rectangle{Min: lasso[0], Max: lasso[0]}
	for _, p := range lasso[1:] {
		if p.X < r.Min.X {
			r.Min.X = p.X
		}
		if p.X > r.Max.X {
			r.Max.X = p.X
		}
		if p.Y < r.Min.Y {
			r.Min.Y = p.Y
		}
		if p.Y > r.Max.Y {
			r.Max.Y = p.Y
		}
	}
	return r
}

func overTag(over bool) string {
	if over {
		return "over"
	}
	return "under"
}

// arcInLassoStats reports whether at least one polyline point lies
// strictly inside the lasso polygon (any) and whether every point does
// (all). A polyline that has at least one point inside but at least
// one outside is a "boundary-crossing" arc — it enters or leaves the
// lasso.
func arcInLassoStats(lasso []image.Point, poly []image.Point) (any, all bool) {
	all = len(poly) > 0
	for _, p := range poly {
		if closedPolygonContainsPoint(lasso, p) {
			any = true
		} else {
			all = false
		}
	}
	return any, all
}

// closedPolygonContainsPoint reports whether p lies inside the closed
// polygon described by poly using the even-odd / ray-cast rule. poly
// is expected to be a closed polygon (first point repeated at the
// end), but the algorithm only requires that consecutive entries form
// the polygon's edges and that the polygon close back somehow — extra
// or missing closing duplicates don't change the parity result.
//
// The ray cast goes in the +X direction. Edges that cross the ray
// flip the inside/outside parity. Vertices that lie exactly on the
// ray are tie-broken with the standard "lower-Y endpoint counts,
// upper-Y endpoint does not" rule so each edge contributes once.
func closedPolygonContainsPoint(poly []image.Point, p image.Point) bool {
	n := len(poly)
	if n < 3 {
		return false
	}
	inside := false
	x, y := float64(p.X), float64(p.Y)
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := float64(poly[i].X), float64(poly[i].Y)
		xj, yj := float64(poly[j].X), float64(poly[j].Y)
		if (yi > y) != (yj > y) {
			xIntersect := (xj-xi)*(y-yi)/(yj-yi) + xi
			if x < xIntersect {
				inside = !inside
			}
		}
		j = i
	}
	return inside
}

// ----- R1 simplification (kink removal) -----

// r1Hit names the rewrite a successful R1 detector found: the kink
// crossing, the self-loop arc, and the two non-loop arcs at the
// crossing (whose endpoints there are on opposite strands — one
// over, one under). The two non-loop arcs are spliced into a single
// arc bypassing the crossing; their dart directions (Start vs End at
// v) are not constrained, since spliceArcsThroughCrossings handles
// arbitrary orientations.
type r1Hit struct {
	crossing int
	loop     int
	carrier1 int // non-loop arc whose endpoint at v has Over = !L.End.Over (matches L.Start.Over)
	carrier2 int // non-loop arc whose endpoint at v has Over = !L.Start.Over (matches L.End.Over)
	// carrierIsSelfLoop is true when carrier1 == carrier2 — the only
	// non-loop arc at v is itself a self-loop (the diagram is the
	// terminal "1 crossing + 2 self-loops" unknot rendering). The
	// rewrite drops the crossing and BOTH self-loop arcs, promoting
	// the carrier's polyline to a free-floating Loop.
	carrierIsSelfLoop bool
}

// detectR1 returns an r1Hit if the lasso encloses exactly one crossing
// and exactly one fully-inside arc that is a self-loop at that
// crossing whose two endpoints there have opposite over flags (the
// loop crosses itself), and the other two darts at the crossing
// belong to two distinct non-loop arcs with one over endpoint and
// one under endpoint there.
//
// Note: convertImage may label arc directions inconsistently — at a
// kink crossing, both non-loop arcs may *start* at the crossing
// rather than the textbook "one starts, one ends" pattern. This
// detector identifies them by over-flag, not by direction, and
// leaves the splice mechanics to spliceArcsThroughCrossings.
func detectR1(d *Diagram, lasso []image.Point) (r1Hit, bool) {
	var zero r1Hit
	insideC := []int{}
	for i, c := range d.Crossings {
		if closedPolygonContainsPoint(lasso, c) {
			insideC = append(insideC, i)
		}
	}
	if len(insideC) != 1 {
		return zero, false
	}
	v := insideC[0]
	insideA := []int{}
	for i, a := range d.Arcs {
		_, all := arcInLassoStats(lasso, a.Polyline)
		if all {
			insideA = append(insideA, i)
		}
	}
	if len(insideA) != 1 {
		return zero, false
	}
	loop := insideA[0]
	if d.Arcs[loop].Start.Crossing != v || d.Arcs[loop].End.Crossing != v {
		return zero, false
	}
	// Loop must really cross itself at v: opposite over flags.
	if d.Arcs[loop].Start.Over == d.Arcs[loop].End.Over {
		return zero, false
	}
	// Walk every other arc and bin by (which endpoint is at v, what
	// over-flag does that endpoint have). For a clean kink there
	// must be exactly one over and one under non-loop endpoint at v
	// — and they must come from two distinct arcs (a second self-
	// loop at v is a degenerate "double kink at v" we don't handle).
	overEnd := -1
	undEnd := -1
	for i, a := range d.Arcs {
		if i == loop {
			continue
		}
		if a.Start.Crossing == v {
			if a.Start.Over {
				if overEnd != -1 {
					return zero, false
				}
				overEnd = i
			} else {
				if undEnd != -1 {
					return zero, false
				}
				undEnd = i
			}
		}
		if a.End.Crossing == v {
			if a.End.Over {
				if overEnd != -1 {
					return zero, false
				}
				overEnd = i
			} else {
				if undEnd != -1 {
					return zero, false
				}
				undEnd = i
			}
		}
	}
	if overEnd < 0 || undEnd < 0 {
		return zero, false
	}
	selfLoopCarrier := overEnd == undEnd
	// Pair the carriers with the loop's same-over-flag endpoints. The
	// loop's start has flag X; carrier whose v-endpoint is X is the
	// "before-the-kink" side (carrier1). carrier2 mirrors that with
	// the opposite flag.
	c1, c2 := overEnd, undEnd
	if !d.Arcs[loop].Start.Over {
		c1, c2 = undEnd, overEnd
	}
	return r1Hit{
		crossing:          v,
		loop:              loop,
		carrier1:          c1,
		carrier2:          c2,
		carrierIsSelfLoop: selfLoopCarrier,
	}, true
}

// applyR1 performs the kink-removal rewrite: drops the crossing and
// the self-loop arc, splices the two carrier arcs into one continuous
// arc that bypasses where the crossing used to be. d is mutated in
// place; arc indices and crossing indices in the resulting Diagram
// are renumbered so they remain contiguous.
//
// Special case: when the carrier itself is a self-loop (the only
// non-loop arc at v has both endpoints there too — the terminal
// "1 crossing + 2 self-loops" unknot rendering), the rewrite drops
// the crossing and BOTH self-loop arcs and promotes the carrier's
// polyline (closed at v) into Diagram.Loops as a free-floating
// closed curve.
func applyR1(d *Diagram, r r1Hit) {
	if r.carrierIsSelfLoop {
		// Promote the carrier's polyline into a loop. Both endpoints
		// were at the (now-removed) crossing, so the polyline is
		// already closed.
		carrierPoly := append([]image.Point(nil), d.Arcs[r.carrier1].Polyline...)
		drop := map[int]bool{r.loop: true, r.carrier1: true}
		newArcs := make([]Arc, 0, len(d.Arcs)-len(drop))
		for i, a := range d.Arcs {
			if drop[i] {
				continue
			}
			newArcs = append(newArcs, a)
		}
		d.Arcs = newArcs
		d.Loops = append(d.Loops, carrierPoly)
		d.Crossings = append(d.Crossings[:r.crossing], d.Crossings[r.crossing+1:]...)
		for i := range d.Arcs {
			fixCrossingRef(&d.Arcs[i].Start.Crossing, r.crossing)
			fixCrossingRef(&d.Arcs[i].End.Crossing, r.crossing)
		}
		return
	}

	merged := spliceArcsThroughCrossings(d, r.carrier1, r.crossing, r.carrier2, r.crossing)

	drop := map[int]bool{r.loop: true, r.carrier1: true, r.carrier2: true}
	newArcs := make([]Arc, 0, len(d.Arcs)-3+1)
	for i, a := range d.Arcs {
		if drop[i] {
			continue
		}
		newArcs = append(newArcs, a)
	}
	newArcs = append(newArcs, merged)
	d.Arcs = newArcs

	// Drop crossing v and renumber every Crossing reference in
	// remaining arcs.
	d.Crossings = append(d.Crossings[:r.crossing], d.Crossings[r.crossing+1:]...)
	for i := range d.Arcs {
		fixCrossingRef(&d.Arcs[i].Start.Crossing, r.crossing)
		fixCrossingRef(&d.Arcs[i].End.Crossing, r.crossing)
	}
}

// fixCrossingRef shifts a single crossing reference down by 1 if the
// referenced index was greater than removed, leaves it alone if less,
// and treats "equal" as a bug (the rewrite should have detached every
// reference to removed before deletion).
func fixCrossingRef(idx *int, removed int) {
	if *idx > removed {
		*idx--
	}
}

// ----- R2 simplification (poke-through bigon) -----

// r2Hit names the rewrite a successful R2 detector found: the two
// crossings v and w bounding the bigon, the two interior arcs arcA
// and arcB, and the four "exterior" carrier arcs. The exterior arcs
// are the four arcs that have ONE endpoint at v or w and are not
// arcA or arcB. They get spliced pairwise: vOver↔wUnder (one strand)
// and vUnder↔wOver (the other strand). All indices refer to the
// input Diagram.
type r2Hit struct {
	v, w     int
	arcA     int
	arcB     int
	vOverArc int // exterior arc whose endpoint at v is the over-strand
	vUndArc  int // exterior arc whose endpoint at v is the under-strand
	wOverArc int
	wUndArc  int
}

// arcEndpointAtCrossing returns the index in {0=Start, 1=End} of the
// endpoint that touches v; if both endpoints touch v (the arc is a
// self-loop at v) it returns -1; if neither touches it returns -2.
func arcEndpointAtCrossing(a Arc, v int) int {
	switch {
	case a.Start.Crossing == v && a.End.Crossing == v:
		return -1
	case a.Start.Crossing == v:
		return 0
	case a.End.Crossing == v:
		return 1
	}
	return -2
}

// arcOverAtCrossing returns the over flag of arc a at crossing v.
// Caller must have already checked that a touches v at exactly one
// endpoint (arcEndpointAtCrossing returned 0 or 1).
func arcOverAtCrossing(a Arc, v int) bool {
	if a.Start.Crossing == v {
		return a.Start.Over
	}
	return a.End.Over
}

// detectR2 returns an r2Hit if the lasso encloses exactly two
// crossings and exactly two fully-inside arcs which together form the
// bigon's boundary, with the canonical R2 over/under alternation
// (one arc is over at one crossing and under at the other, the other
// arc is the inverse).
func detectR2(d *Diagram, lasso []image.Point) (r2Hit, bool) {
	var zero r2Hit
	insideC := []int{}
	for i, c := range d.Crossings {
		if closedPolygonContainsPoint(lasso, c) {
			insideC = append(insideC, i)
		}
	}
	if len(insideC) != 2 {
		return zero, false
	}
	v, w := insideC[0], insideC[1]

	insideA := []int{}
	for i, a := range d.Arcs {
		_, all := arcInLassoStats(lasso, a.Polyline)
		if all {
			insideA = append(insideA, i)
		}
	}
	if len(insideA) != 2 {
		return zero, false
	}
	arcA, arcB := insideA[0], insideA[1]
	// Both interior arcs must connect v and w.
	for _, ai := range insideA {
		s, e := d.Arcs[ai].Start.Crossing, d.Arcs[ai].End.Crossing
		ok := (s == v && e == w) || (s == w && e == v)
		if !ok {
			return zero, false
		}
	}

	overAvAtV := arcOverAtCrossing(d.Arcs[arcA], v)
	overAvAtW := arcOverAtCrossing(d.Arcs[arcA], w)
	overBvAtV := arcOverAtCrossing(d.Arcs[arcB], v)
	overBvAtW := arcOverAtCrossing(d.Arcs[arcB], w)
	// R2 (the "twist" — one strand passes over another twice and can
	// be lifted off): both interior arcs keep the SAME over flag at
	// both crossings (one arc is over→over, the other is under→
	// under). Trefoil-style "linked" bigons have the alternating
	// pattern (over→under and under→over) and cannot be removed by
	// R2 alone — those are correctly rejected here.
	if overAvAtV != overAvAtW || overBvAtV != overBvAtW {
		return zero, false
	}
	// At each crossing the two interior arcs must lie on opposite
	// strands (one over, one under).
	if overAvAtV == overBvAtV || overAvAtW == overBvAtW {
		return zero, false
	}

	// Find exterior carrier arcs at v and w. We need ONE over-strand
	// dart and ONE under-strand dart at each crossing. Each
	// non-bigon arc at v contributes either one dart (its single
	// endpoint at v with that endpoint's over flag) or — if it's a
	// self-loop at v — two darts (Start with Start.Over, End with
	// End.Over). For a clean R1-removable kink at v, those two
	// flags are different, so a self-loop fills both vOver and vUnd
	// from the same arc index. applyR2 detects that case
	// (vOverArc == vUndArc) and does a single-arc merge instead of
	// the standard two-arc splice.
	vOver, vUnd, wOver, wUnd := -1, -1, -1, -1
	consider := func(a Arc, ai, target int, slot func(int, *int) bool) {}
	_ = consider
	setSlot := func(slot *int, ai int) bool {
		if *slot != -1 && *slot != ai {
			return false
		}
		*slot = ai
		return true
	}
	for i, a := range d.Arcs {
		if i == arcA || i == arcB {
			continue
		}
		if a.Start.Crossing == v {
			if a.Start.Over {
				if !setSlot(&vOver, i) {
					return zero, false
				}
			} else {
				if !setSlot(&vUnd, i) {
					return zero, false
				}
			}
		}
		if a.End.Crossing == v {
			if a.End.Over {
				if !setSlot(&vOver, i) {
					return zero, false
				}
			} else {
				if !setSlot(&vUnd, i) {
					return zero, false
				}
			}
		}
		if a.Start.Crossing == w {
			if a.Start.Over {
				if !setSlot(&wOver, i) {
					return zero, false
				}
			} else {
				if !setSlot(&wUnd, i) {
					return zero, false
				}
			}
		}
		if a.End.Crossing == w {
			if a.End.Over {
				if !setSlot(&wOver, i) {
					return zero, false
				}
			} else {
				if !setSlot(&wUnd, i) {
					return zero, false
				}
			}
		}
	}
	if vOver < 0 || vUnd < 0 || wOver < 0 || wUnd < 0 {
		return zero, false
	}
	// Both v and w being self-loop carriers at once is too degenerate
	// to handle cleanly — bail.
	if vOver == vUnd && wOver == wUnd {
		return zero, false
	}
	return r2Hit{v: v, w: w, arcA: arcA, arcB: arcB,
		vOverArc: vOver, vUndArc: vUnd, wOverArc: wOver, wUndArc: wUnd}, true
}

// applyR2 performs the bigon-removal rewrite: drops the two
// crossings, the two interior arcs, splices the four exterior
// carriers pairwise so the two strands continue smoothly past where
// the bigon used to be. d is mutated in place; arc / crossing
// indices are renumbered to stay contiguous.
//
// In an R2-removable (twist) bigon, each strand keeps the same
// over/under flag at both crossings — the over-strand stays on top
// through both, the under-strand stays under at both. The splice
// pairing is (vOver ↔ wOver) and (vUnd ↔ wUnd).
//
// Special case: if one side's exterior is a single self-loop arc
// (a kink at v or w whose two darts at that crossing fill both the
// over and under carrier slots), the four-dart pairing collapses to
// a single merged arc that absorbs the self-loop's polyline as the
// "bridge" between the two strands.
func applyR2(d *Diagram, r r2Hit) {
	wIsSelfLoop := r.wOverArc == r.wUndArc
	vIsSelfLoop := r.vOverArc == r.vUndArc

	drop := map[int]bool{
		r.arcA: true, r.arcB: true,
		r.vOverArc: true, r.vUndArc: true,
		r.wOverArc: true, r.wUndArc: true,
	}

	var newR2Arcs []Arc
	switch {
	case wIsSelfLoop:
		newR2Arcs = []Arc{spliceR2WithSelfLoop(d, r, true)}
	case vIsSelfLoop:
		newR2Arcs = []Arc{spliceR2WithSelfLoop(d, r, false)}
	default:
		merged1 := spliceArcsThroughCrossings(d, r.vOverArc, r.v, r.wOverArc, r.w)
		merged2 := spliceArcsThroughCrossings(d, r.vUndArc, r.v, r.wUndArc, r.w)
		newR2Arcs = []Arc{merged1, merged2}
	}

	newArcs := make([]Arc, 0, len(d.Arcs)-len(drop)+len(newR2Arcs))
	for i, a := range d.Arcs {
		if drop[i] {
			continue
		}
		newArcs = append(newArcs, a)
	}
	newArcs = append(newArcs, newR2Arcs...)
	d.Arcs = newArcs

	// Drop both crossings; remove the larger index first so the
	// smaller-index removal isn't shifted.
	hi, lo := r.v, r.w
	if hi < lo {
		hi, lo = lo, hi
	}
	d.Crossings = append(d.Crossings[:hi], d.Crossings[hi+1:]...)
	d.Crossings = append(d.Crossings[:lo], d.Crossings[lo+1:]...)
	for i := range d.Arcs {
		fixCrossingRef(&d.Arcs[i].Start.Crossing, hi)
		fixCrossingRef(&d.Arcs[i].End.Crossing, hi)
		fixCrossingRef(&d.Arcs[i].Start.Crossing, lo)
		fixCrossingRef(&d.Arcs[i].End.Crossing, lo)
	}
}

// spliceR2WithSelfLoop produces the single merged arc that replaces
// the bigon when one side's exterior carrier is a self-loop arc
// (e.g. a kink directly attached to the bigon at w). The merged arc
// runs:
//
//	(vOver's far end) → through where v was → through where w was
//	  → through the self-loop's polyline → through where w was again
//	  → through where v was again → (vUnd's far end)
//
// The strand entering via vOver "becomes" the strand exiting via
// vUnd (they're the same physical strand connected by the self-
// loop). Polyline gaps at the crossing-removal points get smoothed
// out by the renderer's Chaikin pass.
//
// When wIsSelfLoop is false the same logic runs with v and w swapped.
func spliceR2WithSelfLoop(d *Diagram, r r2Hit, wIsSelfLoop bool) Arc {
	var pivot int            // crossing the two non-self-loop carriers attach to
	var slArc int            // self-loop arc id
	var leftArc, rightArc int // the two non-self-loop carriers at pivot (over and under)
	if wIsSelfLoop {
		pivot = r.v
		slArc = r.wOverArc
		leftArc = r.vOverArc
		rightArc = r.vUndArc
	} else {
		pivot = r.w
		slArc = r.vOverArc
		leftArc = r.wOverArc
		rightArc = r.wUndArc
	}

	a1 := d.Arcs[leftArc]
	sla := d.Arcs[slArc]
	a2 := d.Arcs[rightArc]

	// poly1: leftArc's polyline oriented to end at the pivot, with
	// the pivot vertex itself dropped.
	var startEP Endpoint
	var poly1 []image.Point
	if a1.End.Crossing == pivot {
		poly1 = a1.Polyline[:len(a1.Polyline)-1]
		startEP = a1.Start
	} else {
		poly1 = reversePolyline(a1.Polyline)
		poly1 = poly1[:len(poly1)-1]
		startEP = a1.End
	}

	// poly2: self-loop's interior, oriented over → under so the
	// stitched merged arc enters the loop on the over side and
	// exits on the under side. If sla.Start.Over is true, the
	// polyline already runs over → under; otherwise reverse it.
	slPoly := sla.Polyline
	if !sla.Start.Over {
		slPoly = reversePolyline(slPoly)
	}
	var poly2 []image.Point
	if len(slPoly) >= 2 {
		poly2 = slPoly[1 : len(slPoly)-1]
	}

	// poly3: rightArc's polyline oriented to start at the pivot,
	// with the pivot vertex itself dropped.
	var endEP Endpoint
	var poly3 []image.Point
	if a2.Start.Crossing == pivot {
		poly3 = a2.Polyline[1:]
		endEP = a2.End
	} else {
		poly3 = reversePolyline(a2.Polyline)
		poly3 = poly3[1:]
		endEP = a2.Start
	}

	poly := make([]image.Point, 0, len(poly1)+len(poly2)+len(poly3))
	poly = append(poly, poly1...)
	poly = append(poly, poly2...)
	poly = append(poly, poly3...)
	return Arc{Polyline: poly, Start: startEP, End: endEP}
}

// spliceArcsThroughCrossings merges two arcs into one by removing
// their endpoints at v (for the first arc) and w (for the second),
// stitching the polylines so the new arc runs from the first arc's
// "other" endpoint, through where v and w used to be, to the second
// arc's "other" endpoint.
//
// The arc whose endpoint-at-v is its End is "outgoing" through v;
// its polyline is read forward up to but not including the v vertex.
// The arc whose endpoint-at-v is its Start is "incoming" through v;
// its polyline is reversed for the same effect. Same logic at w.
func spliceArcsThroughCrossings(d *Diagram, ai1, v, ai2, w int) Arc {
	a1 := d.Arcs[ai1]
	a2 := d.Arcs[ai2]
	var poly1 []image.Point
	var startEP Endpoint
	if a1.End.Crossing == v {
		// a1 ends at v; keep its polyline forward, drop the v vertex.
		poly1 = a1.Polyline[:len(a1.Polyline)-1:len(a1.Polyline)-1]
		startEP = a1.Start
	} else {
		// a1 starts at v; reverse the polyline so the now-removed v
		// is at the end.
		poly1 = reversePolyline(a1.Polyline)
		poly1 = poly1[:len(poly1)-1]
		startEP = a1.End
	}
	var poly2 []image.Point
	var endEP Endpoint
	if a2.Start.Crossing == w {
		// a2 starts at w; keep forward, drop the w vertex by skipping
		// poly2[0].
		poly2 = a2.Polyline[1:]
		endEP = a2.End
	} else {
		// a2 ends at w; reverse so the w vertex is at the start, then
		// drop it.
		poly2 = reversePolyline(a2.Polyline)
		poly2 = poly2[1:]
		endEP = a2.Start
	}
	poly := make([]image.Point, 0, len(poly1)+len(poly2))
	poly = append(poly, poly1...)
	poly = append(poly, poly2...)
	return Arc{Polyline: poly, Start: startEP, End: endEP}
}

// reversePolyline returns a fresh slice with poly's points in reverse
// order. The input is left untouched.
func reversePolyline(poly []image.Point) []image.Point {
	out := make([]image.Point, len(poly))
	for i, p := range poly {
		out[len(poly)-1-i] = p
	}
	return out
}

// ----- R3 (strand-slide across a triangle of crossings) -----

// r3Hit names a successful R3 detection. Three crossings inside the
// lasso form a triangle (each pair connected by exactly one fully-
// inside arc). The "movable" strand is the one whose two endpoints
// inside the triangle have the same over flag (both over or both
// under) — that's the strand that's consistently on top (or bottom)
// at its two triangle crossings, and the only one that can be slid
// across the third crossing without changing the over/under
// pattern. After R3, the two crossings on that strand reflect
// through the pivot crossing.
type r3Hit struct {
	a, b  int // the two crossings the movable strand passes through (these MOVE)
	pivot int // the third crossing (the one the movable strand slides past — STAYS PUT)
	arcAB int // index of the movable-strand arc between a and b
	arcAP int // index of the arc between a and pivot
	arcBP int // index of the arc between b and pivot
	// New positions for a and b after the slide. a slides along
	// strand Y to the opposite side of pivot, B slides along
	// strand Z, computed as midpoints between pivot and the lasso-
	// boundary crossing of the strand-Y / strand-Z exterior arcs at
	// pivot. Pivot's own position is unchanged.
	newA, newB image.Point
}

// detectR3 returns an r3Hit if the lasso encloses exactly three
// crossings, exactly three fully-inside arcs (each connecting a
// distinct pair of those crossings), and exactly one of those arcs
// has matching over flags at both endpoints — the canonical R3
// movable-strand pattern.
func detectR3(d *Diagram, lasso []image.Point) (r3Hit, bool) {
	var zero r3Hit
	insideC := []int{}
	for i, c := range d.Crossings {
		if closedPolygonContainsPoint(lasso, c) {
			insideC = append(insideC, i)
		}
	}
	if len(insideC) != 3 {
		return zero, false
	}

	insideA := []int{}
	for i, a := range d.Arcs {
		_, all := arcInLassoStats(lasso, a.Polyline)
		if all {
			insideA = append(insideA, i)
		}
	}
	if len(insideA) < 3 {
		return zero, false
	}

	pairKey := func(p, q int) [2]int {
		if p > q {
			return [2]int{q, p}
		}
		return [2]int{p, q}
	}
	// Filter the fully-inside arcs down to ones that connect TWO
	// DIFFERENT triangle crossings — those are the triangle's three
	// edges. Self-loops fully inside (kinks at a triangle crossing)
	// or arcs to non-triangle crossings are tolerated and left
	// untouched: detectR3's job is to find a clean triangle.
	arcByPair := map[[2]int]int{}
	insideC0, insideC1, insideC2 := insideC[0], insideC[1], insideC[2]
	isTriV := func(c int) bool {
		return c == insideC0 || c == insideC1 || c == insideC2
	}
	for _, ai := range insideA {
		a := d.Arcs[ai]
		s, e := a.Start.Crossing, a.End.Crossing
		if s == e {
			continue // self-loop — not a triangle arc
		}
		if !isTriV(s) || !isTriV(e) {
			continue
		}
		k := pairKey(s, e)
		if _, exists := arcByPair[k]; exists {
			return zero, false // a second arc between the same pair → not a clean triangle
		}
		arcByPair[k] = ai
	}
	if len(arcByPair) != 3 {
		return zero, false
	}

	// Find an arc whose two endpoints have the same over flag — that's
	// the consistently-over (or consistently-under) strand that R3
	// can slide. If multiple arcs match (i.e. two of the three strands
	// are both consistent), prefer the over-strand: it's the
	// canonical "movable" one in textbook R3 diagrams.
	movableArc := -1
	movableArcOver := false
	var movableAB [2]int
	for k, ai := range arcByPair {
		a := d.Arcs[ai]
		sOver := arcOverAtCrossing(a, k[0])
		eOver := arcOverAtCrossing(a, k[1])
		if sOver != eOver {
			continue
		}
		// Match. Take it unless we already have an over-strand match
		// and this one is under (the over-strand wins ties).
		if movableArc == -1 || (sOver && !movableArcOver) {
			movableArc = ai
			movableArcOver = sOver
			movableAB = k
		}
	}
	if movableArc < 0 {
		return zero, false
	}

	a, b := movableAB[0], movableAB[1]
	pivot := -1
	for _, c := range insideC {
		if c != a && c != b {
			pivot = c
			break
		}
	}
	if pivot < 0 {
		return zero, false
	}

	hit := r3Hit{
		a:     a,
		b:     b,
		pivot: pivot,
		arcAB: movableArc,
		arcAP: arcByPair[pairKey(a, pivot)],
		arcBP: arcByPair[pairKey(b, pivot)],
	}
	// Compute A_new and B_new along strand Y and strand Z respectively.
	// Strand Y is the one passing through A and pivot (= arcAP); strand
	// Z passes through B and pivot (= arcBP). For each, find the
	// exterior arc at pivot on that strand (matching over flag at
	// pivot), find its lasso-boundary crossing point, and put the new
	// position at the midpoint between pivot and that boundary.
	overYatPivot := arcOverAtCrossing(d.Arcs[hit.arcAP], pivot)
	overZatPivot := arcOverAtCrossing(d.Arcs[hit.arcBP], pivot)
	hit.newA = newPosAlongStrandExterior(d, lasso, hit, overYatPivot)
	hit.newB = newPosAlongStrandExterior(d, lasso, hit, overZatPivot)
	if hit.newA == d.Crossings[pivot] || hit.newB == d.Crossings[pivot] {
		// Couldn't find a clean exterior arc on one of the strands —
		// degenerate setup, bail.
		return zero, false
	}
	return hit, true
}

// newPosAlongStrandExterior returns a point inside the lasso, on
// strand-side defined by overFlag at pivot, at the midpoint between
// pivot and the lasso-boundary crossing of the exterior arc at
// pivot whose endpoint there matches overFlag. Returns the pivot
// itself when no matching exterior arc can be found.
func newPosAlongStrandExterior(d *Diagram, lasso []image.Point, r r3Hit, overFlag bool) image.Point {
	pivotPt := d.Crossings[r.pivot]
	for i, arc := range d.Arcs {
		if i == r.arcAB || i == r.arcAP || i == r.arcBP {
			continue
		}
		var match bool
		if arc.Start.Crossing == r.pivot && arc.Start.Over == overFlag {
			match = true
		}
		if arc.End.Crossing == r.pivot && arc.End.Over == overFlag {
			match = true
		}
		if !match {
			continue
		}
		// Find this arc's lasso-boundary crossing point.
		k, bp := findLassoBoundaryCrossing(arc.Polyline, lasso)
		if k < 0 {
			// Polyline doesn't cross the boundary — exterior arc is
			// somehow entirely inside or entirely outside; skip.
			continue
		}
		newPt := image.Point{
			X: (pivotPt.X + bp.X) / 2,
			Y: (pivotPt.Y + bp.Y) / 2,
		}
		if !closedPolygonContainsPoint(lasso, newPt) {
			continue
		}
		return newPt
	}
	return pivotPt
}

// applyR3 performs the strand-slide per doc/reidemeister_r3.md. The
// non-movable strands (AC and BC) keep their physical curves
// completely intact — we relabel where the crossings sit along
// those curves: the crossing that was at position A_old becomes the
// new C-endpoint along its strand (i.e. the same point on the
// curve, but the dart graph now says C is there); A's identity
// moves to a new point further along the curve, past the (still
// fixed) C. The movable strand AB gets a fresh inside-the-lasso
// curve from its lasso-boundary entry to A_new to B_new to its
// lasso-boundary exit. Over/under flags update to reflect each
// endpoint's new anchor crossing.
func applyR3(d *Diagram, r r3Hit, lasso []image.Point) {
	overACatA := arcOverAtCrossing(d.Arcs[r.arcAP], r.a)
	overACatC := arcOverAtCrossing(d.Arcs[r.arcAP], r.pivot)
	overBCatB := arcOverAtCrossing(d.Arcs[r.arcBP], r.b)
	overBCatC := arcOverAtCrossing(d.Arcs[r.arcBP], r.pivot)
	overABatA := arcOverAtCrossing(d.Arcs[r.arcAB], r.a)
	overABatB := arcOverAtCrossing(d.Arcs[r.arcAB], r.b)

	tri := []int{r.arcAB, r.arcAP, r.arcBP}
	extACatA := findExtArc(d, r.a, tri, overACatA)
	extACatC := findExtArc(d, r.pivot, tri, overACatC)
	extBCatB := findExtArc(d, r.b, tri, overBCatB)
	extBCatC := findExtArc(d, r.pivot, tri, overBCatC)
	extABatA := findExtArc(d, r.a, tri, overABatA)
	extABatB := findExtArc(d, r.b, tri, overABatB)

	// Snapshot polylines BEFORE any mutation (since strand AC's
	// processing rewires endpoints that strand BC's processing
	// would otherwise look up by their old identities, etc.).
	if extACatA >= 0 && extACatC >= 0 {
		processStrandACorBC(d, r.arcAP, extACatA, extACatC,
			r.a, r.pivot, overACatA, overACatC, lasso)
	}
	if extBCatB >= 0 && extBCatC >= 0 {
		processStrandACorBC(d, r.arcBP, extBCatB, extBCatC,
			r.b, r.pivot, overBCatB, overBCatC, lasso)
	}

	// Movable strand AB: A and B are now at their new positions.
	// arcAB connects A_new to B_new with a fresh straight chord.
	// Each exterior AB arc keeps its outside-of-lasso portion and
	// reconnects from the lasso boundary to the new crossing.
	_ = overABatA
	_ = overABatB
	straightenArc := func(ai int) {
		a := &d.Arcs[ai]
		if len(a.Polyline) < 2 {
			return
		}
		p0 := d.Crossings[a.Start.Crossing]
		p1 := d.Crossings[a.End.Crossing]
		n := len(a.Polyline)
		for i := 0; i < n; i++ {
			t := float64(i) / float64(n-1)
			a.Polyline[i] = image.Point{
				X: int(math.Round(float64(p0.X)*(1-t) + float64(p1.X)*t)),
				Y: int(math.Round(float64(p0.Y)*(1-t) + float64(p1.Y)*t)),
			}
		}
	}
	straightenArc(r.arcAB)
	rebuildAt := func(ai, vertex int) {
		if ai < 0 {
			return
		}
		a := &d.Arcs[ai]
		if len(a.Polyline) < 2 {
			return
		}
		if a.Start.Crossing == vertex {
			rebuildExteriorArcAtStart(a, d.Crossings[vertex], lasso)
		}
		if a.End.Crossing == vertex {
			rebuildExteriorArcAtEnd(a, d.Crossings[vertex], lasso)
		}
	}
	rebuildAt(extABatA, r.a)
	rebuildAt(extABatB, r.b)
}

// processStrandACorBC implements the user's spec for one of the two
// non-movable strands. It rewires the three arcs along that strand
// so the order of crossings flips (X→A→C→Y becomes X→C→A→Y, with A
// at a new position along the original curve past C). Polyline
// points are PRESERVED — the physical strand curve doesn't move,
// just the crossings' anchor points along it.
//
//   - aiInterior: arc connecting movableEnd (A or B) and pivot (C).
//   - extAtMov: exterior arc anchored at movableEnd before rewire.
//   - extAtPivot: exterior arc anchored at pivot before rewire.
//   - movableEnd: A or B (will move to a new position).
//   - pivot: C (stays put).
//   - overAtMov, overAtPivot: strand's over flags at those crossings.
func processStrandACorBC(d *Diagram, aiInterior, extAtMov, extAtPivot, movableEnd, pivot int, overAtMov, overAtPivot bool, lasso []image.Point) {
	// Orient each polyline along the strand walk direction
	// "external_at_movableEnd → movableEnd → pivot → external_at_pivot":
	//   poly1 = ext_at_movableEnd polyline ending at movableEnd.
	//   poly2 = aiInterior polyline starting at movableEnd, ending at pivot.
	//   poly3 = ext_at_pivot polyline starting at pivot, ending at external.
	poly1 := orientPolylineEndingAt(d.Arcs[extAtMov], movableEnd)
	poly2 := orientPolylineStartingAt(d.Arcs[aiInterior], movableEnd)
	poly3 := orientPolylineStartingAt(d.Arcs[extAtPivot], pivot)

	// Compute the arc-length midpoint of poly3 from pivot to the
	// lasso boundary crossing. A_new (or B_new) sits at that point
	// — interpolated within whatever segment of poly3 contains it,
	// so the original polyline points are preserved on either side.
	firstHalf, secondHalf, ANewPos, ok := splitPolylineAtArcLengthMid(poly3, lasso)
	if !ok {
		return
	}

	// Concatenate poly1 + poly2 (skipping the duplicated movableEnd
	// vertex at the join). That walks external_at_movableEnd → pivot
	// (since poly2 ends at pivot). This is the new polyline for the
	// "external_at_movableEnd → C" rewired arc.
	newPolyForExtAtMov := make([]image.Point, 0, len(poly1)+len(poly2)-1)
	newPolyForExtAtMov = append(newPolyForExtAtMov, poly1...)
	newPolyForExtAtMov = append(newPolyForExtAtMov, poly2[1:]...)

	// Interior arc's new polyline: pivot → A_new (firstHalf).
	newPolyForInterior := firstHalf

	// External-at-pivot arc's new polyline: A_new → external (secondHalf).
	newPolyForExtAtPivot := secondHalf

	// Update arc records. Each arc keeps the OPPOSITE endpoint that
	// pointed outside the triangle; only the inside endpoint changes.
	extAtMovExt := otherEndpoint(d.Arcs[extAtMov], movableEnd)
	d.Arcs[extAtMov] = Arc{
		Start:    extAtMovExt,
		End:      Endpoint{Crossing: pivot, Over: overAtPivot},
		Polyline: newPolyForExtAtMov,
	}
	d.Arcs[aiInterior] = Arc{
		Start:    Endpoint{Crossing: pivot, Over: overAtPivot},
		End:      Endpoint{Crossing: movableEnd, Over: overAtMov},
		Polyline: newPolyForInterior,
	}
	extAtPivotExt := otherEndpoint(d.Arcs[extAtPivot], pivot)
	d.Arcs[extAtPivot] = Arc{
		Start:    Endpoint{Crossing: movableEnd, Over: overAtMov},
		End:      extAtPivotExt,
		Polyline: newPolyForExtAtPivot,
	}
	d.Crossings[movableEnd] = ANewPos
}

// splitPolylineAtArcLengthMid takes a polyline that starts inside
// the lasso and crosses the boundary somewhere, and returns the
// polyline split at the arc-length midpoint between poly[0] and
// the boundary crossing point. The midpoint is included in both
// halves (last point of firstHalf, first point of secondHalf).
// Outside-of-lasso polyline points (everything past the boundary
// crossing index in the original) end up at the tail of secondHalf
// — preserved exactly. Returns ok=false when poly doesn't cross
// the boundary.
func splitPolylineAtArcLengthMid(poly []image.Point, lasso []image.Point) (firstHalf, secondHalf []image.Point, mid image.Point, ok bool) {
	bIdx, bp := findLassoBoundaryCrossing(poly, lasso)
	if bIdx <= 0 {
		return nil, nil, image.Point{}, false
	}
	type seg struct {
		a, b image.Point
		l    float64
	}
	var segs []seg
	var totalLen float64
	for i := 0; i < bIdx-1; i++ {
		s := seg{a: poly[i], b: poly[i+1]}
		s.l = math.Hypot(float64(s.b.X-s.a.X), float64(s.b.Y-s.a.Y))
		segs = append(segs, s)
		totalLen += s.l
	}
	// Final partial segment from the last inside point to the
	// bisected boundary point.
	tail := seg{a: poly[bIdx-1], b: bp}
	tail.l = math.Hypot(float64(tail.b.X-tail.a.X), float64(tail.b.Y-tail.a.Y))
	totalLen += tail.l
	if totalLen <= 0 {
		return nil, nil, image.Point{}, false
	}
	target := totalLen / 2
	var acc float64
	splitSegIdx := -1
	var splitMid image.Point
	for i, s := range segs {
		if acc+s.l >= target && s.l > 0 {
			t := (target - acc) / s.l
			splitMid = image.Point{
				X: int(math.Round(float64(s.a.X) + t*float64(s.b.X-s.a.X))),
				Y: int(math.Round(float64(s.a.Y) + t*float64(s.b.Y-s.a.Y))),
			}
			splitSegIdx = i
			break
		}
		acc += s.l
	}
	if splitSegIdx < 0 {
		// Midpoint is in the trailing partial segment (poly[bIdx-1] → bp).
		if tail.l > 0 {
			t := (target - acc) / tail.l
			splitMid = image.Point{
				X: int(math.Round(float64(tail.a.X) + t*float64(tail.b.X-tail.a.X))),
				Y: int(math.Round(float64(tail.a.Y) + t*float64(tail.b.Y-tail.a.Y))),
			}
			splitSegIdx = bIdx - 1
		} else {
			return nil, nil, image.Point{}, false
		}
	}
	// firstHalf: poly[0..splitSegIdx] then mid (closes at A_new).
	firstHalf = make([]image.Point, 0, splitSegIdx+2)
	firstHalf = append(firstHalf, poly[:splitSegIdx+1]...)
	firstHalf = append(firstHalf, splitMid)
	// secondHalf: mid then poly[splitSegIdx+1..end] (preserves
	// inside remnants AND outside-of-lasso tail).
	secondHalf = make([]image.Point, 0, len(poly)-splitSegIdx)
	secondHalf = append(secondHalf, splitMid)
	secondHalf = append(secondHalf, poly[splitSegIdx+1:]...)
	return firstHalf, secondHalf, splitMid, true
}

// orientPolylineEndingAt returns arc.Polyline reversed if necessary
// so polyline[len-1] is at the position of vertex.
func orientPolylineEndingAt(arc Arc, vertex int) []image.Point {
	if arc.End.Crossing == vertex {
		return append([]image.Point(nil), arc.Polyline...)
	}
	out := make([]image.Point, len(arc.Polyline))
	for i, p := range arc.Polyline {
		out[len(arc.Polyline)-1-i] = p
	}
	return out
}

// orientPolylineStartingAt returns arc.Polyline reversed if
// necessary so polyline[0] is at the position of vertex.
func orientPolylineStartingAt(arc Arc, vertex int) []image.Point {
	if arc.Start.Crossing == vertex {
		return append([]image.Point(nil), arc.Polyline...)
	}
	out := make([]image.Point, len(arc.Polyline))
	for i, p := range arc.Polyline {
		out[len(arc.Polyline)-1-i] = p
	}
	return out
}

// otherEndpoint returns whichever of arc.Start and arc.End doesn't
// reference vertex. Caller is responsible for ensuring vertex
// appears at exactly one endpoint (i.e. arc isn't a self-loop at v).
func otherEndpoint(arc Arc, vertex int) Endpoint {
	if arc.Start.Crossing == vertex {
		return arc.End
	}
	return arc.Start
}

// findExtArc returns the index of an arc that has an endpoint at
// vertex with the given over flag and isn't in the exclude list.
// Returns -1 if no such arc exists.
func findExtArc(d *Diagram, vertex int, exclude []int, overFlag bool) int {
	excluded := make(map[int]bool, len(exclude))
	for _, e := range exclude {
		excluded[e] = true
	}
	for i, arc := range d.Arcs {
		if excluded[i] {
			continue
		}
		if arc.Start.Crossing == vertex && arc.Start.Over == overFlag {
			return i
		}
		if arc.End.Crossing == vertex && arc.End.Over == overFlag {
			return i
		}
	}
	return -1
}

// rewireArcEndpoint changes whichever endpoint of arc currently
// references (oldCrossing, oldOver) so it instead references
// (newCrossing, newOver). Used by R3 to flip the order of crossings
// along a non-movable strand.
func rewireArcEndpoint(arc *Arc, oldCrossing, newCrossing int, oldOver, newOver bool) {
	if arc.Start.Crossing == oldCrossing && arc.Start.Over == oldOver {
		arc.Start.Crossing = newCrossing
		arc.Start.Over = newOver
		return
	}
	if arc.End.Crossing == oldCrossing && arc.End.Over == oldOver {
		arc.End.Crossing = newCrossing
		arc.End.Over = newOver
		return
	}
}

// rebuildExteriorArcAtStart rewrites the inside-the-lasso portion of
// a's polyline (the start side) so it goes from the moved crossing
// position newPos to the first preserved outside-the-lasso polyline
// point, leaving the outside portion byte-for-byte intact.
//
// The line segment from newPos to poly[k] (the first outside point)
// crosses the lasso boundary somewhere — we don't insert an explicit
// boundary point because doing so would change the count of "outside
// points" by ±1, which the R3-OK verification flags as a violation.
// The renderer's Chaikin smoothing makes the crossing point look
// natural anyway.
func rebuildExteriorArcAtStart(a *Arc, newPos image.Point, lasso []image.Point) {
	k, _ := findLassoBoundaryCrossing(a.Polyline, lasso)
	if k < 0 {
		// Polyline is entirely on one side of the lasso boundary
		// (rare; only happens for very tight or very wide lassoes).
		// Just snap the endpoint.
		a.Polyline[0] = newPos
		return
	}
	out := make([]image.Point, 0, 1+len(a.Polyline)-k)
	out = append(out, newPos)
	out = append(out, a.Polyline[k:]...)
	a.Polyline = out
}

// rebuildExteriorArcAtEnd rewrites the inside-the-lasso portion of
// a's polyline (the end side) so it goes from the lasso-boundary
// crossing to the moved crossing position newPos, leaving the
// outside-the-lasso portion exactly intact.
//
// Uses findLastLassoBoundaryCrossing rather than the
// first-transition variant: for self-loop arcs (Start.Crossing ==
// End.Crossing both at a moved crossing) the polyline crosses the
// lasso boundary twice, and the End-side rebuild needs to target
// the SECOND (last) transition so it doesn't collapse what the
// Start-side rebuild already preserved.
func rebuildExteriorArcAtEnd(a *Arc, newPos image.Point, lasso []image.Point) {
	k, _ := findLastLassoBoundaryCrossing(a.Polyline, lasso)
	if k < 0 {
		a.Polyline[len(a.Polyline)-1] = newPos
		return
	}
	out := make([]image.Point, 0, k+1)
	out = append(out, a.Polyline[:k]...)
	out = append(out, newPos)
	a.Polyline = out
}

// findLastLassoBoundaryCrossing walks poly from the END backward and
// returns the LAST boundary transition (the one closest to the end
// of the polyline). Returns the index k of the polyline point on
// the END side of the boundary plus an interpolated boundary point.
// (-1, _) when poly never transitions.
func findLastLassoBoundaryCrossing(poly []image.Point, lasso []image.Point) (int, image.Point) {
	if len(poly) < 2 {
		return -1, image.Point{}
	}
	insideEnd := closedPolygonContainsPoint(lasso, poly[len(poly)-1])
	for i := len(poly) - 2; i >= 0; i-- {
		if closedPolygonContainsPoint(lasso, poly[i]) != insideEnd {
			a, b := poly[i], poly[i+1]
			aInside := !insideEnd
			for j := 0; j < 8; j++ {
				m := image.Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
				if closedPolygonContainsPoint(lasso, m) == aInside {
					a = m
				} else {
					b = m
				}
			}
			return i + 1, image.Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
		}
	}
	return -1, image.Point{}
}

// boundaryEntry captures where an arc's polyline crosses the lasso
// boundary, plus the polyline index of the first point on the
// opposite side of the boundary. Used to verify R3 leaves the
// outside-of-lasso part unchanged.
type boundaryEntry struct {
	bp image.Point
	k  int
}

// snapshotBoundaryCrossings returns one boundaryEntry per arc whose
// polyline straddles the lasso boundary. Arcs entirely inside or
// entirely outside the lasso are skipped.
func snapshotBoundaryCrossings(d *Diagram, lasso []image.Point) map[int]boundaryEntry {
	out := map[int]boundaryEntry{}
	for i, a := range d.Arcs {
		k, bp := findLassoBoundaryCrossing(a.Polyline, lasso)
		if k < 0 {
			continue
		}
		out[i] = boundaryEntry{bp: bp, k: k}
	}
	return out
}

// snapshotOutsidePolyline records, for every arc, just the polyline
// points that lie outside the lasso. After R3, these points must be
// identical (byte-for-byte) — that's the user-visible "the diagram
// outside the lasso area must not change" contract.
func snapshotOutsidePolyline(d *Diagram, lasso []image.Point) map[int][]image.Point {
	out := map[int][]image.Point{}
	for i, a := range d.Arcs {
		var keep []image.Point
		for _, p := range a.Polyline {
			if !closedPolygonContainsPoint(lasso, p) {
				keep = append(keep, p)
			}
		}
		if keep != nil {
			out[i] = keep
		}
	}
	return out
}

// verifyOutsideUnchanged compares the post-R3 outside-the-lasso
// points of each arc with the pre-R3 snapshot. Every arc's outside
// portion must be identical, in the same order. Differences are
// logged as warnings — they signal a bug in the R3 rewrite.
func verifyOutsideUnchanged(d *Diagram, lasso []image.Point, pre map[int][]image.Point) {
	violations := 0
	for i, a := range d.Arcs {
		var post []image.Point
		for _, p := range a.Polyline {
			if !closedPolygonContainsPoint(lasso, p) {
				post = append(post, p)
			}
		}
		preList, ok := pre[i]
		if !ok {
			if len(post) > 0 {
				log.Printf("R3-VIOL A%d: had no outside points pre-R3 but %d post", i, len(post))
				violations++
			}
			continue
		}
		if len(preList) != len(post) {
			log.Printf("R3-VIOL A%d: outside count %d → %d", i, len(preList), len(post))
			violations++
			continue
		}
		for j := range preList {
			if preList[j] != post[j] {
				log.Printf("R3-VIOL A%d: outside[%d] %v → %v", i, j, preList[j], post[j])
				violations++
			}
		}
	}
	if violations == 0 {
		log.Printf("R3-OK: every arc's outside-of-lasso polyline is identical pre/post.")
	} else {
		log.Printf("R3-VIOL: %d outside-of-lasso polyline points changed.", violations)
	}
}

// findLassoBoundaryCrossing locates the first index k in poly such
// that poly[k-1] and poly[k] are on opposite sides of the lasso
// polygon (one inside, one outside). Returns k together with a
// bisected approximation of the boundary point. Returns (-1, _) if
// the polyline never transitions.
func findLassoBoundaryCrossing(poly []image.Point, lasso []image.Point) (int, image.Point) {
	if len(poly) < 2 {
		return -1, image.Point{}
	}
	inside0 := closedPolygonContainsPoint(lasso, poly[0])
	for i := 1; i < len(poly); i++ {
		if closedPolygonContainsPoint(lasso, poly[i]) != inside0 {
			a, b := poly[i-1], poly[i]
			aInside := inside0
			for j := 0; j < 8; j++ {
				m := image.Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
				if closedPolygonContainsPoint(lasso, m) == aInside {
					a = m
				} else {
					b = m
				}
			}
			return i, image.Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
		}
	}
	return -1, image.Point{}
}
