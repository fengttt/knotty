package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/png"
	"sort"
	"strings"
	"testing"

	"github.com/fengttt/knotty/knot"
	"github.com/fengttt/knotty/knotdb"
)

// TestJonesAgainstDatabase walks every prime knot up to 11 crossings
// in two render styles (Diagram and Snappy), reconstructs the diagram
// from the rasterized image, computes the Jones polynomial via
// Kauffman bracket + writhe, and compares against the polynomial
// stored in the dataset.
//
// Comparison rules:
//   - Jones is parsed from the dataset string into a sparse Laurent
//     polynomial in t.
//   - We accept either V(t) or V(1/t) — the converted image's
//     chirality is not guaranteed to match the dataset's, and a
//     knot's mirror has Jones polynomial V_K(1/t).
//   - Knots without a stored image for the given style are skipped
//     (counted but not failed). Knots whose convertImage fails are
//     also skipped (already-known input quirks; the existing
//     wider_test catches those separately).
//   - PD codes are computed and printed but not compared exactly:
//     arc-numbering conventions differ between our walk and the
//     dataset, so an exact match is too brittle. Structural
//     soundness (right number of entries, every arc appears twice,
//     odd/even alternation at each crossing) is already covered by
//     TestPDStructure. The Jones match is the rigorous structural
//     check.
//
// The test is verbose by design — it logs a per-name pass/skip/
// fail line for every knot+style combination, so the output is a
// triage report when something regresses.
func TestJonesAgainstDatabase(t *testing.T) {
	setupTestDataset(t)

	// Probe one canonical knot up front; if the dataset is a stub
	// (LFS pointer text rather than a real zip), every lookup will
	// fail with the same parse error and the test would otherwise
	// emit ~800 identical failure lines. Skip cleanly instead.
	if _, err := knot.FindKnotByName("3_1"); err != nil && !errors.Is(err, knotdb.ErrKnotNotFound) {
		t.Skipf("dataset unavailable: %v", err)
	}

	type bucket struct {
		label  string
		prefix string // "" for plain N_M; "a" or "n" for 11a_M / 11n_M
		count  int
	}
	// Counts are KnotInfo's prime-knot enumeration. Slightly over-
	// counting on 11a/11n is harmless — the per-name skip-on-
	// not-found logic catches off-by-one without failing the test.
	groups := []bucket{
		{label: "3", count: 1},
		{label: "4", count: 1},
		{label: "5", count: 2},
		{label: "6", count: 3},
		{label: "7", count: 7},
		{label: "8", count: 21},
		{label: "9", count: 49},
		{label: "10", count: 165},
		{label: "11", prefix: "a", count: 367},
		{label: "11", prefix: "n", count: 185},
	}
	styles := []knot.ImageType{knot.StyleDiagram, knot.StyleSnappy}

	for _, style := range styles {
		t.Run(style.String(), func(t *testing.T) {
			var (
				total, passed, skipped int
				fails                  []string
			)
			for _, g := range groups {
				for n := 1; n <= g.count; n++ {
					var name string
					if g.prefix == "" {
						name = fmt.Sprintf("%s_%d", g.label, n)
					} else {
						name = fmt.Sprintf("%s%s_%d", g.label, g.prefix, n)
					}
					total++
					switch msg, res := compareJones(name, style); res {
					case sweepPass:
						passed++
					case sweepSkip:
						skipped++
					case sweepFail:
						fails = append(fails, msg)
					}
				}
			}
			t.Logf("%s: passed %d, skipped %d, failed %d (of %d)",
				style, passed, skipped, len(fails), total)
			// Cap the dump so a wholesale regression doesn't drown the
			// test log.
			const dumpLimit = 50
			sort.Strings(fails)
			for i, f := range fails {
				if i >= dumpLimit {
					t.Logf("  ... (%d more failures elided)", len(fails)-dumpLimit)
					break
				}
				t.Log("  " + f)
			}
			if len(fails) > 0 {
				t.Errorf("%s: %d Jones mismatches", style, len(fails))
			}
		})
	}
}

// compareJones runs the convert→Jones pipeline against one knot and
// returns (msg, sweepPass | sweepSkip | sweepFail). The msg is a
// short human-readable failure line (empty on pass / skip).
func compareJones(name string, style knot.ImageType) (string, sweepResult) {
	k, err := knot.FindKnotByName(name)
	if err != nil {
		if errors.Is(err, knotdb.ErrKnotNotFound) {
			return "", sweepSkip
		}
		return fmt.Sprintf("%s: FindKnotByName: %v", name, err), sweepFail
	}
	wantStr := k.GetJonesPolynomial()
	if strings.TrimSpace(wantStr) == "" {
		return "", sweepSkip
	}
	want, err := parseJonesPoly(wantStr)
	if err != nil {
		return fmt.Sprintf("%s: parseJonesPoly(%q): %v", name, wantStr, err), sweepFail
	}

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
		// Convert failures are tracked by TestConvertCrossingCountWide;
		// don't double-count them here.
		return "", sweepSkip
	}
	g, err := newDartGraph(d)
	if err != nil {
		return "", sweepSkip
	}
	if g.NumComponents() != 1 {
		return fmt.Sprintf("%s: NumComponents=%d (expected 1)", name, g.NumComponents()), sweepFail
	}
	got, err := g.jones()
	if err != nil {
		return fmt.Sprintf("%s: jones: %v", name, err), sweepFail
	}
	if polyEqual(got, want) || polyEqual(got, mirrorPoly(want)) {
		return "", sweepPass
	}
	return fmt.Sprintf("%s: V(t)=%s; want %s (or its mirror)",
		name, formatPoly(got, "t"), formatPoly(want, "t")), sweepFail
}
