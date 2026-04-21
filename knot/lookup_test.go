package knot

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/fengttt/knotty/knotdb"
	_ "modernc.org/sqlite"
)

const testDbRel = "../dataset/knotty.sqlite3"

// useTestDB redirects knotdb to the test sqlite file for the duration of t.
func useTestDB(t *testing.T) string {
	t.Helper()
	dbPath, err := filepath.Abs(testDbRel)
	if err != nil {
		t.Fatalf("abs db: %v", err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("db not loaded yet at %s: %v", dbPath, err)
	}
	prev := knotdb.Path()
	knotdb.SetPath(dbPath)
	t.Cleanup(func() {
		_ = knotdb.Close()
		knotdb.SetPath(prev)
	})
	return dbPath
}

func TestFindKnotByName(t *testing.T) {
	useTestDB(t)

	k, err := FindKnotByName("4_1")
	if err != nil {
		t.Fatalf("FindKnotByName(4_1): %v", err)
	}
	if got := k.GetName(); got != "4_1" {
		t.Errorf("GetName = %q", got)
	}
	if got := k.GetCrossingNumber(); got != 4 {
		t.Errorf("crossing_number = %d", got)
	}
	if got := k.GetSignature(); got != 0 {
		t.Errorf("signature = %d (want 0 for 4_1)", got)
	}
	if got := k.GetDeterminant(); got != 5 {
		t.Errorf("determinant = %d (want 5)", got)
	}
	if got := k.GetAlternating(); got != "Y" {
		t.Errorf("alternating = %q", got)
	}
	if got := k.GetJonesPolynomial(); got == "" {
		t.Error("jones_polynomial should be non-empty for 4_1")
	}
	if pd := k.GetPdNotation(); len(pd) == 0 {
		t.Error("pd_notation should be non-empty for 4_1")
	}
}

func TestFindKnotByNameTrefoil(t *testing.T) {
	useTestDB(t)

	k, err := FindKnotByName("3_1")
	if err != nil {
		t.Fatalf("FindKnotByName(3_1): %v", err)
	}
	if got := k.GetCrossingNumber(); got != 3 {
		t.Errorf("crossing_number = %d", got)
	}
	if got := k.GetSignature(); got != -2 {
		t.Errorf("signature = %d", got)
	}
	if got := k.GetDeterminant(); got != 3 {
		t.Errorf("determinant = %d", got)
	}
	want := []int8{1, -2, 3, -1, 2, -3}
	if got := k.GetGaussNotation(); !equalInt8(got, want) {
		t.Errorf("gauss_notation = %v, want %v", got, want)
	}
}

func TestFindKnotByNameMissing(t *testing.T) {
	useTestDB(t)
	_, err := FindKnotByName("this_is_not_a_knot")
	if !errors.Is(err, knotdb.ErrKnotNotFound) {
		t.Errorf("expected ErrKnotNotFound, got %v", err)
	}
}

func TestLoadImage(t *testing.T) {
	useTestDB(t)

	// 0_1 (unknot) has no images -- returns (nil, kind, nil).
	unknot, err := FindKnotByName("0_1")
	if err != nil {
		t.Fatalf("FindKnotByName(0_1): %v", err)
	}
	data, kind, err := unknot.LoadImage(Diagram)
	if err != nil {
		t.Fatalf("LoadImage(0_1, Diagram): %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected no data for 0_1 diagram, got %d bytes", len(data))
	}
	if kind != PNG {
		t.Errorf("kind = %q, want %q", kind, PNG)
	}

	tre, err := FindKnotByName("3_1")
	if err != nil {
		t.Fatalf("FindKnotByName(3_1): %v", err)
	}
	for _, ty := range []ImageType{Diagram, DiagramMirror, Snappy, SnappyMirror, Grid} {
		data, kind, err := tre.LoadImage(ty)
		if err != nil {
			t.Fatalf("LoadImage(3_1, %s): %v", ty, err)
		}
		if len(data) == 0 {
			t.Errorf("3_1 %s: got 0 bytes", ty)
		}
		wantKind := PNG
		var wantMagic []byte
		if ty == Grid {
			wantKind = SVG
			wantMagic = []byte("<?xml")
		} else {
			wantMagic = []byte{0x89, 'P', 'N', 'G'}
		}
		if kind != wantKind {
			t.Errorf("3_1 %s: kind = %q, want %q", ty, kind, wantKind)
		}
		if len(data) < len(wantMagic) {
			continue
		}
		for i, b := range wantMagic {
			if data[i] != b {
				if ty == Grid {
					break
				}
				t.Errorf("3_1 %s: byte %d = %#x, want %#x", ty, i, data[i], b)
				break
			}
		}
	}
}

func equalInt8(a, b []int8) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
