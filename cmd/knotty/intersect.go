package main

import "image"

// Geometric intersection helpers used by the drag guard to detect when
// a drag would introduce or remove a crossing. A "crossing" in this
// sense is a strict-interior segment-segment intersection between two
// arc polyline segments — at every registered Diagram crossing the
// involved polylines TERMINATE rather than passing through, so a
// well-formed diagram has zero strict-interior intersections among its
// arc segments. Any non-zero count indicates a geometric crossing the
// diagram's combinatorial data isn't tracking.

// properSegmentIntersection reports whether the open segments AB and CD
// cross strictly in the interior of both. Shared endpoints (and touches
// within eps of an endpoint) don't count, so segments that legitimately
// meet at a Crossings vertex aren't flagged.
func properSegmentIntersection(a, b, c, d image.Point, eps float64) bool {
	ax, ay := float64(a.X), float64(a.Y)
	bx, by := float64(b.X), float64(b.Y)
	cx, cy := float64(c.X), float64(c.Y)
	dx, dy := float64(d.X), float64(d.Y)
	rx, ry := bx-ax, by-ay
	sx, sy := dx-cx, dy-cy
	denom := rx*sy - ry*sx
	if denom == 0 {
		// Parallel or collinear: treat as non-crossing. A drag step
		// that lands exactly collinear is vanishingly rare and the
		// next sub-pixel motion will resolve it.
		return false
	}
	qpx, qpy := cx-ax, cy-ay
	u := (qpx*sy - qpy*sx) / denom
	v := (qpx*ry - qpy*rx) / denom
	return u > eps && u < 1-eps && v > eps && v < 1-eps
}

// countDiagramCrossings counts strict-interior segment-segment
// intersections across all arcs of d (including each arc with itself).
// In a well-formed diagram this is 0; the drag guard uses changes in
// this count to detect a frame that would create a new crossing.
//
// eps is the parametric margin used to ignore shared endpoints. 1e-6 is
// safe — segments that share a vertex have u or v at exactly 0 or 1.
func countDiagramCrossings(d *Diagram) int {
	const eps = 1e-6
	if d == nil {
		return 0
	}
	count := 0
	for i := 0; i < len(d.Arcs); i++ {
		pi := d.Arcs[i].Polyline
		for si := 0; si+1 < len(pi); si++ {
			a, b := pi[si], pi[si+1]
			// Self-intersections within the same arc: skip adjacent
			// segments (they share a vertex by construction).
			for sj := si + 2; sj+1 < len(pi); sj++ {
				if properSegmentIntersection(a, b, pi[sj], pi[sj+1], eps) {
					count++
				}
			}
			// Pairs across distinct arcs: iterate j > i to avoid
			// double-counting.
			for j := i + 1; j < len(d.Arcs); j++ {
				pj := d.Arcs[j].Polyline
				for sj := 0; sj+1 < len(pj); sj++ {
					if properSegmentIntersection(a, b, pj[sj], pj[sj+1], eps) {
						count++
					}
				}
			}
		}
	}
	return count
}
