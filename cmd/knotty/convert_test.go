package main

import (
	"bytes"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/fengttt/knotty/knot"
	"github.com/fengttt/knotty/knotdb"
)

func setupTestDataset(t *testing.T) {
	t.Helper()
	dir, err := filepath.Abs("../../dataset")
	if err != nil {
		t.Fatalf("abs dir: %v", err)
	}
	sp, err := filepath.Abs(filepath.Join("../../dataset", knotdb.KnotInfoSmallFile))
	if err != nil {
		t.Fatalf("abs small: %v", err)
	}
	if _, err := os.Stat(sp); err != nil {
		t.Skipf("%s not built at %s: %v", knotdb.KnotInfoSmallFile, sp, err)
	}
	prev := knotdb.Dir()
	knotdb.SetDir(dir)
	t.Cleanup(func() {
		knotdb.SetDir(prev)
		knotdb.Reset()
	})
}

// TestConvertCrossingCount runs the image-to-diagram pipeline on the
// canonical low-crossing knots and checks that the number of crossings
// detected matches the knot's crossing_number from KnotInfo.
func TestConvertCrossingCount(t *testing.T) {
	setupTestDataset(t)

	names := []string{
		"3_1",
		"4_1",
		"5_1", "5_2",
		"6_1", "6_2", "6_3",
		"7_1", "7_2", "7_3", "7_4", "7_5", "7_6", "7_7",
	}
	for _, style := range []knot.ImageType{knot.StyleDiagram, knot.StyleSnappy} {
		for _, name := range names {
			t.Run(style.String()+"/"+name, func(t *testing.T) {
				checkConvertCrossings(t, name, style)
			})
		}
	}
}

// TestConvert10_42Mirror exercises the mirror diagram of 10_42 — the
// first knot that forced the snap-to-nearest-edge fallback for the
// non-mirror PNG. Mirroring swaps over/under at every crossing, so the
// skeleton has a different shape near the problematic short-match gap,
// and this guards against regressions specific to the mirror rendering.
func TestConvert10_42Mirror(t *testing.T) {
	setupTestDataset(t)
	checkConvertCrossings(t, "10_42", knot.StyleDiagramMirror)
}

// TestConvertArcOverUnderBalance checks that the number of over
// arc-endpoints equals the number of under arc-endpoints. At every
// crossing 4 arc-endpoints meet (2 from the over strand, 2 from the
// under), so the totals must match. We start with 4_1 because it's the
// smallest non-trivial knot where an imbalance has been observed.
func TestConvertArcOverUnderBalance(t *testing.T) {
	setupTestDataset(t)

	names := []string{
		"3_1",
		"4_1",
		"5_1", "5_2",
		"6_1", "6_2", "6_3",
		"7_1", "7_2", "7_3", "7_4", "7_5", "7_6", "7_7",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			k, err := knot.FindKnotByName(name)
			if err != nil {
				t.Fatalf("FindKnotByName: %v", err)
			}
			data, kind, err := k.LoadImage(knot.StyleDiagram)
			if err != nil || kind != knot.PNG || len(data) == 0 {
				t.Skipf("no PNG Diagram for %s", name)
			}
			img, _, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			d, err := convertImage(img)
			if err != nil {
				t.Fatalf("convertImage: %v", err)
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
			want := 2 * len(d.Crossings)
			if over != want || under != want {
				t.Errorf("%s: over=%d under=%d (want %d each); crossings=%d arcs=%d",
					name, over, under, want, len(d.Crossings), len(d.Arcs))
			}
		})
	}
}

func checkConvertCrossings(t *testing.T, name string, style knot.ImageType) {
	t.Helper()
	k, err := knot.FindKnotByName(name)
	if err != nil {
		t.Fatalf("FindKnotByName(%s): %v", name, err)
	}
	want := k.GetCrossingNumber()
	data, kind, err := k.LoadImage(style)
	if err != nil {
		t.Fatalf("LoadImage: %v", err)
	}
	if len(data) == 0 {
		t.Skipf("no %s image for %s", style, name)
	}
	if kind != knot.PNG {
		t.Skipf("%s %s is %s, not PNG", name, style, kind)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	d, err := convertImage(img)
	if err != nil {
		t.Errorf("convertImage: %v", err)
		return
	}
	got := len(d.Crossings)
	if got != want {
		t.Errorf("crossings = %d, want %d (arcs=%d)", got, want, len(d.Arcs))
		return
	}
	t.Logf("ok: %d crossings, %d arcs", got, len(d.Arcs))
}
