package knot

import (
	"reflect"
	"testing"
)

func TestColumnNames(t *testing.T) {
	cols := ColumnNames()
	if got := len(cols); got != 122 {
		t.Errorf("expected 122 columns, got %d", got)
	}
	if cols[0] != "name" {
		t.Errorf("first column: expected %q, got %q", "name", cols[0])
	}
	if cols[len(cols)-1] != "q_polynomial" {
		t.Errorf("last column: expected %q, got %q", "q_polynomial", cols[len(cols)-1])
	}
}

func TestNewFromRowAndRaw(t *testing.T) {
	cols := []string{"name", "category", "crossing_number", "jones_polynomial"}
	vals := []string{"3_1", "3", "3", "t+ t^3-t^4"}
	k, err := NewFromRow(cols, vals)
	if err != nil {
		t.Fatalf("NewFromRow: %v", err)
	}
	if got := k.GetName(); got != "3_1" {
		t.Errorf("GetName = %q", got)
	}
	if got := k.Raw("crossing_number"); got != "3" {
		t.Errorf("Raw(crossing_number) = %q", got)
	}
	if got := k.Raw("not_a_column"); got != "" {
		t.Errorf("Raw(unknown) = %q, want empty", got)
	}
}

func TestNewFromRowMismatchedLengths(t *testing.T) {
	if _, err := NewFromRow([]string{"name"}, []string{"a", "b"}); err == nil {
		t.Error("expected error on mismatched lengths")
	}
}

func TestIntFloatStringGetters(t *testing.T) {
	k, _ := NewFromRow(
		[]string{"name", "crossing_number", "signature", "determinant", "volume", "ropelength", "jones_polynomial", "alternating"},
		[]string{"4_1", "4", "0", "5", "2.0298832128", "42.0887", "t^(-2)-t^(-1)+ 1-t+ t^2", "Y"},
	)
	if got := k.GetCrossingNumber(); got != 4 {
		t.Errorf("GetCrossingNumber = %d", got)
	}
	if got := k.GetSignature(); got != 0 {
		t.Errorf("GetSignature = %d", got)
	}
	if got := k.GetDeterminant(); got != 5 {
		t.Errorf("GetDeterminant = %d", got)
	}
	if got := k.GetVolume(); got != 2.0298832128 {
		t.Errorf("GetVolume = %g", got)
	}
	if got := k.GetRopelength(); got != 42.0887 {
		t.Errorf("GetRopelength = %g", got)
	}
	if got := k.GetJonesPolynomial(); got != "t^(-2)-t^(-1)+ 1-t+ t^2" {
		t.Errorf("GetJonesPolynomial = %q", got)
	}
	if got := k.GetAlternating(); got != "Y" {
		t.Errorf("GetAlternating = %q", got)
	}
}

func TestIntGetterLenient(t *testing.T) {
	// Empty, unparseable, and range-like values all return 0.
	for _, s := range []string{"", " ", "[1,2]", "abc"} {
		k, _ := NewFromRow([]string{"crossing_number"}, []string{s})
		if got := k.GetCrossingNumber(); got != 0 {
			t.Errorf("lenient parse of %q: got %d, want 0", s, got)
		}
	}
}

func TestGetDTGaussNotation(t *testing.T) {
	k, _ := NewFromRow(
		[]string{"dt_notation", "gauss_notation"},
		[]string{"[4, 6, 2]", "[1, -2, 3, -1, 2, -3]"},
	)
	if got := k.GetDtNotation(); !reflect.DeepEqual(got, []int8{4, 6, 2}) {
		t.Errorf("GetDtNotation = %v", got)
	}
	if got := k.GetGaussNotation(); !reflect.DeepEqual(got, []int8{1, -2, 3, -1, 2, -3}) {
		t.Errorf("GetGaussNotation = %v", got)
	}
}

func TestGetPDNotation(t *testing.T) {
	k, _ := NewFromRow(
		[]string{"pd_notation"},
		[]string{"[[1,5,2,4],[3,1,4,6],[5,3,6,2]]"},
	)
	want := [][4]int8{{1, 5, 2, 4}, {3, 1, 4, 6}, {5, 3, 6, 2}}
	if got := k.GetPdNotation(); !reflect.DeepEqual(got, want) {
		t.Errorf("GetPdNotation = %v", got)
	}
}

func TestGetPDNotationEmpty(t *testing.T) {
	k, _ := NewFromRow([]string{"pd_notation"}, []string{""})
	if got := k.GetPdNotation(); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestImageTypeColumn(t *testing.T) {
	cases := []struct {
		t    ImageType
		col  string
		kind ImageKind
	}{
		{Diagram, "diagram", PNG},
		{DiagramMirror, "diagram_mirror", PNG},
		{Snappy, "snappy", PNG},
		{SnappyMirror, "snappy_mirror", PNG},
		{Grid, "grid", SVG},
	}
	for _, c := range cases {
		if got := c.t.Column(); got != c.col {
			t.Errorf("%s.Column() = %q, want %q", c.t, got, c.col)
		}
		if got := c.t.Kind(); got != c.kind {
			t.Errorf("%s.Kind() = %q, want %q", c.t, got, c.kind)
		}
	}
}
