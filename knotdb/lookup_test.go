package knotdb

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func smallZipPath(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join(testDatasetRel, KnotInfoSmallFile))
	if err != nil {
		t.Fatalf("abs small zip: %v", err)
	}
	return p
}

func fullZipPath(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join(testDatasetRel, KnotInfoFile))
	if err != nil {
		t.Fatalf("abs full zip: %v", err)
	}
	return p
}

func TestFindKnotRow(t *testing.T) {
	sp := smallZipPath(t)
	if _, err := os.Stat(sp); err != nil {
		t.Skipf("%s not built yet: %v", sp, err)
	}
	useTestDir(t)

	cols, vals, err := FindKnotRow("4_1")
	if err != nil {
		t.Fatalf("FindKnotRow(4_1): %v", err)
	}
	if len(cols) != len(vals) {
		t.Fatalf("len(cols)=%d != len(vals)=%d", len(cols), len(vals))
	}
	if len(cols) != len(SmallColumns) {
		t.Fatalf("small row has %d cols, want %d", len(cols), len(SmallColumns))
	}
	idx := -1
	for i, c := range cols {
		if c == "name" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("no name column")
	}
	if vals[idx] != "4_1" {
		t.Errorf("name = %q, want %q", vals[idx], "4_1")
	}
}

func TestFindKnotRowFull(t *testing.T) {
	fp := fullZipPath(t)
	if _, err := os.Stat(fp); err != nil {
		t.Skipf("%s not built yet: %v", fp, err)
	}
	useTestDir(t)

	cols, vals, err := FindKnotRowFull("4_1")
	if err != nil {
		t.Fatalf("FindKnotRowFull(4_1): %v", err)
	}
	if len(cols) != len(vals) {
		t.Fatalf("len(cols)=%d != len(vals)=%d", len(cols), len(vals))
	}
	idx := -1
	for i, c := range cols {
		if c == "name" {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("no name column")
	}
	if vals[idx] != "4_1" {
		t.Errorf("name = %q, want %q", vals[idx], "4_1")
	}
}

func TestFindKnotRowMissing(t *testing.T) {
	sp := smallZipPath(t)
	if _, err := os.Stat(sp); err != nil {
		t.Skipf("%s not built yet: %v", sp, err)
	}
	useTestDir(t)

	_, _, err := FindKnotRow("this_is_not_a_knot")
	if !errors.Is(err, ErrKnotNotFound) {
		t.Errorf("expected ErrKnotNotFound, got %v", err)
	}
}

func TestRandomKnotName(t *testing.T) {
	sp := smallZipPath(t)
	if _, err := os.Stat(sp); err != nil {
		t.Skipf("%s not built yet: %v", sp, err)
	}
	useTestDir(t)

	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		n, err := RandomKnotName()
		if err != nil {
			t.Fatalf("RandomKnotName: %v", err)
		}
		if n == "" {
			t.Fatal("empty name")
		}
		seen[n] = true
	}
	if len(seen) < 2 {
		t.Errorf("expected variety across 5 picks, got %v", seen)
	}
}

func TestKnotInfoColumns(t *testing.T) {
	cols := KnotInfoColumns()
	if len(cols) != len(SmallColumns) {
		t.Fatalf("KnotInfoColumns len = %d, want %d", len(cols), len(SmallColumns))
	}
	for i, c := range cols {
		if c != SmallColumns[i] {
			t.Errorf("cols[%d] = %q, want %q", i, c, SmallColumns[i])
		}
	}
}

func TestKnotInfoColumnsFull(t *testing.T) {
	fp := fullZipPath(t)
	if _, err := os.Stat(fp); err != nil {
		t.Skipf("%s not built yet: %v", fp, err)
	}
	useTestDir(t)

	cols, err := KnotInfoColumnsFull()
	if err != nil {
		t.Fatalf("KnotInfoColumnsFull: %v", err)
	}
	if len(cols) == 0 {
		t.Fatal("no columns returned")
	}
	if cols[0] != "name" {
		t.Errorf("expected first column %q, got %q", "name", cols[0])
	}
	want := []string{"crossing_number", "jones_polynomial", "signature", "determinant"}
	have := map[string]bool{}
	for _, c := range cols {
		have[c] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("missing expected column %q", w)
		}
	}
}
