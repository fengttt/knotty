package main

import (
	"testing"
)

// polyEqual compares two Laurent polynomials disregarding zero terms,
// so a {0:0} entry from one side doesn't trip a comparison against a
// missing key on the other.
func polyEqual(a, b poly) bool {
	count := func(p poly) int {
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

// TestJonesUnknot — V(unknot) = 1 by normalization. We feed an empty
// PD code (the convention used by bracketFromPD for a no-crossing
// diagram) and check the round-trip.
func TestJonesUnknot(t *testing.T) {
	br := bracketFromPD(nil)
	if !polyEqual(br, poly{0: 1}) {
		t.Errorf("bracket(unknot) = %v want {0:1}", br)
	}
	j, err := jonesFromBracketWrithe(br, 0)
	if err != nil {
		t.Fatalf("jones: %v", err)
	}
	if !polyEqual(j, poly{0: 1}) {
		t.Errorf("V(unknot) = %v want {0:1}", j)
	}
	if got := formatPoly(j, "t"); got != "1" {
		t.Errorf("formatted V(unknot) = %q want %q", got, "1")
	}
}

// TestJonesTrefoil — V(left trefoil) = -t^-4 + t^-3 + t^-1, and the
// right trefoil is its mirror with V(t) = -t^4 + t^3 + t. The
// KnotInfo PD code for 3_1 doesn't fix the orientation, so we accept
// either; the bracket exponents are all in one residue class mod 4
// so only one writhe sign produces integer t-powers — the other
// will legitimately error from jonesFromBracketWrithe and we treat
// that as "doesn't match" rather than a test failure.
func TestJonesTrefoil(t *testing.T) {
	pd := [][4]int{{1, 4, 2, 5}, {3, 6, 4, 1}, {5, 2, 6, 3}}
	br := bracketFromPD(pd)

	tryWrithe := func(w int, want poly) bool {
		j, err := jonesFromBracketWrithe(br, w)
		return err == nil && polyEqual(j, want)
	}
	wantLeft := poly{-4: -1, -3: 1, -1: 1}
	wantRight := poly{4: -1, 3: 1, 1: 1}

	if !tryWrithe(-3, wantLeft) && !tryWrithe(3, wantRight) {
		jL, eL := jonesFromBracketWrithe(br, -3)
		jR, eR := jonesFromBracketWrithe(br, 3)
		t.Errorf("trefoil: V(writhe=-3)=%v err=%v; V(writhe=+3)=%v err=%v; expected one of %v or %v",
			jL, eL, jR, eR, wantLeft, wantRight)
	}
}

// TestJonesFigureEight — V(4_1) = t^-2 - t^-1 + 1 - t + t^2. The
// figure-eight is amphichiral so writhe = 0 regardless of orientation
// choice; this is the cleanest single-known-value end-to-end check.
func TestJonesFigureEight(t *testing.T) {
	pd := [][4]int{{4, 2, 5, 1}, {8, 6, 1, 5}, {6, 3, 7, 4}, {2, 7, 3, 8}}
	br := bracketFromPD(pd)
	j, err := jonesFromBracketWrithe(br, 0)
	if err != nil {
		t.Fatalf("jones: %v", err)
	}
	want := poly{-2: 1, -1: -1, 0: 1, 1: -1, 2: 1}
	if !polyEqual(j, want) {
		t.Errorf("V(4_1) = %v want %v", j, want)
	}
	const wantFmt = "t^(-2) - t^(-1) + 1 - t + t^2"
	if got := formatPoly(j, "t"); got != wantFmt {
		t.Errorf("formatted V(4_1) = %q want %q", got, wantFmt)
	}
}

// TestFormatPolyEdgeCases sanity-checks the printer on terms that
// matter for the doConvert output: empty polynomial, single
// constant, single negative leading term, and mixed-sign series.
func TestFormatPolyEdgeCases(t *testing.T) {
	cases := []struct {
		in   poly
		want string
	}{
		{poly{}, "0"},
		{poly{0: 1}, "1"},
		{poly{0: -1}, "-1"},
		{poly{1: 1}, "t"},
		{poly{1: -1}, "-t"},
		{poly{-1: 1}, "t^(-1)"},
		{poly{2: 3}, "3t^2"},
		{poly{-2: -3}, "-3t^(-2)"},
		{poly{0: 1, 1: -2, 2: 1}, "1 - 2t + t^2"},
		// Zero coefficients should be invisible.
		{poly{0: 0, 1: 1}, "t"},
	}
	for _, c := range cases {
		if got := formatPoly(c.in, "t"); got != c.want {
			t.Errorf("formatPoly(%v) = %q want %q", c.in, got, c.want)
		}
	}
}

// TestFormatPD ensures the X[a,b,c,d] joiner handles single and
// multi-entry diagrams the same way KnotInfo prints them.
func TestFormatPD(t *testing.T) {
	cases := []struct {
		in   [][4]int
		want string
	}{
		{nil, ""},
		{[][4]int{{1, 2, 3, 4}}, "X[1,2,3,4]"},
		{[][4]int{{1, 4, 2, 5}, {3, 6, 4, 1}, {5, 2, 6, 3}},
			"X[1,4,2,5], X[3,6,4,1], X[5,2,6,3]"},
	}
	for _, c := range cases {
		if got := formatPD(c.in); got != c.want {
			t.Errorf("formatPD(%v) = %q want %q", c.in, got, c.want)
		}
	}
}
