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

	stroke := color.NRGBA{0x10, 0x10, 0x10, 0xff}
	const strokeW = float32(3.0)
	const gapPx = 8.0
	const subdiv = 6

	for _, a := range d.Arcs {
		smooth := smoothCatmullRom(a.Polyline, subdiv)
		poly := trimPolylineEnds(smooth, !a.Start.Over, !a.End.Over, gapPx)
		strokeSmoothPolyline(canvas, poly, strokeW, stroke)
	}
}

// smoothCatmullRom returns a denser polyline that is the uniform
// Catmull-Rom spline through poly's points. Each input segment becomes
// `subdiv` straight micro-segments. The first and last input points are
// kept exactly; virtual control points outside the polyline are obtained
// by reflecting (2·p[0] − p[1] and 2·p[n-1] − p[n-2]) so the curve has
// natural-looking tangents at both ends.
func smoothCatmullRom(poly []image.Point, subdiv int) []image.Point {
	n := len(poly)
	if n < 2 || subdiv <= 1 {
		return poly
	}
	pt := func(i int) (float64, float64) {
		switch {
		case i < 0:
			return 2*float64(poly[0].X) - float64(poly[1].X),
				2*float64(poly[0].Y) - float64(poly[1].Y)
		case i >= n:
			return 2*float64(poly[n-1].X) - float64(poly[n-2].X),
				2*float64(poly[n-1].Y) - float64(poly[n-2].Y)
		default:
			return float64(poly[i].X), float64(poly[i].Y)
		}
	}

	out := make([]image.Point, 0, (n-1)*subdiv+1)
	for i := 0; i < n-1; i++ {
		p0x, p0y := pt(i - 1)
		p1x, p1y := pt(i)
		p2x, p2y := pt(i + 1)
		p3x, p3y := pt(i + 2)
		m1x, m1y := (p2x-p0x)/2, (p2y-p0y)/2
		m2x, m2y := (p3x-p1x)/2, (p3y-p1y)/2
		if i == 0 {
			out = append(out, image.Point{
				X: int(math.Round(p1x)),
				Y: int(math.Round(p1y)),
			})
		}
		for s := 1; s <= subdiv; s++ {
			t := float64(s) / float64(subdiv)
			t2 := t * t
			t3 := t2 * t
			h00 := 2*t3 - 3*t2 + 1
			h10 := t3 - 2*t2 + t
			h01 := -2*t3 + 3*t2
			h11 := t3 - t2
			x := h00*p1x + h10*m1x + h01*p2x + h11*m2x
			y := h00*p1y + h10*m1y + h01*p2y + h11*m2y
			out = append(out, image.Point{
				X: int(math.Round(x)),
				Y: int(math.Round(y)),
			})
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
