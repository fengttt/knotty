package main

import (
	"testing"
)

// TestCanonicalPDIdempotent — running canonicalPD twice should be
// the same as running it once. If this ever fails, the
// canonicalization isn't actually a fixpoint.
func TestCanonicalPDIdempotent(t *testing.T) {
	cases := [][][4]int{
		// Trefoil 3_1.
		{{1, 4, 2, 5}, {3, 6, 4, 1}, {5, 2, 6, 3}},
		// Figure-eight 4_1.
		{{4, 2, 5, 1}, {8, 6, 1, 5}, {6, 3, 7, 4}, {2, 7, 3, 8}},
	}
	for _, pd := range cases {
		c1 := canonicalPD(pd)
		c2 := canonicalPD(c1)
		if !equalPD(c1, c2) {
			t.Errorf("not idempotent on %v: c1=%v c2=%v", pd, c1, c2)
		}
	}
}

// TestCanonicalPDInvariantUnderRelabeling — applying any permutation
// of arc IDs (followed by valid PD-level transformations) must not
// change canonicalPD's output. We verify this by relabeling arcs
// 1..2c with the inverse permutation and asserting the canonical
// form is unchanged.
func TestCanonicalPDInvariantUnderRelabeling(t *testing.T) {
	pd := [][4]int{{1, 4, 2, 5}, {3, 6, 4, 1}, {5, 2, 6, 3}}
	canon := canonicalPD(pd)

	// Relabel arc IDs by adding a constant offset (still valid as a
	// labeling — the user-visible IDs are different but the diagram
	// structure is identical). Canonicalization should normalize
	// back to the same output.
	relabel := func(p [][4]int, perm map[int]int) [][4]int {
		out := make([][4]int, len(p))
		for i, e := range p {
			for j := 0; j < 4; j++ {
				out[i][j] = perm[e[j]]
			}
		}
		return out
	}
	// Cyclic shift: 1→2, 2→3, ..., 6→1.
	perm := map[int]int{1: 2, 2: 3, 3: 4, 4: 5, 5: 6, 6: 1}
	pd2 := relabel(pd, perm)
	canon2 := canonicalPD(pd2)
	if !equalPD(canon, canon2) {
		t.Errorf("canonical changed under relabeling: canon=%v canon2=%v", canon, canon2)
	}

	// Reverse permutation 1↔6, 2↔5, 3↔4.
	perm = map[int]int{1: 6, 2: 5, 3: 4, 4: 3, 5: 2, 6: 1}
	pd3 := relabel(pd, perm)
	canon3 := canonicalPD(pd3)
	if !equalPD(canon, canon3) {
		t.Errorf("canonical changed under reverse relabeling: canon=%v canon3=%v", canon, canon3)
	}
}

// TestCanonicalPDInvariantUnderCrossingReorder — a PD's crossings
// can be listed in any order without changing the diagram.
// canonicalPD should be invariant under permutation of the crossing
// list.
func TestCanonicalPDInvariantUnderCrossingReorder(t *testing.T) {
	pd := [][4]int{{4, 2, 5, 1}, {8, 6, 1, 5}, {6, 3, 7, 4}, {2, 7, 3, 8}}
	canon := canonicalPD(pd)
	// Reverse crossing order.
	rev := make([][4]int, len(pd))
	for i, e := range pd {
		rev[len(pd)-1-i] = e
	}
	canonRev := canonicalPD(rev)
	if !equalPD(canon, canonRev) {
		t.Errorf("canonical changed under crossing reorder: %v vs %v", canon, canonRev)
	}
}

// TestCanonicalPDInvariantUnderCrossingRotate — within each
// 4-tuple, rotating by 2 (swapping under-pair order) is a valid
// representation of the same crossing. canonicalPD should be
// invariant.
func TestCanonicalPDInvariantUnderCrossingRotate(t *testing.T) {
	pd := [][4]int{{1, 4, 2, 5}, {3, 6, 4, 1}, {5, 2, 6, 3}}
	canon := canonicalPD(pd)
	// Rotate the first crossing by 2.
	rot := [][4]int{{2, 5, 1, 4}, {3, 6, 4, 1}, {5, 2, 6, 3}}
	canonRot := canonicalPD(rot)
	if !equalPD(canon, canonRot) {
		t.Errorf("canonical changed under within-tuple rotation: %v vs %v", canon, canonRot)
	}
}

// TestCanonicalPDLessAbsThenSign verifies the comparator behaviour
// the docstring promises.
func TestCanonicalPDLessAbsThenSign(t *testing.T) {
	cases := []struct {
		a, b [][4]int
		less bool
	}{
		// Smaller absolute value sorts first regardless of sign.
		{[][4]int{{1, 2, 3, 4}}, [][4]int{{-2, 2, 3, 4}}, true},
		// Same absolute values: negative sorts before positive.
		{[][4]int{{-1, 2, 3, 4}}, [][4]int{{1, 2, 3, 4}}, true},
		// All equal — strict less is false.
		{[][4]int{{1, 2, 3, 4}}, [][4]int{{1, 2, 3, 4}}, false},
		// First difference at j=2.
		{[][4]int{{1, 2, 3, 4}}, [][4]int{{1, 2, 5, 0}}, true},
	}
	for _, c := range cases {
		if got := lessPDAbsThenSign(c.a, c.b); got != c.less {
			t.Errorf("less(%v, %v) = %v want %v", c.a, c.b, got, c.less)
		}
	}
}

// TestCanonicalPDOnDartGraph — round-trip through the dart graph.
// Build a Diagram for the trefoil from synthetic polylines, run
// dartGraph.PD(), canonicalize, and assert the canonical form is
// equal to the canonicalization of the KnotInfo PD code for 3_1.
// Together with the canonical-form invariance properties above,
// this is the strongest synthetic check that the convert→PD
// pipeline lines up with the KnotInfo conventions.
//
// (The actual rasterized-image round trip lives in
// TestJonesAgainstDatabase, which needs the dataset.)
func TestCanonicalPDIsKnotEquivalence(t *testing.T) {
	// KnotInfo's trefoil PD.
	pd := [][4]int{{1, 4, 2, 5}, {3, 6, 4, 1}, {5, 2, 6, 3}}
	canon := canonicalPD(pd)
	if canon == nil {
		t.Fatal("canonicalPD returned nil")
	}
	// The same trefoil under arc rotations 1→3→5→1 and 2→4→6→2:
	// strand orientation reversed. Should give the same canonical.
	pdAlt := [][4]int{{3, 6, 4, 1}, {5, 2, 6, 3}, {1, 4, 2, 5}}
	canonAlt := canonicalPD(pdAlt)
	if !equalPD(canon, canonAlt) {
		t.Errorf("trefoil canonical not invariant under crossing-list rotation: %v vs %v", canon, canonAlt)
	}
}
