package main

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// renderDiagram clears canvas to bg and draws the Diagram's arcs as
// strokes. Under-strand arcs are trimmed at every "under" endpoint so a
// visible gap appears at each crossing where the strand passes
// underneath; over-strand arcs are drawn full-length and so meet
// continuously at every over-crossing.
func renderDiagram(canvas *ebiten.Image, d *Diagram, bg color.Color) {
	if canvas == nil || d == nil {
		return
	}
	canvas.Fill(bg)

	defaultStroke := color.NRGBA{0x10, 0x10, 0x10, 0xff}
	const strokeW = float32(3.0)
	const gapPx = 8.0
	const chaikinIters = 3

	for _, a := range d.Arcs {
		smooth := smoothChaikin(a.Polyline, chaikinIters)
		poly := trimPolylineEnds(smooth, !a.Start.Over, !a.End.Over, gapPx)
		stroke := color.Color(defaultStroke)
		if a.Color.A != 0 {
			stroke = a.Color
		}
		strokeSmoothPolyline(canvas, poly, strokeW, stroke)
	}
	// Free-floating loops (no crossings, no over/under). Smooth as
	// closed curves: append the first point at the end so Chaikin
	// rounds the seam, then drop it for stroking.
	for _, lp := range d.Loops {
		if len(lp) < 2 {
			continue
		}
		closed := append(append([]image.Point(nil), lp...), lp[0])
		smooth := smoothChaikin(closed, chaikinIters)
		strokeSmoothPolyline(canvas, smooth, strokeW, defaultStroke)
	}
}

// resamplePolylineUniform returns a polyline with exactly n points sampled
// at uniform arc-length intervals along poly. The first and last points
// are preserved exactly; interior points are linearly interpolated within
// the source segment that contains the corresponding arc-length target.
//
// Used to normalize freshly-converted polylines (which can have hundreds
// of one-pixel-spaced points) down to a small smooth control polygon
// that the renderer can interpolate cleanly and the drag math can move
// without amplifying pixel-grid jitter.
//
// Returns poly unchanged if it already has the requested point count, has
// fewer than 2 points, or has zero total length.
func resamplePolylineUniform(poly []image.Point, n int) []image.Point {
	if n < 2 || len(poly) < 2 || len(poly) == n {
		return poly
	}
	cum := make([]float64, len(poly))
	for i := 1; i < len(poly); i++ {
		dx := float64(poly[i].X - poly[i-1].X)
		dy := float64(poly[i].Y - poly[i-1].Y)
		cum[i] = cum[i-1] + math.Hypot(dx, dy)
	}
	total := cum[len(poly)-1]
	if total == 0 {
		return poly
	}
	out := make([]image.Point, n)
	out[0] = poly[0]
	out[n-1] = poly[len(poly)-1]
	j := 1
	for k := 1; k < n-1; k++ {
		target := total * float64(k) / float64(n-1)
		for j < len(poly)-1 && cum[j] < target {
			j++
		}
		segLen := cum[j] - cum[j-1]
		t := 0.0
		if segLen > 0 {
			t = (target - cum[j-1]) / segLen
		}
		x := float64(poly[j-1].X) + t*float64(poly[j].X-poly[j-1].X)
		y := float64(poly[j-1].Y) + t*float64(poly[j].Y-poly[j-1].Y)
		out[k] = image.Point{X: int(math.Round(x)), Y: int(math.Round(y))}
	}
	return out
}

// smoothChaikin returns the polyline produced by k iterations of
// Chaikin's corner-cutting algorithm: each interior segment is replaced
// with two new points, one a quarter and one three-quarters of the way
// along it. The first and last points are preserved exactly; interior
// control points become "directors" that the curve approaches but does
// not pass through. The limit (k → ∞) is a quadratic uniform B-spline.
//
// Crucially, Chaikin is non-interpolating: pixel-grid jitter on
// interior control points is averaged away rather than amplified into
// visible wiggles, which Catmull-Rom does. k=3 is plenty for visual
// smoothness and turns an n-point input into a 2³·(n−1)+2 = 8n−6 point
// output (~100 points for our 13-point arcs).
func smoothChaikin(poly []image.Point, k int) []image.Point {
	if len(poly) < 2 || k <= 0 {
		return poly
	}
	type fp struct{ X, Y float64 }
	cur := make([]fp, len(poly))
	for i, p := range poly {
		cur[i] = fp{float64(p.X), float64(p.Y)}
	}
	for iter := 0; iter < k; iter++ {
		nxt := make([]fp, 0, 2*len(cur))
		nxt = append(nxt, cur[0])
		for i := 0; i < len(cur)-1; i++ {
			a, b := cur[i], cur[i+1]
			q := fp{X: 0.75*a.X + 0.25*b.X, Y: 0.75*a.Y + 0.25*b.Y}
			r := fp{X: 0.25*a.X + 0.75*b.X, Y: 0.25*a.Y + 0.75*b.Y}
			nxt = append(nxt, q, r)
		}
		nxt = append(nxt, cur[len(cur)-1])
		cur = nxt
	}
	out := make([]image.Point, len(cur))
	for i, p := range cur {
		out[i] = image.Point{
			X: int(math.Round(p.X)),
			Y: int(math.Round(p.Y)),
		}
	}
	return out
}

// strokeSmoothPolyline strokes consecutive segments of poly, then fills a
// disk of radius w/2 at each vertex. The disks round out the segment
// joins (StrokeLine uses butt caps so without them tight bends show a
// visible kink) and at the polyline ends provide round caps.
func strokeSmoothPolyline(canvas *ebiten.Image, poly []image.Point, w float32, c color.Color) {
	if len(poly) < 2 {
		return
	}
	r := w / 2
	for i := 0; i+1 < len(poly); i++ {
		p, q := poly[i], poly[i+1]
		vector.StrokeLine(canvas,
			float32(p.X), float32(p.Y),
			float32(q.X), float32(q.Y),
			w, c, true)
	}
	for _, p := range poly {
		vector.FillCircle(canvas, float32(p.X), float32(p.Y), r, c, true)
	}
}

// trimPolylineEnds returns a polyline that is poly with at most gapPx of
// path length removed from the start and/or end, in floating-point
// precision. The first/last point becomes a new interpolated point; the
// rest of the polyline is preserved. If a side is trimmed away entirely
// (gapPx exceeds total path length), the result has length < 2.
func trimPolylineEnds(poly []image.Point, trimStart, trimEnd bool, gapPx float64) []image.Point {
	if len(poly) < 2 || (!trimStart && !trimEnd) {
		return poly
	}
	type fp struct{ X, Y float64 }
	pts := make([]fp, len(poly))
	for i, p := range poly {
		pts[i] = fp{float64(p.X), float64(p.Y)}
	}

	if trimStart {
		remaining := gapPx
		for len(pts) >= 2 && remaining > 0 {
			dx := pts[1].X - pts[0].X
			dy := pts[1].Y - pts[0].Y
			d := math.Hypot(dx, dy)
			if d <= remaining {
				pts = pts[1:]
				remaining -= d
			} else {
				t := remaining / d
				pts[0] = fp{pts[0].X + dx*t, pts[0].Y + dy*t}
				remaining = 0
			}
		}
	}
	if trimEnd {
		remaining := gapPx
		for len(pts) >= 2 && remaining > 0 {
			n := len(pts)
			dx := pts[n-2].X - pts[n-1].X
			dy := pts[n-2].Y - pts[n-1].Y
			d := math.Hypot(dx, dy)
			if d <= remaining {
				pts = pts[:n-1]
				remaining -= d
			} else {
				t := remaining / d
				pts[n-1] = fp{pts[n-1].X + dx*t, pts[n-1].Y + dy*t}
				remaining = 0
			}
		}
	}

	out := make([]image.Point, len(pts))
	for i, p := range pts {
		out[i] = image.Point{X: int(math.Round(p.X)), Y: int(math.Round(p.Y))}
	}
	return out
}
