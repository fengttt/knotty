package main

import (
	"image"
	"sort"
	"testing"

	"github.com/fengttt/knotty/knot"
)

// pdSorted returns a canonical form of a PD listing: each entry sorted in
// ascending order, then the list of entries sorted lexicographically. Two
// PD listings that describe the same diagram up to relabelling and starting
// position have the same canonical form.
func pdSorted(pd [][4]int) [][4]int {
	out := make([][4]int, len(pd))
	for i, e := range pd {
		v := [4]int{e[0], e[1], e[2], e[3]}
		sort.Ints(v[:])
		out[i] = v
	}
	sort.Slice(out, func(i, j int) bool {
		for k := 0; k < 4; k++ {
			if out[i][k] != out[j][k] {
				return out[i][k] < out[j][k]
			}
		}
		return false
	})
	return out
}

func pdEqual(a, b [][4]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestBeautifyPreservesTopology checks that Beautify returns a diagram with
// the same PD code (up to relabelling/starting offset) as the input, for the
// canonical small prime knots.
func TestBeautifyPreservesTopology(t *testing.T) {
	setupTestDataset(t)
	names := []string{"3_1", "4_1", "5_1", "5_2", "6_1"}
	styles := []knot.ImageType{
		knot.StyleDiagram,
		knot.StyleSnappy,
	}
	for _, style := range styles {
		for _, name := range names {
			t.Run(style.String()+"/"+name, func(t *testing.T) {
				d, skip := loadAndConvert(t, name, style)
				if skip {
					t.Skipf("no %s image", style)
				}
				inPD, err := d.PD()
				if err != nil {
					t.Fatalf("input PD: %v", err)
				}
				out, err := d.Beautify(540, 540)
				if err != nil {
					t.Fatalf("Beautify: %v", err)
				}
				if len(out.Crossings) != len(d.Crossings) {
					t.Fatalf("crossing count %d != %d", len(out.Crossings), len(d.Crossings))
				}
				if len(out.Arcs) != len(d.Arcs) {
					t.Fatalf("arc count %d != %d", len(out.Arcs), len(d.Arcs))
				}
				outPD, err := out.PD()
				if err != nil {
					t.Fatalf("output PD: %v", err)
				}
				if !pdEqual(pdSorted(inPD), pdSorted(outPD)) {
					t.Fatalf("PD changed:\n in:  %v\n out: %v", pdSorted(inPD), pdSorted(outPD))
				}
				// All crossings should sit inside the canvas with margin.
				for i, c := range out.Crossings {
					if c.X < 0 || c.X > 540 || c.Y < 0 || c.Y > 540 {
						t.Errorf("crossing %d at %v out of canvas", i, c)
					}
				}
				// Each polyline is the original arc's chain through 7 medial
				// midpoints plus 2 endpoints.
				for i, a := range out.Arcs {
					if len(a.Polyline) != 9 {
						t.Errorf("arc %d polyline has %d points (want 9)", i, len(a.Polyline))
					}
					if a.Polyline[0] != out.Crossings[a.Start.Crossing] {
						t.Errorf("arc %d start polyline[0] %v != crossing %v", i, a.Polyline[0], out.Crossings[a.Start.Crossing])
					}
					if a.Polyline[len(a.Polyline)-1] != out.Crossings[a.End.Crossing] {
						t.Errorf("arc %d end polyline[-1] %v != crossing %v", i, a.Polyline[len(a.Polyline)-1], out.Crossings[a.End.Crossing])
					}
				}
			})
		}
	}
}

// TestBeautifyEmptyDiagram makes sure a Diagram with no crossings is a no-op.
func TestBeautifyEmptyDiagram(t *testing.T) {
	d := &Diagram{Crossings: []image.Point{}, Arcs: nil}
	out, err := d.Beautify(540, 540)
	if err != nil {
		t.Fatalf("Beautify: %v", err)
	}
	if out != d {
		t.Errorf("expected same Diagram pointer for empty input")
	}
}
