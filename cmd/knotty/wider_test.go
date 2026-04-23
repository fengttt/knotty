package main

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"testing"

	"github.com/fengttt/knotty/knot"
)

// TestConvertCrossingCountWide sweeps the canonical prime knots from 8 to
// 10 crossings in both the Diagram and Snappy styles, and reports every
// mismatch. Useful for catching regressions when tuning the convert-to-
// diagram heuristics.
func TestConvertCrossingCountWide(t *testing.T) {
	setupTestDataset(t)

	ranges := []struct {
		cross int
		count int
	}{
		{8, 21},
		{9, 49},
		{10, 165},
	}
	for _, style := range []knot.ImageType{knot.StyleDiagram, knot.StyleSnappy} {
		t.Run(style.String(), func(t *testing.T) {
			total, passed := 0, 0
			var failures []string
			for _, r := range ranges {
				for n := 1; n <= r.count; n++ {
					name := fmt.Sprintf("%d_%d", r.cross, n)
					total++
					msg, ok := sweepOne(name, style)
					if ok {
						passed++
					} else {
						failures = append(failures, msg)
					}
				}
			}
			t.Logf("%s: passed %d/%d", style, passed, total)
			for _, f := range failures {
				t.Log("  " + f)
			}
			if passed != total {
				t.Errorf("%s: %d/%d failed", style, total-passed, total)
			}
		})
	}
}

func sweepOne(name string, style knot.ImageType) (string, bool) {
	k, err := knot.FindKnotByName(name)
	if err != nil {
		return fmt.Sprintf("%s: FindKnotByName: %v", name, err), false
	}
	want := k.GetCrossingNumber()
	data, kind, err := k.LoadImage(style)
	if err != nil || len(data) == 0 || kind != knot.PNG {
		return fmt.Sprintf("%s: no PNG image", name), false
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Sprintf("%s: decode: %v", name, err), false
	}
	d, err := convertImage(img)
	if err != nil {
		return fmt.Sprintf("%s: convertImage: %v", name, err), false
	}
	got := len(d.Crossings)
	if got != want {
		return fmt.Sprintf("%s: crossings=%d want=%d (arcs=%d)",
			name, got, want, len(d.Arcs)), false
	}
	return "", true
}
