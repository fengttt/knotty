package knotdb

import (
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

// TestBuildKnotInfoJSON generates knot_info.json from the xls. Skips if
// the xls isn't present (as in CI without the dataset).
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
