package main

import (
	"image"
	"testing"
)

func TestClosedPolygonContainsPointSquare(t *testing.T) {
	square := []image.Point{
		{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0},
	}
	cases := []struct {
		p    image.Point
		want bool
	}{
		{image.Point{5, 5}, true},
		{image.Point{0, 5}, true},   // left edge counts as inside under even-odd
		{image.Point{11, 5}, false}, // outside
		{image.Point{-1, 5}, false},
		{image.Point{5, -1}, false},
		{image.Point{5, 11}, false},
	}
	for _, tc := range cases {
		got := closedPolygonContainsPoint(square, tc.p)
		if got != tc.want {
			t.Errorf("contains(%v) = %v, want %v", tc.p, got, tc.want)
		}
	}
}

func TestClosedPolygonContainsPointConcave(t *testing.T) {
	// "C" shape: a 10×10 square with the right-middle scooped out.
	cshape := []image.Point{
		{0, 0}, {10, 0}, {10, 3},
		{4, 3}, {4, 7}, {10, 7},
		{10, 10}, {0, 10}, {0, 0},
	}
	if !closedPolygonContainsPoint(cshape, image.Point{2, 5}) {
		t.Errorf("(2,5) should be inside the C")
	}
	if closedPolygonContainsPoint(cshape, image.Point{7, 5}) {
		t.Errorf("(7,5) sits in the scooped-out cavity, should be OUTSIDE")
	}
	if !closedPolygonContainsPoint(cshape, image.Point{8, 1}) {
		t.Errorf("(8,1) should be inside the upper bar of the C")
	}
}

func TestClosedPolygonContainsPointTiny(t *testing.T) {
	tri := []image.Point{{0, 0}, {2, 0}, {1, 2}}
	if got := closedPolygonContainsPoint(tri, image.Point{1, 1}); !got {
		t.Errorf("(1,1) should be inside the open triangle")
	}
}

func TestClosedPolygonContainsPointDegenerate(t *testing.T) {
	if closedPolygonContainsPoint(nil, image.Point{0, 0}) {
		t.Errorf("nil polygon should never contain anything")
	}
	if closedPolygonContainsPoint([]image.Point{{0, 0}, {1, 0}}, image.Point{0, 0}) {
		t.Errorf("two-point polygon should never contain anything")
	}
}

// ----- R1 / R2 simplification (Phase 2) -----

// makeR1Diagram builds a synthetic Diagram representing a single kink
// at crossing 0 plus a remote crossing 1 with a self-loop, connected
// by two carrier arcs. This is the canonical "kink at one crossing,
// rest of the unknot somewhere else" topology.
//
//	C0 = (100,100): kink crossing
//	C1 = (300,100): remote crossing
//	Arcs (in this order — used to predict indices in tests):
//	  0: L  — kink loop at C0  (over at start, under at end)
//	  1: A  — carrier-over C0 → C1  (Start.Over true at C0)
//	  2: B  — carrier-under C1 → C0 (End.Over false at C0)
//	  3: K  — self-loop at C1
//
// At C0 the four darts are +A (over), +L (over), -L (under), -B
// (under) — valid 2-over / 2-under split. The carrier strand is
// over (entering via A) before the kink and under (exiting via B)
// after, matching the canonical R1 over/under-flip.
func makeR1Diagram() *Diagram {
	c0 := image.Point{X: 100, Y: 100}
	c1 := image.Point{X: 300, Y: 100}
	d := &Diagram{
		Crossings: []image.Point{c0, c1},
		Arcs: []Arc{
			{
				Polyline: []image.Point{c0, {90, 80}, {110, 80}, c0},
				Start:    Endpoint{Crossing: 0, Over: true},
				End:      Endpoint{Crossing: 0, Over: false},
			},
			{
				Polyline: []image.Point{c0, {200, 100}, c1},
				Start:    Endpoint{Crossing: 0, Over: true},
				End:      Endpoint{Crossing: 1, Over: true},
			},
			{
				Polyline: []image.Point{c1, {200, 110}, c0},
				Start:    Endpoint{Crossing: 1, Over: true},
				End:      Endpoint{Crossing: 0, Over: false},
			},
			{
				Polyline: []image.Point{c1, {290, 80}, {310, 80}, c1},
				Start:    Endpoint{Crossing: 1, Over: true},
				End:      Endpoint{Crossing: 1, Over: false},
			},
		},
	}
	return d
}

// lassoSquare returns a closed polygon (last point == first) that's
// an axis-aligned square covering [x1,x2] × [y1,y2].
func lassoSquare(x1, y1, x2, y2 int) []image.Point {
	return []image.Point{
		{X: x1, Y: y1}, {X: x2, Y: y1}, {X: x2, Y: y2}, {X: x1, Y: y2}, {X: x1, Y: y1},
	}
}

func TestDetectR1Simple(t *testing.T) {
	d := makeR1Diagram()
	lasso := lassoSquare(40, 50, 160, 150) // surrounds C0 only
	r1, ok := detectR1(d, lasso)
	if !ok {
		t.Fatalf("detectR1 returned not-ok")
	}
	if r1.crossing != 0 {
		t.Errorf("crossing = %d, want 0", r1.crossing)
	}
	if r1.loop != 0 {
		t.Errorf("loop = %d, want 0 (the L arc)", r1.loop)
	}
	// L.Start.Over=true → carrier1 is the non-loop arc whose
	// v-endpoint is over → A (index 1). carrier2 is the under one
	// → B (index 2).
	if r1.carrier1 != 1 {
		t.Errorf("carrier1 = %d, want 1", r1.carrier1)
	}
	if r1.carrier2 != 2 {
		t.Errorf("carrier2 = %d, want 2", r1.carrier2)
	}
}

func TestDetectR1WrongLasso(t *testing.T) {
	d := makeR1Diagram()
	// Lasso surrounding both crossings → not a single-kink shape.
	lasso := lassoSquare(0, 0, 400, 200)
	if _, ok := detectR1(d, lasso); ok {
		t.Errorf("detectR1 should not match a 2-crossing lasso")
	}
}

func TestApplyR1ProducesCorrectShape(t *testing.T) {
	d := makeR1Diagram()
	lasso := lassoSquare(40, 50, 160, 150)
	r1, _ := detectR1(d, lasso)
	applyR1(d, r1)
	if len(d.Crossings) != 1 {
		t.Errorf("after R1: %d crossings, want 1", len(d.Crossings))
	}
	if len(d.Arcs) != 2 {
		t.Errorf("after R1: %d arcs, want 2", len(d.Arcs))
	}
	// Both surviving arcs should be self-loops at the only remaining
	// crossing (index 0).
	for i, a := range d.Arcs {
		if a.Start.Crossing != 0 || a.End.Crossing != 0 {
			t.Errorf("arc %d after R1: %v→%v, want both at 0",
				i, a.Start.Crossing, a.End.Crossing)
		}
	}
}

// makeR2Diagram builds a 4-crossing Diagram whose first two crossings
// (C0, C1) form a removable R2 (twist) bigon: the over-strand stays
// over at both C0 and C1, the under-strand stays under at both. The
// other two (C2, C3) absorb the four exterior carrier arcs and each
// carries a self-loop so the graph is well-formed.
func makeR2Diagram() *Diagram {
	c0 := image.Point{X: 100, Y: 100}
	c1 := image.Point{X: 200, Y: 100}
	c2 := image.Point{X: 150, Y: 50}
	c3 := image.Point{X: 150, Y: 150}
	d := &Diagram{
		Crossings: []image.Point{c0, c1, c2, c3},
		Arcs: []Arc{
			// 0: arcA — interior, over-strand stays OVER at C0 and C1
			{
				Polyline: []image.Point{c0, {150, 90}, c1},
				Start:    Endpoint{Crossing: 0, Over: true},
				End:      Endpoint{Crossing: 1, Over: true},
			},
			// 1: arcB — interior, under-strand stays UNDER at C0 and C1
			{
				Polyline: []image.Point{c0, {150, 110}, c1},
				Start:    Endpoint{Crossing: 0, Over: false},
				End:      Endpoint{Crossing: 1, Over: false},
			},
			// 2: E1 vOver — C2 → C0, ends over at C0
			{
				Polyline: []image.Point{c2, {120, 70}, c0},
				Start:    Endpoint{Crossing: 2, Over: false},
				End:      Endpoint{Crossing: 0, Over: true},
			},
			// 3: E2 vUnd — C3 → C0, ends under at C0
			{
				Polyline: []image.Point{c3, {120, 130}, c0},
				Start:    Endpoint{Crossing: 3, Over: false},
				End:      Endpoint{Crossing: 0, Over: false},
			},
			// 4: E3 wOver — C1 → C2, starts over at C1
			{
				Polyline: []image.Point{c1, {180, 70}, c2},
				Start:    Endpoint{Crossing: 1, Over: true},
				End:      Endpoint{Crossing: 2, Over: true},
			},
			// 5: E4 wUnd — C1 → C3, starts under at C1
			{
				Polyline: []image.Point{c1, {180, 130}, c3},
				Start:    Endpoint{Crossing: 1, Over: false},
				End:      Endpoint{Crossing: 3, Over: true},
			},
			// 6: K2 — self-loop at C2
			{
				Polyline: []image.Point{c2, {130, 30}, {170, 30}, c2},
				Start:    Endpoint{Crossing: 2, Over: true},
				End:      Endpoint{Crossing: 2, Over: false},
			},
			// 7: K3 — self-loop at C3
			{
				Polyline: []image.Point{c3, {130, 170}, {170, 170}, c3},
				Start:    Endpoint{Crossing: 3, Over: true},
				End:      Endpoint{Crossing: 3, Over: false},
			},
		},
	}
	return d
}

func TestDetectR2Simple(t *testing.T) {
	d := makeR2Diagram()
	lasso := lassoSquare(80, 80, 220, 120) // surrounds C0 and C1 only
	r2, ok := detectR2(d, lasso)
	if !ok {
		t.Fatalf("detectR2 returned not-ok")
	}
	hasV := r2.v == 0 || r2.w == 0
	hasW := r2.v == 1 || r2.w == 1
	if !hasV || !hasW {
		t.Errorf("crossings v=%d w=%d, want 0 and 1 in some order", r2.v, r2.w)
	}
	if (r2.arcA == 0) == (r2.arcB == 0) {
		t.Errorf("one of arcA/arcB should be 0 (other 1); got %d, %d",
			r2.arcA, r2.arcB)
	}
	// Identify v=0's exterior carriers — should be E1=2 (over) and
	// E2=3 (under), independent of which inside crossing got named v.
	at0Over := r2.vOverArc
	at0Und := r2.vUndArc
	if r2.v == 1 {
		at0Over, at0Und = r2.wOverArc, r2.wUndArc
	}
	if at0Over != 2 || at0Und != 3 {
		t.Errorf("at C0: vOver=%d vUnd=%d, want 2 and 3", at0Over, at0Und)
	}
}

func TestApplyR2ProducesCorrectShape(t *testing.T) {
	d := makeR2Diagram()
	lasso := lassoSquare(80, 80, 220, 120)
	r2, ok := detectR2(d, lasso)
	if !ok {
		t.Fatalf("detectR2 returned not-ok")
	}
	applyR2(d, r2)
	if len(d.Crossings) != 2 {
		t.Errorf("after R2: %d crossings, want 2", len(d.Crossings))
	}
	if len(d.Arcs) != 4 {
		t.Errorf("after R2: %d arcs, want 4", len(d.Arcs))
	}
	// All four surviving arcs should reference only crossings 0 and 1
	// (the remaining ones, formerly C2 and C3).
	for i, a := range d.Arcs {
		if a.Start.Crossing < 0 || a.Start.Crossing > 1 ||
			a.End.Crossing < 0 || a.End.Crossing > 1 {
			t.Errorf("arc %d: bad crossing refs %d→%d",
				i, a.Start.Crossing, a.End.Crossing)
		}
	}
}

func TestDetectR2WrongOverPattern(t *testing.T) {
	d := makeR2Diagram()
	// Make arcA flip its over flag between C0 and C1 — that's the
	// trefoil-style "linked" alternating pattern, NOT R2-removable.
	d.Arcs[0].End.Over = false
	lasso := lassoSquare(80, 80, 220, 120)
	if _, ok := detectR2(d, lasso); ok {
		t.Errorf("detectR2 should not match an alternating (linked) bigon")
	}
}

// ----- R3 (strand-slide) -----

// makeR3Diagram builds a 6-crossing Diagram whose crossings 0, 1, 2
// form an R3-applicable triangle with the movable strand running
// between C0 and C1 (over at both endpoints). The remaining
// crossings 3, 4, 5 absorb the six exterior arcs that anchor the
// triangle in the rest of the diagram.
//
// At C0 the four darts come from: arcA-tri (over, to C1), arcA-in
// (under, to C2), and two exterior arcs.
func makeR3Diagram() *Diagram {
	c0 := image.Point{X: 100, Y: 100}
	c1 := image.Point{X: 200, Y: 100}
	c2 := image.Point{X: 150, Y: 50}
	c3 := image.Point{X: 50, Y: 100}
	c4 := image.Point{X: 250, Y: 100}
	c5 := image.Point{X: 150, Y: 200}
	d := &Diagram{
		Crossings: []image.Point{c0, c1, c2, c3, c4, c5},
		Arcs: []Arc{
			// 0: triangle arc C0–C1 — movable strand, OVER at both.
			{
				Polyline: []image.Point{c0, {150, 110}, c1},
				Start:    Endpoint{Crossing: 0, Over: true},
				End:      Endpoint{Crossing: 1, Over: true},
			},
			// 1: triangle arc C0–C2 — alternating (under at C0, over at C2).
			{
				Polyline: []image.Point{c0, {120, 70}, c2},
				Start:    Endpoint{Crossing: 0, Over: false},
				End:      Endpoint{Crossing: 2, Over: true},
			},
			// 2: triangle arc C1–C2 — alternating (over at C1, under
			// at C2; arc 1 already takes the "over at C2" role so
			// strand Z must be under at C2).
			{
				Polyline: []image.Point{c1, {180, 70}, c2},
				Start:    Endpoint{Crossing: 1, Over: true},
				End:      Endpoint{Crossing: 2, Over: false},
			},
			// 3: exterior at C0 — to C3.
			{
				Polyline: []image.Point{c3, {75, 100}, c0},
				Start:    Endpoint{Crossing: 3, Over: false},
				End:      Endpoint{Crossing: 0, Over: true},
			},
			// 4: exterior at C1 — to C4.
			{
				Polyline: []image.Point{c1, {225, 100}, c4},
				Start:    Endpoint{Crossing: 1, Over: true},
				End:      Endpoint{Crossing: 4, Over: false},
			},
			// 5: exterior at C2 — to C5.
			{
				Polyline: []image.Point{c2, {150, 125}, c5},
				Start:    Endpoint{Crossing: 2, Over: false},
				End:      Endpoint{Crossing: 5, Over: false},
			},
			// 6, 7, 8: exterior fillers at C0/C1/C2 to keep them 4-valent.
			{
				Polyline: []image.Point{c0, {100, 75}, c3},
				Start:    Endpoint{Crossing: 0, Over: false},
				End:      Endpoint{Crossing: 3, Over: true},
			},
			{
				Polyline: []image.Point{c1, {200, 75}, c4},
				Start:    Endpoint{Crossing: 1, Over: false},
				End:      Endpoint{Crossing: 4, Over: true},
			},
			{
				Polyline: []image.Point{c2, {150, 25}, c5},
				Start:    Endpoint{Crossing: 2, Over: true},
				End:      Endpoint{Crossing: 5, Over: true},
			},
		},
	}
	return d
}

func TestDetectR3Simple(t *testing.T) {
	d := makeR3Diagram()
	lasso := lassoSquare(80, 30, 220, 130) // surrounds C0, C1, C2
	r3, ok := detectR3(d, lasso)
	if !ok {
		t.Fatalf("detectR3 returned not-ok")
	}
	// The movable arc is index 0 (C0(over)→C1(over)). Pivot is C2.
	if r3.arcAB != 0 {
		t.Errorf("arcAB = %d, want 0", r3.arcAB)
	}
	if r3.pivot != 2 {
		t.Errorf("pivot = %d, want 2", r3.pivot)
	}
	hasC0 := r3.a == 0 || r3.b == 0
	hasC1 := r3.a == 1 || r3.b == 1
	if !hasC0 || !hasC1 {
		t.Errorf("movable endpoints %d,%d, want 0 and 1", r3.a, r3.b)
	}
}

func TestDetectR3WrongPattern(t *testing.T) {
	d := makeR3Diagram()
	// Break the movable strand's matching over flags.
	d.Arcs[0].End.Over = false
	lasso := lassoSquare(80, 30, 220, 130)
	if _, ok := detectR3(d, lasso); ok {
		t.Errorf("detectR3 should not match when no arc has matching over flags")
	}
}

func TestApplyR3MovesAandBAlongTheirStrands(t *testing.T) {
	d := makeR3Diagram()
	// Lasso that excludes C3 (x=50) and C4 (x=250) but extends down
	// past the lasso-boundary crossings of arcs 5 and 8 (the
	// exterior arcs at C2).
	lasso := lassoSquare(51, 30, 249, 199)
	r3, ok := detectR3(d, lasso)
	if !ok {
		t.Fatalf("detectR3 returned not-ok")
	}
	oldA := d.Crossings[r3.a]
	oldB := d.Crossings[r3.b]
	piv := d.Crossings[r3.pivot]
	applyR3(d, r3, lasso)
	if d.Crossings[r3.pivot] != piv {
		t.Errorf("pivot moved to %v, want %v (unchanged)", d.Crossings[r3.pivot], piv)
	}
	if d.Crossings[r3.a] == oldA {
		t.Errorf("a unchanged at %v — should have slid along strand Y", oldA)
	}
	if d.Crossings[r3.b] == oldB {
		t.Errorf("b unchanged at %v — should have slid along strand Z", oldB)
	}
	if !closedPolygonContainsPoint(lasso, d.Crossings[r3.a]) {
		t.Errorf("a at %v left the lasso", d.Crossings[r3.a])
	}
	if !closedPolygonContainsPoint(lasso, d.Crossings[r3.b]) {
		t.Errorf("b at %v left the lasso", d.Crossings[r3.b])
	}
	if d.Crossings[r3.a] == d.Crossings[r3.b] {
		t.Errorf("a and b collapsed to the same point %v", d.Crossings[r3.a])
	}
	if len(d.Crossings) != 6 || len(d.Arcs) != 9 {
		t.Errorf("counts after R3: %d crossings, %d arcs, want 6 and 9",
			len(d.Crossings), len(d.Arcs))
	}
}


// TestArcInLassoStats checks that arcInLassoStats correctly classifies
// arcs as inside-only, outside-only, or boundary-crossing.
func TestArcInLassoStats(t *testing.T) {
	square := []image.Point{
		{0, 0}, {100, 0}, {100, 100}, {0, 100}, {0, 0},
	}
	// Fully inside: every point of the polyline is inside the square.
	inside := []image.Point{{20, 20}, {30, 30}, {40, 40}}
	any, all := arcInLassoStats(square, inside)
	if !any || !all {
		t.Errorf("inside arc: any=%v all=%v, want both true", any, all)
	}
	// Fully outside.
	outside := []image.Point{{200, 200}, {220, 210}}
	any, all = arcInLassoStats(square, outside)
	if any || all {
		t.Errorf("outside arc: any=%v all=%v, want both false", any, all)
	}
	// Boundary-crossing.
	crossing := []image.Point{{50, 50}, {120, 50}}
	any, all = arcInLassoStats(square, crossing)
	if !any || all {
		t.Errorf("crossing arc: any=%v all=%v, want any=true all=false", any, all)
	}
}
