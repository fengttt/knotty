package knotdb

import (
	"database/sql"
	"errors"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func TestEnsureIndexes(t *testing.T) {
	dbPath := useTestDB(t)
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("db not loaded yet at %s: %v", dbPath, err)
	}

	if err := EnsureIndexes(); err != nil {
		t.Fatalf("EnsureIndexes: %v", err)
	}
	// Idempotent.
	if err := EnsureIndexes(); err != nil {
		t.Fatalf("EnsureIndexes (second call): %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	have := map[string]bool{}
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='index'`)
	if err != nil {
		t.Fatalf("query indexes: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		have[n] = true
	}
	for _, want := range []string{"idx_knot_info_name", "idx_knot_img_name"} {
		if !have[want] {
			t.Errorf("missing index %q", want)
		}
	}
}

func TestFindKnotRow(t *testing.T) {
	dbPath := useTestDB(t)
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("db not loaded yet at %s: %v", dbPath, err)
	}

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
	dbPath := useTestDB(t)
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("db not loaded yet at %s: %v", dbPath, err)
	}
	_, _, err := FindKnotRow("this_is_not_a_knot")
	if !errors.Is(err, ErrKnotNotFound) {
		t.Errorf("expected ErrKnotNotFound, got %v", err)
	}
}

func TestRandomKnotName(t *testing.T) {
	dbPath := useTestDB(t)
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("db not loaded yet at %s: %v", dbPath, err)
	}
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
	// With ~12966 rows, five picks should very rarely collide.
	if len(seen) < 2 {
		t.Errorf("expected variety across 5 picks, got %v", seen)
	}
}

func TestLoadImageBlob(t *testing.T) {
	dbPath := useTestDB(t)
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("db not loaded yet at %s: %v", dbPath, err)
	}

	// 0_1 (unknot) has no images.
	data, err := LoadImageBlob("0_1", "diagram")
	if err != nil {
		t.Fatalf("LoadImageBlob(0_1, diagram): %v", err)
	}
	if len(data) != 0 {
		t.Errorf("0_1 diagram: expected empty, got %d bytes", len(data))
	}

	// 3_1 should have a diagram (PNG).
	data, err = LoadImageBlob("3_1", "diagram")
	if err != nil {
		t.Fatalf("LoadImageBlob(3_1, diagram): %v", err)
	}
	if len(data) == 0 {
		t.Errorf("3_1 diagram: got 0 bytes")
	}
	want := []byte{0x89, 'P', 'N', 'G'}
	for i, b := range want {
		if i >= len(data) {
			t.Fatalf("3_1 diagram: too short")
		}
		if data[i] != b {
			t.Errorf("3_1 diagram byte %d = %#x, want %#x", i, data[i], b)
			break
		}
	}
}
