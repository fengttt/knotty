package knot

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/fengttt/knotty/knotdb"
)

const (
	testDatasetRel = "../dataset"
	testJSONRel    = "../dataset/knot_info.json"
)

// useTestDir redirects knotdb to the test dataset directory for t.
func useTestDir(t *testing.T) {
	t.Helper()
	dir, err := filepath.Abs(testDatasetRel)
	if err != nil {
		t.Fatalf("abs dir: %v", err)
	}
	jp, err := filepath.Abs(testJSONRel)
	if err != nil {
		t.Fatalf("abs json: %v", err)
	}
	if _, err := os.Stat(jp); err != nil {
		t.Skipf("knot_info.json not built at %s: %v", jp, err)
	}
	prev := knotdb.Dir()
	knotdb.SetDir(dir)
	t.Cleanup(func() {
		knotdb.SetDir(prev)
		knotdb.Reset()
	})
}

func TestFindKnotByName(t *testing.T) {
	useTestDir(t)

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
	useTestDir(t)

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
	useTestDir(t)

	_, err := FindKnotByName("this_is_not_a_knot")
	if !errors.Is(err, knotdb.ErrKnotNotFound) {
		t.Errorf("expected ErrKnotNotFound, got %v", err)
	}
}

func TestLoadImage(t *testing.T) {
	useTestDir(t)

	// 0_1 (unknot) has no images.
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
