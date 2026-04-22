package knotdb

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

const (
	testXlsRel     = "../dataset/knotinfo_data_complete.xls"
	testDatasetRel = "../dataset"
)

// useTestDir redirects knotdb to the test dataset directory for t.
func useTestDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(testDatasetRel)
	if err != nil {
		t.Fatalf("abs dir: %v", err)
	}
	prev := Dir()
	SetDir(dir)
	t.Cleanup(func() {
		SetDir(prev)
		Reset()
	})
	return dir
}

// TestBuildKnotInfoJSON generates knot_info.json.zip from the xls. Skips
// if the xls isn't present (as in CI without the dataset).
func TestBuildKnotInfoJSON(t *testing.T) {
	xlsPath, err := filepath.Abs(testXlsRel)
	if err != nil {
		t.Fatalf("abs xls: %v", err)
	}
	if _, err := os.Stat(xlsPath); err != nil {
		t.Skipf("xls not available at %s: %v", xlsPath, err)
	}
	dir := useTestDir(t)

	n, err := BuildKnotInfoJSON(xlsPath)
	if err != nil {
		t.Fatalf("BuildKnotInfoJSON: %v", err)
	}
	if n == 0 {
		t.Fatalf("BuildKnotInfoJSON wrote 0 knots")
	}
	t.Logf("wrote %d knots to %s/%s", n, dir, KnotInfoFile)

	outPath := filepath.Join(dir, KnotInfoFile)
	st, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat %s: %v", outPath, err)
	}
	if st.Size() == 0 {
		t.Fatalf("%s is empty", outPath)
	}
}

// TestBuildKnotInfoSmallJSON generates knot_info_small.json.zip from the
// xls. Skips if the xls isn't present. Verifies the archive carries the
// expected inner JSON shape with SmallColumns only.
func TestBuildKnotInfoSmallJSON(t *testing.T) {
	xlsPath, err := filepath.Abs(testXlsRel)
	if err != nil {
		t.Fatalf("abs xls: %v", err)
	}
	if _, err := os.Stat(xlsPath); err != nil {
		t.Skipf("xls not available at %s: %v", xlsPath, err)
	}
	dir := useTestDir(t)

	n, err := BuildKnotInfoSmallJSON(xlsPath)
	if err != nil {
		t.Fatalf("BuildKnotInfoSmallJSON: %v", err)
	}
	if n == 0 {
		t.Fatalf("BuildKnotInfoSmallJSON wrote 0 knots")
	}
	t.Logf("wrote %d knots to %s/%s", n, dir, KnotInfoSmallFile)

	outPath := filepath.Join(dir, KnotInfoSmallFile)
	st, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("stat %s: %v", outPath, err)
	}
	if st.Size() == 0 {
		t.Fatalf("%s is empty", outPath)
	}

	// Crack open the zip and verify the inner JSON has exactly SmallColumns.
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatalf("unzip: %v", err)
	}
	var entry *zip.File
	for _, f := range zr.File {
		if f.Name == KnotInfoSmallJSONEntry {
			entry = f
			break
		}
	}
	if entry == nil {
		t.Fatalf("no %q entry in zip", KnotInfoSmallJSONEntry)
	}
	rc, err := entry.Open()
	if err != nil {
		t.Fatalf("open entry: %v", err)
	}
	defer rc.Close()
	jsonBytes, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read entry: %v", err)
	}
	var ki struct {
		Columns []string                     `json:"columns"`
		Knots   map[string]map[string]string `json:"knots"`
	}
	if err := json.Unmarshal(jsonBytes, &ki); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(ki.Columns) != len(SmallColumns) {
		t.Errorf("columns len = %d, want %d", len(ki.Columns), len(SmallColumns))
	}
	for i, c := range SmallColumns {
		if i >= len(ki.Columns) || ki.Columns[i] != c {
			t.Errorf("columns[%d] = %q, want %q", i, ki.Columns[i], c)
		}
	}
	// Spot-check 4_1 has the expected fields.
	row, ok := ki.Knots["4_1"]
	if !ok {
		t.Fatalf("4_1 missing from small knots map")
	}
	for _, required := range []string{"crossing_number", "jones_polynomial", "pd_notation"} {
		if row[required] == "" {
			t.Errorf("4_1.%s is empty", required)
		}
	}
	// And that no unexpected columns leaked in.
	allowed := make(map[string]bool, len(SmallColumns))
	for _, c := range SmallColumns {
		allowed[c] = true
	}
	for k := range row {
		if !allowed[k] {
			t.Errorf("4_1 has unexpected column %q in small json", k)
		}
	}
}
