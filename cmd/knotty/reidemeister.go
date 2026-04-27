package main

import (
	"fmt"
	"image"
)

// doReidemeister consumes a closed lasso polygon (in source-image pixel
// coordinates, last point == first point) and tries to perform a legal
// Reidemeister move on the attached Diagram.
//
// Phase 1 stub: the lasso UI and overlay are wired up; this method
// receives the closed polygon and reports what's inside it but does
// not yet rewrite the Diagram. Phases 2-4 fill in the R1/R2/R3
// simplification and creation branches.
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
		// Need ≥ 3 distinct points + closing duplicate.
		return
	}
	insideCrossings := 0
	for _, c := range d.Crossings {
		if closedPolygonContainsPoint(closed, c) {
			insideCrossings++
		}
	}
	insideArcsFully := 0
	insideArcsCrossing := 0
	for _, a := range d.Arcs {
		any, all := arcInLassoStats(closed, a.Polyline)
		if all {
			insideArcsFully++
		} else if any {
			insideArcsCrossing++
		}
	}
	g.propsArea.SetText(fmt.Sprintf(
		"reidemeister: lasso %d pts, encloses %d crossings, %d full arcs, %d boundary-crossing arcs\n",
		len(closed)-1, insideCrossings, insideArcsFully, insideArcsCrossing))
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
