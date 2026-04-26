package main

import (
	"image"
	"testing"
)

// makeArc builds an Arc with a straight polyline of n+1 evenly-spaced
// points from a to b, attached to the given start/end crossings.
func makeArc(start, end int, a, b image.Point, n int) Arc {
	poly := make([]image.Point, n+1)
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		poly[i] = image.Point{
			X: int(float64(a.X) + (float64(b.X)-float64(a.X))*t + 0.5),
			Y: int(float64(a.Y) + (float64(b.Y)-float64(a.Y))*t + 0.5),
		}
	}
	return Arc{
		Polyline: poly,
		Start:    Endpoint{Crossing: start},
		End:      Endpoint{Crossing: end},
	}
}

// TestCrossingDragLeavesOtherCrossingsAlone verifies the invariant the
// user called out: dragging a crossing should not move any other
// crossing. We synthesize a 3-crossing diagram with three connecting
// arcs, drag crossing 1, and assert crossings 0 and 2 are untouched.
func TestCrossingDragLeavesOtherCrossingsAlone(t *testing.T) {
	d := &Diagram{
		Crossings: []image.Point{
			{X: 0, Y: 0},
			{X: 100, Y: 0},
			{X: 200, Y: 0},
		},
		Arcs: []Arc{
			makeArc(0, 1, image.Point{X: 0, Y: 0}, image.Point{X: 100, Y: 0}, 10),
			makeArc(1, 2, image.Point{X: 100, Y: 0}, image.Point{X: 200, Y: 0}, 10),
			makeArc(2, 0, image.Point{X: 200, Y: 0}, image.Point{X: 0, Y: 0}, 10),
		},
	}
	c0 := d.Crossings[0]
	c2 := d.Crossings[2]

	st := &dragState{kind: dragCrossing, index: 1, dragging: true}
	if !st.applyDrag(d, imagePointF{X: 120, Y: 40}) {
		t.Fatal("applyDrag returned false but should have moved crossing 1")
	}

	if d.Crossings[0] != c0 {
		t.Errorf("crossing 0 moved: got %v want %v", d.Crossings[0], c0)
	}
	if d.Crossings[2] != c2 {
		t.Errorf("crossing 2 moved: got %v want %v", d.Crossings[2], c2)
	}
	want := image.Point{X: 120, Y: 40}
	if d.Crossings[1] != want {
		t.Errorf("crossing 1: got %v want %v", d.Crossings[1], want)
	}
}

// TestCrossingDragKeepsArcEndpointsAtCrossings verifies the second
// invariant for crossing drags: every arc that touches the dragged
// crossing must have its corresponding endpoint snap exactly to the
// new crossing position, and arcs that don't touch it have both
// endpoints untouched. Without this, the "no new crossings created"
// rule could be quietly violated by drift.
func TestCrossingDragKeepsArcEndpointsAtCrossings(t *testing.T) {
	d := &Diagram{
		Crossings: []image.Point{
			{X: 0, Y: 0},
			{X: 100, Y: 0},
			{X: 200, Y: 0},
		},
		Arcs: []Arc{
			makeArc(0, 1, image.Point{X: 0, Y: 0}, image.Point{X: 100, Y: 0}, 10),
			makeArc(1, 2, image.Point{X: 100, Y: 0}, image.Point{X: 200, Y: 0}, 10),
			makeArc(2, 0, image.Point{X: 200, Y: 0}, image.Point{X: 0, Y: 0}, 10),
		},
	}

	st := &dragState{kind: dragCrossing, index: 1, dragging: true}
	st.applyDrag(d, imagePointF{X: 120, Y: 40})
	want := d.Crossings[1]

	// Arc 0 ends at crossing 1.
	a0 := d.Arcs[0].Polyline
	if a0[len(a0)-1] != want {
		t.Errorf("arc 0 end: got %v want %v", a0[len(a0)-1], want)
	}
	if a0[0] != d.Crossings[0] {
		t.Errorf("arc 0 start moved: got %v want %v", a0[0], d.Crossings[0])
	}

	// Arc 1 starts at crossing 1.
	a1 := d.Arcs[1].Polyline
	if a1[0] != want {
		t.Errorf("arc 1 start: got %v want %v", a1[0], want)
	}
	if a1[len(a1)-1] != d.Crossings[2] {
		t.Errorf("arc 1 end moved: got %v want %v", a1[len(a1)-1], d.Crossings[2])
	}

	// Arc 2 doesn't touch crossing 1 — both endpoints must be unchanged.
	a2 := d.Arcs[2].Polyline
	if a2[0] != d.Crossings[2] {
		t.Errorf("arc 2 start moved: got %v want %v", a2[0], d.Crossings[2])
	}
	if a2[len(a2)-1] != d.Crossings[0] {
		t.Errorf("arc 2 end moved: got %v want %v", a2[len(a2)-1], d.Crossings[0])
	}
}

// TestArcDragKeepsEndpointsFixed verifies the user's invariant for arc
// drags: dragging the interior of an arc must not move its endpoints
// (which sit at crossings). The test grabs near the midpoint, drags
// it perpendicular to the arc, and asserts both endpoints are
// pixel-identical before/after.
func TestArcDragKeepsEndpointsFixed(t *testing.T) {
	a := makeArc(0, 1, image.Point{X: 0, Y: 0}, image.Point{X: 100, Y: 0}, 20)
	d := &Diagram{
		Crossings: []image.Point{{X: 0, Y: 0}, {X: 100, Y: 0}},
		Arcs:      []Arc{a},
	}
	startBefore := d.Arcs[0].Polyline[0]
	endBefore := d.Arcs[0].Polyline[len(d.Arcs[0].Polyline)-1]

	st := &dragState{kind: dragArc, index: 0}
	st.beginDrag(d, imagePointF{X: 50, Y: 0})
	st.dragging = true
	for i := 0; i < 5; i++ {
		st.applyDrag(d, imagePointF{X: 50, Y: float64(10 * (i + 1))})
	}

	startAfter := d.Arcs[0].Polyline[0]
	endAfter := d.Arcs[0].Polyline[len(d.Arcs[0].Polyline)-1]

	if startAfter != startBefore {
		t.Errorf("arc start endpoint moved: got %v want %v", startAfter, startBefore)
	}
	if endAfter != endBefore {
		t.Errorf("arc end endpoint moved: got %v want %v", endAfter, endBefore)
	}

	// Sanity: at least one interior point must have moved away from the
	// original straight line.
	moved := false
	for i := 1; i < len(d.Arcs[0].Polyline)-1; i++ {
		if d.Arcs[0].Polyline[i].Y != 0 {
			moved = true
			break
		}
	}
	if !moved {
		t.Error("expected interior arc points to be deformed by drag")
	}
}

// TestArcDragNeverChangesArcCount and TestCrossingDragNeverChangesCrossingCount
// guard the "no new crossings created or removed" rule — the slice
// lengths must be invariant under any drag.
func TestArcDragNeverChangesArcCount(t *testing.T) {
	d := &Diagram{
		Crossings: []image.Point{{X: 0, Y: 0}, {X: 100, Y: 0}},
		Arcs:      []Arc{makeArc(0, 1, image.Point{X: 0, Y: 0}, image.Point{X: 100, Y: 0}, 12)},
	}
	beforeArcs := len(d.Arcs)
	beforeCross := len(d.Crossings)
	beforePoly := len(d.Arcs[0].Polyline)

	st := &dragState{kind: dragArc, index: 0, arcParam: 0.5, dragging: true}
	st.applyDrag(d, imagePointF{X: 50, Y: 30})

	if len(d.Arcs) != beforeArcs {
		t.Errorf("arc count changed: %d -> %d", beforeArcs, len(d.Arcs))
	}
	if len(d.Crossings) != beforeCross {
		t.Errorf("crossing count changed: %d -> %d", beforeCross, len(d.Crossings))
	}
	if len(d.Arcs[0].Polyline) != beforePoly {
		t.Errorf("polyline length changed: %d -> %d", beforePoly, len(d.Arcs[0].Polyline))
	}
}

func TestCrossingDragNeverChangesCrossingCount(t *testing.T) {
	d := &Diagram{
		Crossings: []image.Point{{X: 0, Y: 0}, {X: 100, Y: 0}},
		Arcs: []Arc{
			makeArc(0, 1, image.Point{X: 0, Y: 0}, image.Point{X: 100, Y: 0}, 8),
		},
	}
	st := &dragState{kind: dragCrossing, index: 0, dragging: true}
	st.applyDrag(d, imagePointF{X: -30, Y: 20})
	if len(d.Crossings) != 2 {
		t.Errorf("crossing count changed: got %d want 2", len(d.Crossings))
	}
	if len(d.Arcs) != 1 {
		t.Errorf("arc count changed: got %d want 1", len(d.Arcs))
	}
}

// TestArcDragGuardRejectsNewCrossings is the strong invariant test for
// the user-stated rule "dragging should not create new crossing points
// or remove existing crossing points". It builds a 2-arc diagram
// where both arcs share two endpoints (a Hopf-link-style geometry),
// then sweeps a drag of the lower arc upward through and past the
// upper arc. countDiagramCrossings must stay at 0 throughout — every
// frame whose raw motion would cross the other arc must be rolled
// back by the snap-back guard.
func TestArcDragGuardRejectsNewCrossings(t *testing.T) {
	d := &Diagram{
		Crossings: []image.Point{{X: 0, Y: 0}, {X: 100, Y: 0}},
		Arcs: []Arc{
			// Lower arc, bowed down to y = -50 at the midpoint.
			{
				Polyline: []image.Point{
					{X: 0, Y: 0}, {X: 25, Y: -30}, {X: 50, Y: -50},
					{X: 75, Y: -30}, {X: 100, Y: 0},
				},
				Start: Endpoint{Crossing: 0},
				End:   Endpoint{Crossing: 1},
			},
			// Upper arc, bowed up to y = +50 at the midpoint.
			{
				Polyline: []image.Point{
					{X: 0, Y: 0}, {X: 25, Y: 30}, {X: 50, Y: 50},
					{X: 75, Y: 30}, {X: 100, Y: 0},
				},
				Start: Endpoint{Crossing: 0},
				End:   Endpoint{Crossing: 1},
			},
		},
	}
	if got := countDiagramCrossings(d); got != 0 {
		t.Fatalf("baseline diagram: got %d crossings want 0", got)
	}

	startPolyA := append([]image.Point(nil), d.Arcs[0].Polyline...)
	startPolyB := append([]image.Point(nil), d.Arcs[1].Polyline...)
	endA := startPolyA[len(startPolyA)-1]
	startA := startPolyA[0]

	st := &dragState{kind: dragArc, index: 0}
	st.beginDrag(d, imagePointF{X: 50, Y: -50})
	st.dragging = true

	// Sweep the cursor straight up, well past the upper arc. Without
	// the guard, the lower arc would pass through the upper arc and
	// raise the crossing count to a positive value.
	for y := -50.0; y <= 150.0; y += 1.0 {
		st.applyDrag(d, imagePointF{X: 50, Y: y})
		if got := countDiagramCrossings(d); got != 0 {
			t.Fatalf("at cursor y=%v: countDiagramCrossings=%d want 0", y, got)
		}
	}

	// Endpoints of the dragged arc must still be pinned to the
	// crossings they belong to.
	if d.Arcs[0].Polyline[0] != startA {
		t.Errorf("arc 0 start moved: got %v want %v", d.Arcs[0].Polyline[0], startA)
	}
	last := len(d.Arcs[0].Polyline) - 1
	if d.Arcs[0].Polyline[last] != endA {
		t.Errorf("arc 0 end moved: got %v want %v", d.Arcs[0].Polyline[last], endA)
	}
	// The other arc must not have moved at all (only the grabbed
	// arc's interior is touched).
	for i, p := range d.Arcs[1].Polyline {
		if p != startPolyB[i] {
			t.Errorf("arc 1 point %d moved: got %v want %v", i, p, startPolyB[i])
		}
	}
}

// TestCrossingDragGuardRejectsNewCrossings exercises the guard for
// crossing drags. Two arcs run between two crossings; dragging the
// left crossing far to the right would smear the lower arc up
// through the upper arc. The guard must keep the spurious-crossing
// count at zero.
func TestCrossingDragGuardRejectsNewCrossings(t *testing.T) {
	d := &Diagram{
		Crossings: []image.Point{{X: 0, Y: 0}, {X: 100, Y: 0}},
		Arcs: []Arc{
			{
				Polyline: []image.Point{
					{X: 0, Y: 0}, {X: 25, Y: -30}, {X: 50, Y: -50},
					{X: 75, Y: -30}, {X: 100, Y: 0},
				},
				Start: Endpoint{Crossing: 0},
				End:   Endpoint{Crossing: 1},
			},
			{
				Polyline: []image.Point{
					{X: 0, Y: 0}, {X: 25, Y: 30}, {X: 50, Y: 50},
					{X: 75, Y: 30}, {X: 100, Y: 0},
				},
				Start: Endpoint{Crossing: 0},
				End:   Endpoint{Crossing: 1},
			},
		},
	}
	if got := countDiagramCrossings(d); got != 0 {
		t.Fatalf("baseline diagram: got %d crossings want 0", got)
	}

	st := &dragState{kind: dragCrossing, index: 0, dragging: true}
	for y := 0.0; y <= 120.0; y += 1.0 {
		st.applyDrag(d, imagePointF{X: 0, Y: y})
		if got := countDiagramCrossings(d); got != 0 {
			t.Fatalf("at crossing-cursor y=%v: countDiagramCrossings=%d want 0", y, got)
		}
	}
}

// TestArcDragRawWithoutGuardWouldCreateCrossings confirms the test
// scenario in TestArcDragGuardRejectsNewCrossings is not vacuous: the
// raw deformation (without the snap-back guard) DOES introduce
// spurious crossings on the same input. If this test ever stops
// observing any spurious crossings, the guard test above is no longer
// proving anything.
func TestArcDragRawWithoutGuardWouldCreateCrossings(t *testing.T) {
	d := &Diagram{
		Crossings: []image.Point{{X: 0, Y: 0}, {X: 100, Y: 0}},
		Arcs: []Arc{
			{
				Polyline: []image.Point{
					{X: 0, Y: 0}, {X: 25, Y: -30}, {X: 50, Y: -50},
					{X: 75, Y: -30}, {X: 100, Y: 0},
				},
				Start: Endpoint{Crossing: 0},
				End:   Endpoint{Crossing: 1},
			},
			{
				Polyline: []image.Point{
					{X: 0, Y: 0}, {X: 25, Y: 30}, {X: 50, Y: 50},
					{X: 75, Y: 30}, {X: 100, Y: 0},
				},
				Start: Endpoint{Crossing: 0},
				End:   Endpoint{Crossing: 1},
			},
		},
	}
	st := &dragState{kind: dragArc, index: 0}
	st.beginDrag(d, imagePointF{X: 50, Y: -50})
	st.dragging = true

	maxSeen := 0
	for y := -50.0; y <= 150.0; y += 1.0 {
		st.applyDragRaw(d, imagePointF{X: 50, Y: y})
		if c := countDiagramCrossings(d); c > maxSeen {
			maxSeen = c
		}
	}
	if maxSeen == 0 {
		t.Fatal("applyDragRaw never produced a spurious crossing — " +
			"TestArcDragGuardRejectsNewCrossings can't be proving anything")
	}
}

// TestProperSegmentIntersectionShared verifies the geometry helper
// rejects shared-endpoint touches (which the drag guard relies on,
// since polylines that meet at a registered crossing legitimately
// share an endpoint).
func TestProperSegmentIntersectionShared(t *testing.T) {
	a := image.Point{X: 0, Y: 0}
	b := image.Point{X: 100, Y: 0}
	c := image.Point{X: 0, Y: 0}
	d := image.Point{X: 50, Y: 50}
	if properSegmentIntersection(a, b, c, d, 1e-6) {
		t.Error("shared endpoint at A=C should not be a proper intersection")
	}
}

func TestProperSegmentIntersectionCrossing(t *testing.T) {
	a := image.Point{X: 0, Y: 0}
	b := image.Point{X: 100, Y: 0}
	c := image.Point{X: 50, Y: -50}
	d := image.Point{X: 50, Y: 50}
	if !properSegmentIntersection(a, b, c, d, 1e-6) {
		t.Error("transversal crossing at midpoint should be a proper intersection")
	}
}

// TestNearestOnPolylineMidpoint sanity-checks the geometry helper used
// to pick which arc a hover/drag belongs to.
func TestNearestOnPolylineMidpoint(t *testing.T) {
	poly := []image.Point{{X: 0, Y: 0}, {X: 100, Y: 0}}
	d2, tparam := nearestOnPolyline(poly, imagePointF{X: 50, Y: 7})
	if d2 < 48 || d2 > 50 {
		t.Errorf("d2: got %v want ~49", d2)
	}
	if tparam < 0.49 || tparam > 0.51 {
		t.Errorf("t: got %v want ~0.5", tparam)
	}
}
