package knotdb

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
)

const testDatasetDirRel = "../dataset"

func TestLoadKnotImg(t *testing.T) {
	datasetDir, _ := filepath.Abs(testDatasetDirRel)
	dbPath, _ := filepath.Abs(testDbRel)

	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("db not loaded yet at %s (run TestLoadKnotInfo first): %v", dbPath, err)
	}
	if _, err := os.Stat(filepath.Join(datasetDir, "diagrams")); err != nil {
		t.Skipf("diagrams dir missing: %v", err)
	}

	n, err := LoadKnotImages(datasetDir, dbPath)
	if err != nil {
		t.Fatalf("LoadKnotImages: %v", err)
	}
	if n == 0 {
		t.Fatalf("no rows inserted")
	}
	t.Logf("loaded images for %d knots", n)

	// Expected count: all knots in knot_info with crossing <= MaxImageCrossing.
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	var rows int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "` + KnotImgTable + `"`).Scan(&rows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rows != n {
		t.Errorf("row count mismatch: insert reported %d, table has %d", n, rows)
	}
}

func TestCheckKnotImage(t *testing.T) {
	datasetDir, _ := filepath.Abs(testDatasetDirRel)
	dbPath, _ := filepath.Abs(testDbRel)
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("db not loaded: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Report totals.
	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "` + KnotImgTable + `"`).Scan(&total); err != nil {
		t.Fatalf("count: %v", err)
	}
	t.Logf("total knots in %s: %d", KnotImgTable, total)
	if total == 0 {
		t.Fatalf("%s is empty (run TestLoadKnotImg first)", KnotImgTable)
	}

	type colInfo struct {
		col     string
		present int
		bytes   int64
	}
	stats := []colInfo{
		{col: "diagram"},
		{col: "diagram_mirror"},
		{col: "snappy"},
		{col: "snappy_mirror"},
		{col: "grid"},
	}
	for i, s := range stats {
		q := `SELECT COUNT(` + quoteIdent(s.col) + `), IFNULL(SUM(LENGTH(` +
			quoteIdent(s.col) + `)), 0) FROM "` + KnotImgTable + `"`
		if err := db.QueryRow(q).Scan(&stats[i].present, &stats[i].bytes); err != nil {
			t.Fatalf("stats for %s: %v", s.col, err)
		}
	}
	t.Logf("%-16s %10s %15s", "column", "nonNull", "bytes")
	for _, s := range stats {
		t.Logf("%-16s %10d %15d", s.col, s.present, s.bytes)
	}

	// Randomly sample rows and verify each BLOB matches the source file.
	sampleSize := 10
	names := loadSampleNames(t, db, sampleSize)
	if len(names) == 0 {
		t.Fatalf("could not sample any knot names")
	}
	t.Logf("verifying %d sampled knots", len(names))

	layout := imageLayout{datasetDir: datasetDir}
	type check struct {
		col  string
		path func(string) string
	}
	checks := []check{
		{"diagram", layout.diagramPath},
		{"diagram_mirror", layout.diagramMirrorPath},
		{"snappy", layout.snappyPath},
		{"snappy_mirror", layout.snappyMirrorPath},
		{"grid", layout.gridPath},
	}

	for _, name := range names {
		for _, c := range checks {
			var blob []byte
			err := db.QueryRow(`SELECT `+quoteIdent(c.col)+` FROM "`+KnotImgTable+`" WHERE name = ?`, name).Scan(&blob)
			if err != nil {
				t.Fatalf("select %s %s: %v", c.col, name, err)
			}

			path := c.path(name)
			fileData, ferr := os.ReadFile(path)
			if ferr != nil {
				// No file on disk -> blob must be NULL/empty.
				if len(blob) != 0 {
					t.Errorf("%s %s: db has %d bytes but file missing (%v)", name, c.col, len(blob), ferr)
				}
				continue
			}
			// File exists -> blob must match bytes.
			if !bytes.Equal(blob, fileData) {
				t.Errorf("%s %s: blob (%d bytes, sha=%x) != file (%d bytes, sha=%x)",
					name, c.col, len(blob), sha256.Sum256(blob), len(fileData), sha256.Sum256(fileData))
			}
		}
	}
}

// loadSampleNames pulls all names, shuffles with a fixed seed, and returns
// up to n of them. Using a deterministic seed keeps test output stable.
func loadSampleNames(t *testing.T, db *sql.DB, n int) []string {
	t.Helper()
	rows, err := db.Query(`SELECT name FROM "` + KnotImgTable + `"`)
	if err != nil {
		t.Fatalf("select names: %v", err)
	}
	defer rows.Close()
	var all []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan: %v", err)
		}
		all = append(all, s)
	}
	if len(all) == 0 {
		return nil
	}
	r := rand.New(rand.NewPCG(1, 2))
	r.Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })
	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}
