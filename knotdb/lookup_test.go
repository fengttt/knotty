package knotdb

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func jsonPath(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join(testDatasetRel, KnotInfoFile))
	if err != nil {
		t.Fatalf("abs json: %v", err)
	}
	return p
}

func TestFindKnotRow(t *testing.T) {
	jp := jsonPath(t)
	if _, err := os.Stat(jp); err != nil {
		t.Skipf("%s not built yet: %v", jp, err)
	}
	useTestDir(t)

	cols, vals, err := FindKnotRow("4_1")
	if err != nil {
		t.Fatalf("FindKnotRow(4_1): %v", err)
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
	jp := jsonPath(t)
	if _, err := os.Stat(jp); err != nil {
		t.Skipf("%s not built yet: %v", jp, err)
	}
	useTestDir(t)

	_, _, err := FindKnotRow("this_is_not_a_knot")
	if !errors.Is(err, ErrKnotNotFound) {
		t.Errorf("expected ErrKnotNotFound, got %v", err)
	}
}

func TestRandomKnotName(t *testing.T) {
	jp := jsonPath(t)
	if _, err := os.Stat(jp); err != nil {
		t.Skipf("%s not built yet: %v", jp, err)
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
	jp := jsonPath(t)
	if _, err := os.Stat(jp); err != nil {
		t.Skipf("%s not built yet: %v", jp, err)
	}
	useTestDir(t)

	cols, err := KnotInfoColumns()
	if err != nil {
		t.Fatalf("KnotInfoColumns: %v", err)
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
