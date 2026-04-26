package main

import (
	"image"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Drag interaction:
//
// When a Diagram is attached to scaledImage, the user may grab and drag
// individual crossings or interior arc points. The interaction is
// "press-and-hold to grab" so casual pencil/eraser strokes are not
// hijacked: pressing the left mouse button while the cursor sits within
// grab range of a crossing or arc starts a hold timer; if the button is
// still held grabHoldDuration later AND the cursor has not moved more
// than grabSlackPx, the closer of {crossing, arc} is grabbed and the
// drag begins.
//
// Once grabbed, dragging a crossing translates the crossing point itself
// and smoothly deforms each incident arc so its endpoint at this
// crossing follows the cursor while the opposite endpoint stays fixed.
// Dragging an arc moves the grabbed interior point and applies a
// hat-shaped falloff along the arc so both endpoints remain pinned at
// their crossings. Neither operation creates or removes crossings.
//
// All distances below are in source-image (canvas) pixels.
const (
	grabHoldDuration = time.Second
	grabRangeCross   = 14.0
	grabRangeArc     = 10.0
	grabSlackPx      = 4.0
)

// dragKind enumerates what the cursor is currently interacting with.
type dragKind int

const (
	dragNone dragKind = iota
	dragCrossing
	dragArc
)

// dragState owns the press/hold/drag state machine. It does not own the
// Diagram or the canvas; callers pass those in each frame.
type dragState struct {
	// Which target was grabbed and what its index is.
	kind  dragKind
	index int

	// For arc drags: the [0,1] parameter of the grabbed point along the
	// (smoothed) arc length, captured at grab time.
	arcParam float64

	// pressing tracks whether the left button has been continuously held
	// since pressDownAt, with the cursor staying within grabSlackPx of
	// pressOrigin. holdFired is set once the hold timer elapses and the
	// grab is committed.
	pressing    bool
	pressOrigin imagePointF
	pressDownAt time.Time
	holdFired   bool

	// dragging is true between holdFired and button release.
	dragging bool

	// Hover state (for visual feedback when not yet pressed).
	hoverKind  dragKind
	hoverIndex int
}

type imagePointF struct{ X, Y float64 }

// reset clears all state — called on button release or when the diagram
// is cleared out from under us.
func (s *dragState) reset() {
	*s = dragState{}
}

// update advances the state machine for one frame.
//
// d is the current diagram (may be nil); cursor is the cursor position
// in source-image pixel coordinates; inBounds reports whether the
// cursor lies within the canvas bounds at all. pressed is the raw
// left-mouse-button state for this frame.
//
// Returns true when the diagram polylines were mutated this frame, so
// the caller knows to re-render the canvas.
func (s *dragState) update(d *Diagram, cursor imagePointF, inBounds, pressed bool) bool {
	if d == nil {
		s.reset()
		return false
	}

	// Drop pressing state when the button is released.
	if !pressed {
		s.pressing = false
		s.holdFired = false
		s.dragging = false
		s.kind = dragNone
		s.refreshHover(d, cursor, inBounds)
		return false
	}

	// While dragging, push the cursor delta into the diagram.
	if s.dragging {
		mutated := s.applyDrag(d, cursor)
		// Hover stays locked to the dragged target.
		s.hoverKind = s.kind
		s.hoverIndex = s.index
		return mutated
	}

	// Pressed but not yet dragging: arm the hold-timer on the first
	// press while pointing at something grabbable, then promote to a
	// drag once the timer elapses (and the cursor has stayed within
	// grabSlackPx of the press origin).
	if !s.pressing {
		s.refreshHover(d, cursor, inBounds)
		if !inBounds || s.hoverKind == dragNone {
			return false
		}
		s.pressing = true
		s.pressOrigin = cursor
		s.pressDownAt = time.Now()
		s.holdFired = false
		s.kind = s.hoverKind
		s.index = s.hoverIndex
		// arcParam is computed at grab-fire time so a small drift before
		// the timer fires doesn't move the grab point.
		return false
	}

	// Drift cancels the hold (so pencil strokes still work — the user
	// has visibly moved the cursor while pressing).
	dx := cursor.X - s.pressOrigin.X
	dy := cursor.Y - s.pressOrigin.Y
	if dx*dx+dy*dy > grabSlackPx*grabSlackPx {
		s.pressing = false
		s.holdFired = false
		s.kind = dragNone
		return false
	}

	if !s.holdFired && time.Since(s.pressDownAt) >= grabHoldDuration {
		s.holdFired = true
		s.beginDrag(d, cursor)
	}
	return false
}

// refreshHover updates hoverKind/hoverIndex so renderers can show a
// "this is grabbable" indicator. Crossings beat arcs at equal distance
// because they're the smaller target and easier to mis-grab as an arc.
func (s *dragState) refreshHover(d *Diagram, cursor imagePointF, inBounds bool) {
	if !inBounds {
		s.hoverKind = dragNone
		return
	}
	bestKind := dragNone
	bestIdx := -1
	bestD2 := math.Inf(1)

	rc2 := grabRangeCross * grabRangeCross
	for i, c := range d.Crossings {
		dx := cursor.X - float64(c.X)
		dy := cursor.Y - float64(c.Y)
		d2 := dx*dx + dy*dy
		if d2 <= rc2 && d2 < bestD2 {
			bestD2 = d2
			bestKind = dragCrossing
			bestIdx = i
		}
	}
	if bestKind == dragNone {
		ra2 := grabRangeArc * grabRangeArc
		for i, a := range d.Arcs {
			d2, _ := nearestOnPolyline(a.Polyline, cursor)
			if d2 <= ra2 && d2 < bestD2 {
				bestD2 = d2
				bestKind = dragArc
				bestIdx = i
			}
		}
	}
	s.hoverKind = bestKind
	s.hoverIndex = bestIdx
}

// beginDrag captures any per-target state needed to drive the drag.
func (s *dragState) beginDrag(d *Diagram, cursor imagePointF) {
	s.dragging = true
	if s.kind == dragArc && s.index >= 0 && s.index < len(d.Arcs) {
		_, t := nearestOnPolyline(d.Arcs[s.index].Polyline, cursor)
		// Clamp away from the endpoints so the arc actually deforms when
		// dragged (a t of exactly 0 or 1 has zero hat-weight).
		if t < 0.05 {
			t = 0.05
		}
		if t > 0.95 {
			t = 0.95
		}
		s.arcParam = t
	}
}

// applyDrag mutates d to reflect a cursor-driven drag for one frame.
// Returns true if anything changed.
//
// A drag step is committed only if it does not increase the number of
// strict-interior segment-segment intersections in the diagram —
// i.e. it does not introduce a new geometric crossing. Steps that
// would create a new crossing are silently rejected: the polylines
// (and crossing position, for crossing-drags) are restored to their
// pre-step values. This enforces the "dragging should not create new
// crossing points or remove existing crossing points" rule by making
// the bad sub-step a no-op; the user's cursor keeps moving and a
// later sub-step in a different direction will succeed.
func (s *dragState) applyDrag(d *Diagram, cursor imagePointF) bool {
	snap := snapshotForDrag(d, s)
	beforeCount := countDiagramCrossings(d)
	mutated := s.applyDragRaw(d, cursor)
	if !mutated {
		return false
	}
	if countDiagramCrossings(d) > beforeCount {
		restoreSnapshot(d, snap)
		return false
	}
	return true
}

// applyDragRaw is applyDrag without the snap-back guard. Tests use it
// to verify the un-guarded math directly; applyDrag wraps it.
func (s *dragState) applyDragRaw(d *Diagram, cursor imagePointF) bool {
	switch s.kind {
	case dragCrossing:
		if s.index < 0 || s.index >= len(d.Crossings) {
			return false
		}
		old := d.Crossings[s.index]
		newX, newY := int(math.Round(cursor.X)), int(math.Round(cursor.Y))
		if newX == old.X && newY == old.Y {
			return false
		}
		dx := cursor.X - float64(old.X)
		dy := cursor.Y - float64(old.Y)
		d.Crossings[s.index] = image.Point{X: newX, Y: newY}
		// Deform every arc that touches this crossing: translate its
		// near endpoint by (dx,dy) with a smooth falloff to 0 at the
		// far endpoint.
		for i := range d.Arcs {
			a := &d.Arcs[i]
			if a.Start.Crossing == s.index {
				translateArcEnd(a.Polyline, dx, dy, true)
				if len(a.Polyline) > 0 {
					a.Polyline[0] = image.Point{X: newX, Y: newY}
				}
			}
			if a.End.Crossing == s.index {
				translateArcEnd(a.Polyline, dx, dy, false)
				if n := len(a.Polyline); n > 0 {
					a.Polyline[n-1] = image.Point{X: newX, Y: newY}
				}
			}
		}
		return true

	case dragArc:
		if s.index < 0 || s.index >= len(d.Arcs) {
			return false
		}
		a := &d.Arcs[s.index]
		poly := a.Polyline
		if len(poly) < 2 {
			return false
		}
		// Where the grabbed point sits NOW given the current polyline.
		gx, gy := pointAtParam(poly, s.arcParam)
		dx := cursor.X - gx
		dy := cursor.Y - gy
		if dx == 0 && dy == 0 {
			return false
		}
		// Skip the two endpoints — they stay pinned to the crossings.
		moved := false
		n := len(poly)
		for i := 1; i < n-1; i++ {
			t := float64(i) / float64(n-1)
			w := hatWeight(t, s.arcParam)
			if w == 0 {
				continue
			}
			nx := float64(poly[i].X) + dx*w
			ny := float64(poly[i].Y) + dy*w
			np := image.Point{X: int(math.Round(nx)), Y: int(math.Round(ny))}
			if np != poly[i] {
				poly[i] = np
				moved = true
			}
		}
		return moved
	}
	return false
}

// dragSnapshot remembers exactly what a single applyDragRaw call may
// touch, so a guard that detects a bad step can restore the prior
// state. The set of fields captured depends on the drag kind:
//
//   - dragCrossing: the moving crossing's position plus the polylines
//     of every arc incident to it (because crossing-drag rewrites
//     those endpoints with falloff).
//   - dragArc: just the polyline of the grabbed arc.
type dragSnapshot struct {
	kind         dragKind
	crossingIdx  int
	crossingPt   image.Point
	arcSnapshots []arcPolylineSnapshot
}

type arcPolylineSnapshot struct {
	idx  int
	poly []image.Point
}

func snapshotForDrag(d *Diagram, s *dragState) dragSnapshot {
	snap := dragSnapshot{kind: s.kind}
	if d == nil {
		return snap
	}
	switch s.kind {
	case dragCrossing:
		if s.index < 0 || s.index >= len(d.Crossings) {
			return snap
		}
		snap.crossingIdx = s.index
		snap.crossingPt = d.Crossings[s.index]
		for i := range d.Arcs {
			a := &d.Arcs[i]
			if a.Start.Crossing == s.index || a.End.Crossing == s.index {
				snap.arcSnapshots = append(snap.arcSnapshots, arcPolylineSnapshot{
					idx:  i,
					poly: append([]image.Point(nil), a.Polyline...),
				})
			}
		}
	case dragArc:
		if s.index < 0 || s.index >= len(d.Arcs) {
			return snap
		}
		snap.arcSnapshots = append(snap.arcSnapshots, arcPolylineSnapshot{
			idx:  s.index,
			poly: append([]image.Point(nil), d.Arcs[s.index].Polyline...),
		})
	}
	return snap
}

func restoreSnapshot(d *Diagram, snap dragSnapshot) {
	if d == nil {
		return
	}
	if snap.kind == dragCrossing && snap.crossingIdx >= 0 && snap.crossingIdx < len(d.Crossings) {
		d.Crossings[snap.crossingIdx] = snap.crossingPt
	}
	for _, as := range snap.arcSnapshots {
		if as.idx < 0 || as.idx >= len(d.Arcs) {
			continue
		}
		// Restore in place so the slice header callers may hold onto
		// remains valid. The polyline may have been the same length
		// throughout (drag never resizes it), so a copy suffices.
		dst := d.Arcs[as.idx].Polyline
		if len(dst) != len(as.poly) {
			d.Arcs[as.idx].Polyline = append(dst[:0], as.poly...)
		} else {
			copy(dst, as.poly)
		}
	}
}

// translateArcEnd shifts the points of poly by (dx,dy) with a triangular
// falloff: full at the chosen end, zero at the other end. The end point
// itself is left to be overwritten with the exact new crossing
// coordinate by the caller.
func translateArcEnd(poly []image.Point, dx, dy float64, atStart bool) {
	n := len(poly)
	if n < 2 {
		return
	}
	for i := 1; i < n-1; i++ {
		var w float64
		if atStart {
			w = 1.0 - float64(i)/float64(n-1)
		} else {
			w = float64(i) / float64(n-1)
		}
		// Smoothstep so the deformation tapers smoothly at both ends
		// rather than landing as a sharp triangle.
		w = w * w * (3 - 2*w)
		poly[i] = image.Point{
			X: int(math.Round(float64(poly[i].X) + dx*w)),
			Y: int(math.Round(float64(poly[i].Y) + dy*w)),
		}
	}
}

// hatWeight is a smooth 0→1→0 weight on [0,1] peaking at peak. It is
// the smoothstep of a triangular ramp on each side, so the resulting
// arc deformation has continuous tangents at both endpoints.
func hatWeight(t, peak float64) float64 {
	if t <= 0 || t >= 1 {
		return 0
	}
	var u float64
	if t <= peak {
		if peak <= 0 {
			return 0
		}
		u = t / peak
	} else {
		if peak >= 1 {
			return 0
		}
		u = (1 - t) / (1 - peak)
	}
	if u <= 0 {
		return 0
	}
	if u >= 1 {
		return 1
	}
	return u * u * (3 - 2*u)
}

// nearestOnPolyline returns (squared distance, parameter t in [0,1])
// from the closest point on poly to p. The parameter is normalized by
// total polyline length so it can drive arc-length-style operations
// regardless of segment count.
func nearestOnPolyline(poly []image.Point, p imagePointF) (float64, float64) {
	n := len(poly)
	if n == 0 {
		return math.Inf(1), 0
	}
	if n == 1 {
		dx := p.X - float64(poly[0].X)
		dy := p.Y - float64(poly[0].Y)
		return dx*dx + dy*dy, 0
	}
	segs := make([]float64, n-1)
	total := 0.0
	for i := range segs {
		dx := float64(poly[i+1].X - poly[i].X)
		dy := float64(poly[i+1].Y - poly[i].Y)
		segs[i] = math.Hypot(dx, dy)
		total += segs[i]
	}
	bestD2 := math.Inf(1)
	bestT := 0.0
	acc := 0.0
	for i, s := range segs {
		ax := float64(poly[i].X)
		ay := float64(poly[i].Y)
		bx := float64(poly[i+1].X)
		by := float64(poly[i+1].Y)
		dx := bx - ax
		dy := by - ay
		segLen2 := dx*dx + dy*dy
		var u float64
		if segLen2 > 0 {
			u = ((p.X-ax)*dx + (p.Y-ay)*dy) / segLen2
		}
		if u < 0 {
			u = 0
		} else if u > 1 {
			u = 1
		}
		cx := ax + dx*u
		cy := ay + dy*u
		ddx := p.X - cx
		ddy := p.Y - cy
		d2 := ddx*ddx + ddy*ddy
		if d2 < bestD2 {
			bestD2 = d2
			if total > 0 {
				bestT = (acc + u*s) / total
			}
		}
		acc += s
	}
	return bestD2, bestT
}

// pointAtParam returns the (x,y) at arc-length parameter t∈[0,1] along
// poly, treating consecutive vertices as straight segments.
func pointAtParam(poly []image.Point, t float64) (float64, float64) {
	n := len(poly)
	if n == 0 {
		return 0, 0
	}
	if n == 1 || t <= 0 {
		return float64(poly[0].X), float64(poly[0].Y)
	}
	if t >= 1 {
		return float64(poly[n-1].X), float64(poly[n-1].Y)
	}
	segs := make([]float64, n-1)
	total := 0.0
	for i := range segs {
		dx := float64(poly[i+1].X - poly[i].X)
		dy := float64(poly[i+1].Y - poly[i].Y)
		segs[i] = math.Hypot(dx, dy)
		total += segs[i]
	}
	target := t * total
	acc := 0.0
	for i, s := range segs {
		if acc+s >= target {
			u := 0.0
			if s > 0 {
				u = (target - acc) / s
			}
			ax := float64(poly[i].X)
			ay := float64(poly[i].Y)
			bx := float64(poly[i+1].X)
			by := float64(poly[i+1].Y)
			return ax + (bx-ax)*u, ay + (by-ay)*u
		}
		acc += s
	}
	return float64(poly[n-1].X), float64(poly[n-1].Y)
}

// drawDragOverlay paints hover/grab feedback over the rendered canvas.
// holdProgress ∈ [0,1] is how far through the press-and-hold timer the
// current press has gone (0 when not pressing); it animates the hover
// ring so the user can see they need to keep holding.
//
// All inputs are in screen coordinates already scaled.
func drawDragOverlay(screen *ebiten.Image, ox, oy, scale float64, d *Diagram, st *dragState) {
	if d == nil || st.hoverKind == dragNone {
		return
	}

	now := time.Now()
	holdProgress := 0.0
	if st.pressing && !st.holdFired {
		elapsed := now.Sub(st.pressDownAt).Seconds()
		holdProgress = elapsed / grabHoldDuration.Seconds()
		if holdProgress > 1 {
			holdProgress = 1
		}
	} else if st.dragging {
		holdProgress = 1
	}

	hoverColor := color.NRGBA{0x40, 0xc0, 0x40, 0xff}
	grabColor := color.NRGBA{0xff, 0xc0, 0x20, 0xff}
	c := hoverColor
	if st.dragging {
		c = grabColor
	}

	switch st.hoverKind {
	case dragCrossing:
		if st.hoverIndex < 0 || st.hoverIndex >= len(d.Crossings) {
			return
		}
		p := d.Crossings[st.hoverIndex]
		cx := float32(ox) + float32(p.X)*float32(scale)
		cy := float32(oy) + float32(p.Y)*float32(scale)
		baseR := float32(grabRangeCross) * float32(scale)
		vector.StrokeCircle(screen, cx, cy, baseR, 2, c, true)
		if holdProgress > 0 && !st.dragging {
			// A second concentric ring shrinks toward the center as the
			// hold progresses — visual cue that the timer is filling.
			ringR := baseR * float32(1.0-0.6*holdProgress)
			vector.StrokeCircle(screen, cx, cy, ringR, 2, grabColor, true)
		}
	case dragArc:
		if st.hoverIndex < 0 || st.hoverIndex >= len(d.Arcs) {
			return
		}
		a := d.Arcs[st.hoverIndex]
		// Re-stroke the hovered arc in the hover color so it stands out.
		strokePolylineOverlay(screen, a.Polyline, ox, oy, scale, 3, c)
		// Plus a small handle marker at the grab point (or the nearest
		// point if not yet grabbed).
		var t float64
		if st.dragging {
			t = st.arcParam
		} else {
			// Same pointer source as the input layer so touch-press
			// shows its hover handle at the finger position, not the
			// stale mouse cursor.
			pmx, pmy, _ := primaryPointer()
			cx := (pmx - ox) / scale
			cy := (pmy - oy) / scale
			_, t = nearestOnPolyline(a.Polyline, imagePointF{X: cx, Y: cy})
		}
		hx, hy := pointAtParam(a.Polyline, t)
		sx := float32(ox) + float32(hx)*float32(scale)
		sy := float32(oy) + float32(hy)*float32(scale)
		baseR := float32(grabRangeArc) * float32(scale)
		vector.StrokeCircle(screen, sx, sy, baseR, 2, c, true)
		if holdProgress > 0 && !st.dragging {
			ringR := baseR * float32(1.0-0.6*holdProgress)
			vector.StrokeCircle(screen, sx, sy, ringR, 2, grabColor, true)
		}
	}
}

// strokePolylineOverlay strokes poly (in source-image coordinates)
// directly onto screen, applying the current scale/offset. Used to
// highlight a hovered arc on top of the rendered canvas.
func strokePolylineOverlay(screen *ebiten.Image, poly []image.Point, ox, oy, scale float64, w float32, c color.Color) {
	if len(poly) < 2 {
		return
	}
	for i := 0; i+1 < len(poly); i++ {
		ax := float32(ox) + float32(poly[i].X)*float32(scale)
		ay := float32(oy) + float32(poly[i].Y)*float32(scale)
		bx := float32(ox) + float32(poly[i+1].X)*float32(scale)
		by := float32(oy) + float32(poly[i+1].Y)*float32(scale)
		vector.StrokeLine(screen, ax, ay, bx, by, w, c, true)
	}
}
