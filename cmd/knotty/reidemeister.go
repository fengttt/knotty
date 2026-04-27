package main

import (
	"fmt"
	"image"
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

	if r1, ok := detectR1(d, closed); ok {
		applyR1(d, r1)
		resampleDiagramArcs(d, attachedArcPoints)
		renderDiagram(g.imageWidget.Image, d, canvasBG)
		g.propsArea.SetText(fmt.Sprintf("R1: removed kink at crossing %d\n", r1.crossing))
		return
	}
	if r2, ok := detectR2(d, closed); ok {
		applyR2(d, r2)
		resampleDiagramArcs(d, attachedArcPoints)
		renderDiagram(g.imageWidget.Image, d, canvasBG)
		g.propsArea.SetText(fmt.Sprintf(
			"R2: removed bigon (crossings %d, %d)\n", r2.v, r2.w))
		return
	}

	insideCrossings := 0
	for _, c := range d.Crossings {
		if closedPolygonContainsPoint(closed, c) {
			insideCrossings++
		}
	}
	g.propsArea.SetText(fmt.Sprintf(
		"reidemeister: no R1/R2 found in lasso (%d crossings inside)\n", insideCrossings))
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
// crossing, the self-loop arc, the arc whose End is at the kink (the
// "incoming" carrier), and the arc whose Start is at the kink (the
// "outgoing" carrier). All indices refer to the input Diagram before
// the rewrite.
type r1Hit struct {
	crossing int
	loop     int
	inArc    int
	outArc   int
}

// detectR1 returns an r1Hit if the lasso encloses exactly one crossing
// and exactly one fully-inside arc that is a self-loop at that
// crossing, with the carrier strand passing through (i.e. the other
// two darts at the crossing belong to two distinct outside arcs whose
// over/under flags match — they're a continuous strand).
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
	// The kink loop crosses itself at v: its two endpoints there must
	// have opposite over flags. (If they were equal the configuration
	// is degenerate.)
	if d.Arcs[loop].Start.Over == d.Arcs[loop].End.Over {
		return zero, false
	}
	// Find the carrier "incoming" and "outgoing" arcs at v, i.e. the
	// arcs that have an endpoint at v that is NOT the loop.
	inArc, outArc := -1, -1
	for i, a := range d.Arcs {
		if i == loop {
			continue
		}
		if a.End.Crossing == v {
			if inArc != -1 {
				return zero, false
			}
			inArc = i
		}
		if a.Start.Crossing == v {
			if outArc != -1 {
				return zero, false
			}
			outArc = i
		}
	}
	if inArc < 0 || outArc < 0 {
		return zero, false
	}
	// Carrier is a continuous strand through v: same over/under at v.
	if d.Arcs[inArc].End.Over != d.Arcs[outArc].Start.Over {
		return zero, false
	}
	// Self-loop carrier (inArc == outArc) is the degenerate "unknot
	// with two kinks" — bail; the user can split that case manually.
	if inArc == outArc {
		return zero, false
	}
	return r1Hit{crossing: v, loop: loop, inArc: inArc, outArc: outArc}, true
}

// applyR1 performs the kink-removal rewrite: drops the crossing and
// the self-loop arc, splices the two carrier arcs into one continuous
// arc that bypasses where the crossing used to be. d is mutated in
// place; arc indices and crossing indices in the resulting Diagram
// are renumbered so they remain contiguous.
func applyR1(d *Diagram, r r1Hit) {
	in := d.Arcs[r.inArc]
	out := d.Arcs[r.outArc]
	// New polyline: in.Polyline, dropping the duplicated crossing
	// point shared with out.Polyline[0], then concatenated with
	// out.Polyline.
	poly := make([]image.Point, 0, len(in.Polyline)+len(out.Polyline)-1)
	poly = append(poly, in.Polyline...)
	if len(poly) > 0 && len(out.Polyline) > 0 && poly[len(poly)-1] == out.Polyline[0] {
		poly = poly[:len(poly)-1]
	}
	poly = append(poly, out.Polyline...)
	merged := Arc{Polyline: poly, Start: in.Start, End: out.End}

	// Remove the loop arc, the in arc, and the out arc; append the merged.
	drop := map[int]bool{r.loop: true, r.inArc: true, r.outArc: true}
	newArcs := make([]Arc, 0, len(d.Arcs)-3+1)
	for i, a := range d.Arcs {
		if drop[i] {
			continue
		}
		newArcs = append(newArcs, a)
	}
	newArcs = append(newArcs, merged)
	d.Arcs = newArcs

	// Drop crossing v.
	d.Crossings = append(d.Crossings[:r.crossing], d.Crossings[r.crossing+1:]...)
	// Renumber every Crossing reference in remaining arcs.
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
	// R2 alternation: each arc flips over/under between v and w.
	if overAvAtV == overAvAtW || overBvAtV == overBvAtW {
		return zero, false
	}
	// And at each crossing, the two interior arcs must lie on
	// opposite strands (one over, one under).
	if overAvAtV == overBvAtV || overAvAtW == overBvAtW {
		return zero, false
	}

	// Find exterior carrier arcs at v and w. At each crossing there
	// should be exactly one over-strand exterior arc and one under-
	// strand exterior arc — anything else is a degenerate setup.
	vOver, vUnd, wOver, wUnd := -1, -1, -1, -1
	for i, a := range d.Arcs {
		if i == arcA || i == arcB {
			continue
		}
		if arcEndpointAtCrossing(a, v) >= 0 {
			over := arcOverAtCrossing(a, v)
			if over {
				if vOver != -1 {
					return zero, false
				}
				vOver = i
			} else {
				if vUnd != -1 {
					return zero, false
				}
				vUnd = i
			}
		}
		// An arc may touch BOTH v and w (it could be one of the
		// carriers spanning the gap). The Start/End check at w must
		// run independently of the v branch.
		if arcEndpointAtCrossing(a, w) >= 0 {
			over := arcOverAtCrossing(a, w)
			if over {
				if wOver != -1 {
					return zero, false
				}
				wOver = i
			} else {
				if wUnd != -1 {
					return zero, false
				}
				wUnd = i
			}
		}
	}
	if vOver < 0 || vUnd < 0 || wOver < 0 || wUnd < 0 {
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
// The pairing is determined by the R2 alternation: the strand that
// is over at v is under at w (and vice versa), so we splice
// (vOver ↔ wUnd) and (vUnd ↔ wOver).
func applyR2(d *Diagram, r r2Hit) {
	merged1 := spliceArcsThroughCrossings(d, r.vOverArc, r.v, r.wUndArc, r.w)
	merged2 := spliceArcsThroughCrossings(d, r.vUndArc, r.v, r.wOverArc, r.w)

	drop := map[int]bool{
		r.arcA: true, r.arcB: true,
		r.vOverArc: true, r.vUndArc: true,
		r.wOverArc: true, r.wUndArc: true,
	}
	newArcs := make([]Arc, 0, len(d.Arcs)-len(drop)+2)
	for i, a := range d.Arcs {
		if drop[i] {
			continue
		}
		newArcs = append(newArcs, a)
	}
	newArcs = append(newArcs, merged1, merged2)
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
