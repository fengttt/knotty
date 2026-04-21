package knotdb

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// Paths are resolved relative to the knotdb package directory (go test cwd).
const (
	testXlsRel = "../dataset/knotinfo_data_complete.xls"
	testDbRel  = "../dataset/knotty.sqlite3"
)

func TestLoadKnotInfo(t *testing.T) {
	xlsPath, err := filepath.Abs(testXlsRel)
	if err != nil {
		t.Fatalf("abs xls: %v", err)
	}
	dbPath, err := filepath.Abs(testDbRel)
	if err != nil {
		t.Fatalf("abs db: %v", err)
	}
	if _, err := os.Stat(xlsPath); err != nil {
		t.Skipf("xls not available at %s: %v", xlsPath, err)
	}

	n, err := LoadKnotInfo(xlsPath, dbPath)
	if err != nil {
		t.Fatalf("LoadKnotInfo: %v", err)
	}
	if n == 0 {
		t.Fatalf("LoadKnotInfo inserted 0 rows")
	}
	t.Logf("inserted %d rows into %s", n, dbPath)

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("sqlite file not created: %v", err)
	}
}

func TestCheckLoadKnotInfo(t *testing.T) {
	dbPath, err := filepath.Abs(testDbRel)
	if err != nil {
		t.Fatalf("abs db: %v", err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("db not loaded yet at %s: %v", dbPath, err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	var total int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "` + KnotInfoTable + `"`).Scan(&total); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if total == 0 {
		t.Fatalf("%s is empty", KnotInfoTable)
	}
	t.Logf("total knots loaded: %d", total)

	crossingCol := findCrossingColumn(t, db)
	t.Logf("using crossing column: %q", crossingCol)

	// Group by crossing number and report counts. Sort numerically when possible.
	q := `SELECT ` + quoteIdent(crossingCol) + `, COUNT(*) FROM "` + KnotInfoTable +
		`" GROUP BY ` + quoteIdent(crossingCol)
	rows, err := db.Query(q)
	if err != nil {
		t.Fatalf("group by crossing: %v", err)
	}
	defer rows.Close()

	type bucket struct {
		crossing string
		count    int
	}
	var buckets []bucket
	var grandTotal int
	for rows.Next() {
		var cn sql.NullString
		var c int
		if err := rows.Scan(&cn, &c); err != nil {
			t.Fatalf("scan: %v", err)
		}
		buckets = append(buckets, bucket{crossing: cn.String, count: c})
		grandTotal += c
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows err: %v", err)
	}
	if len(buckets) == 0 {
		t.Fatalf("no crossing-number groups found")
	}
	if grandTotal != total {
		t.Fatalf("group-by total %d != row count %d", grandTotal, total)
	}

	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].crossing < buckets[j].crossing
	})
	for _, b := range buckets {
		t.Logf("crossing=%q count=%d", b.crossing, b.count)
	}
}

func TestKnotInfoColumns(t *testing.T) {
	dbPath, err := filepath.Abs(testDbRel)
	if err != nil {
		t.Fatalf("abs db: %v", err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("db not loaded yet at %s: %v", dbPath, err)
	}

	cols, err := KnotInfoColumns(dbPath)
	if err != nil {
		t.Fatalf("KnotInfoColumns: %v", err)
	}
	if len(cols) == 0 {
		t.Fatalf("no columns returned")
	}
	t.Logf("got %d columns", len(cols))

	// First column is the knot's short name.
	if cols[0] != "name" {
		t.Errorf("expected first column %q, got %q", "name", cols[0])
	}

	// A handful of well-known KnotInfo properties must be present.
	want := []string{"crossing_number", "jones_polynomial", "signature", "determinant"}
	have := make(map[string]bool, len(cols))
	for _, c := range cols {
		have[c] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("missing expected column %q", w)
		}
	}
}

// findCrossingColumn looks up the column whose name contains "crossing" (and
// prefers one that also mentions "number").
func findCrossingColumn(t *testing.T, db *sql.DB) string {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info("` + KnotInfoTable + `")`)
	if err != nil {
		t.Fatalf("table_info: %v", err)
	}
	defer rows.Close()

	var best, fallback string
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan pragma: %v", err)
		}
		lower := strings.ToLower(name)
		if strings.Contains(lower, "crossing") {
			if strings.Contains(lower, "number") && best == "" {
				best = name
			} else if fallback == "" {
				fallback = name
			}
		}
	}
	if best != "" {
		return best
	}
	if fallback != "" {
		return fallback
	}
	t.Fatalf("no column containing 'crossing' found in %s", KnotInfoTable)
	return ""
}
