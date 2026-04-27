package main

import (
	"fmt"
	"image"
	"log"
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
