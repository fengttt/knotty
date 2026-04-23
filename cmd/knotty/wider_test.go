package main

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"testing"

	"github.com/fengttt/knotty/knot"
)

// TestConvertCrossingCountWide sweeps every canonical prime knot up to 10
// crossings in all four rendered styles (Diagram, DiagramMirror, Snappy,
// SnappyMirror) and checks two invariants per conversion:
//   - crossing count matches KnotInfo crossing_number
//   - over arc-endpoints == under arc-endpoints == 2 × #crossings
//
// Knots that have no stored image for a given style are counted as
// skipped, not failed.
func TestConvertCrossingCountWide(t *testing.T) {
	setupTestDataset(t)

	ranges := []struct {
		cross int
		count int
	}{
		{3, 1},
		{4, 1},
		{5, 2},
		{6, 3},
		{7, 7},
		{8, 21},
		{9, 49},
		{10, 165},
	}
	styles := []knot.ImageType{
		knot.StyleDiagram,
		knot.StyleDiagramMirror,
		knot.StyleSnappy,
		knot.StyleSnappyMirror,
	}
	for _, style := range styles {
		t.Run(style.String(), func(t *testing.T) {
			total, passed, skipped := 0, 0, 0
			var failures []string
			for _, r := range ranges {
				for n := 1; n <= r.count; n++ {
					name := fmt.Sprintf("%d_%d", r.cross, n)
					total++
					msg, result := sweepOne(name, style)
					switch result {
					case sweepPass:
						passed++
					case sweepSkip:
						skipped++
					case sweepFail:
						failures = append(failures, msg)
					}
				}
			}
			t.Logf("%s: passed %d, skipped %d, failed %d (of %d)",
				style, passed, skipped, len(failures), total)
			for _, f := range failures {
				t.Log("  " + f)
			}
			if len(failures) > 0 {
				t.Errorf("%s: %d failures", style, len(failures))
			}
		})
	}
}

type sweepResult int

const (
	sweepPass sweepResult = iota
	sweepSkip
	sweepFail
)

func sweepOne(name string, style knot.ImageType) (string, sweepResult) {
	k, err := knot.FindKnotByName(name)
	if err != nil {
		return fmt.Sprintf("%s: FindKnotByName: %v", name, err), sweepFail
	}
	want := k.GetCrossingNumber()
	data, kind, err := k.LoadImage(style)
	if err != nil {
		return fmt.Sprintf("%s: LoadImage: %v", name, err), sweepFail
	}
	if len(data) == 0 || kind != knot.PNG {
		return "", sweepSkip
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Sprintf("%s: decode: %v", name, err), sweepFail
	}
	d, err := convertImage(img)
	if err != nil {
		return fmt.Sprintf("%s: convertImage: %v", name, err), sweepFail
	}
	got := len(d.Crossings)
	if got != want {
		return fmt.Sprintf("%s: crossings=%d want=%d (arcs=%d)",
			name, got, want, len(d.Arcs)), sweepFail
	}
	over, under := 0, 0
	for _, a := range d.Arcs {
		for _, ep := range [2]Endpoint{a.Start, a.End} {
			if ep.Over {
				over++
			} else {
				under++
			}
		}
	}
	wantEP := 2 * got
	if over != wantEP || under != wantEP {
		return fmt.Sprintf("%s: over=%d under=%d (want %d each); crossings=%d arcs=%d",
			name, over, under, wantEP, got, len(d.Arcs)), sweepFail
	}
	return "", sweepPass
}
