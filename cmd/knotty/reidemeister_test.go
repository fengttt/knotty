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
