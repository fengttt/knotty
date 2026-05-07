package main

import "testing"

func TestHOMFLYUnknot(t *testing.T) {
	p := homflyFromPD(nil)
	want := poly2{[2]int{0, 0}: 1}
	if !poly2Equal(p, want) {
		t.Errorf("HOMFLY(unknot) = %v want %v", p, want)
	}
	if got := formatPoly2(p, "a", "z"); got != "1" {
		t.Errorf("formatted HOMFLY(unknot) = %q want %q", got, "1")
	}
}

func TestHOMFLYTrefoil(t *testing.T) {
	pd := [][4]int{{1, 4, 2, 5}, {3, 6, 4, 1}, {5, 2, 6, 3}}
	p := homflyFromPD(pd)
	t.Logf("Trefoil HOMFLY: P(a,z) = %s", formatPoly2(p, "a", "z"))

	wantLeft := poly2{
		[2]int{4, 0}: -1,
		[2]int{2, 0}: 2,
		[2]int{2, 2}: 1,
	}
	wantRight := poly2{
		[2]int{-4, 0}: -1,
		[2]int{-2, 0}: 2,
		[2]int{-2, 2}: 1,
	}
	if !poly2Equal(p, wantLeft) && !poly2Equal(p, wantRight) {
		t.Errorf("HOMFLY(trefoil) = %v\n  want left=%v\n  or right=%v", p, wantLeft, wantRight)
	}
}

func TestHOMFLYFigureEight(t *testing.T) {
	pd := [][4]int{{4, 2, 5, 1}, {8, 6, 1, 5}, {6, 3, 7, 4}, {2, 7, 3, 8}}
	p := homflyFromPD(pd)
	t.Logf("Figure-eight HOMFLY: P(a,z) = %s", formatPoly2(p, "a", "z"))

	want := poly2{
		[2]int{2, 0}:  1,
		[2]int{0, 2}:  -1,
		[2]int{0, 0}:  -1,
		[2]int{-2, 0}: 1,
	}
	if !poly2Equal(p, want) {
		t.Errorf("HOMFLY(4_1) = %v want %v", p, want)
	}
}

func poly2Equal(a, b poly2) bool {
	count := func(p poly2) int {
		n := 0
		for _, v := range p {
			if v != 0 {
				n++
			}
		}
		return n
	}
	if count(a) != count(b) {
		return false
	}
	for k, v := range a {
		if v == 0 {
			continue
		}
		if b[k] != v {
			return false
		}
	}
	return true
}
