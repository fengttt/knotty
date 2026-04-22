// Package knot implements pure-Go operations on knots: notation
// conversions, invariant computations, and diagram rendering.
//
// Diagram holds the small set of knot_info properties that the app and
// the embedded wasm client work with. Use Get<Field>() to access a
// value converted to its natural Go type; use Raw to fetch the
// underlying string for any column by name.
package knot

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/fengttt/knotty/knotdb"
)

// columns is the set of Diagram property names in display order. It is
// the small knot_info schema (see knotdb.SmallColumns) plus the
// synthesized "component" field.
var columns = func() []string {
	out := make([]string, 0, len(knotdb.SmallColumns)+1)
	out = append(out, knotdb.SmallColumns...)
	out = append(out, "component")
	return out
}()

// ColumnNames returns the Diagram property names in declaration order.
// Callers can use this to iterate all fields in a stable order.
func ColumnNames() []string {
	out := make([]string, len(columns))
	copy(out, columns)
	return out
}

// Diagram is the small-schema view of a knot_info row. Raw spreadsheet
// strings are stored for the columns in knotdb.SmallColumns; component
// is synthesized and always 1 for knots loaded from knot_info.
type Diagram struct {
	name                  string
	crossingNumber        string
	unknottingNumber      string
	bridgeIndex           string
	symmetryType          string
	pdNotation            string
	dtNotation            string
	conwayNotation        string
	gaussNotation         string
	enhancedGaussNotation string
	jonesPolynomial       string

	// component is the number of connected components in the link. For
	// every knot in knot_info this is 1; the field is carried here so
	// code that iterates Diagram properties can render it uniformly.
	component int
}

// NewFromRow constructs a Diagram from a knot_info row. cols is the
// column name list; vals is the values in the same order. Any
// unrecognized column name is ignored; any unset field stays empty.
// Returns an error if cols and vals have different lengths. The
// component field is initialized to 1.
func NewFromRow(cols []string, vals []string) (*Diagram, error) {
	if len(cols) != len(vals) {
		return nil, fmt.Errorf("NewFromRow: len(cols)=%d != len(vals)=%d", len(cols), len(vals))
	}
	d := &Diagram{component: 1}
	for i, c := range cols {
		d.setRaw(c, vals[i])
	}
	return d, nil
}

func (d *Diagram) setRaw(col, v string) {
	switch col {
	case "name":
		d.name = v
	case "crossing_number":
		d.crossingNumber = v
	case "unknotting_number":
		d.unknottingNumber = v
	case "bridge_index":
		d.bridgeIndex = v
	case "symmetry_type":
		d.symmetryType = v
	case "pd_notation":
		d.pdNotation = v
	case "dt_notation":
		d.dtNotation = v
	case "conway_notation":
		d.conwayNotation = v
	case "gauss_notation":
		d.gaussNotation = v
	case "enhanced_gauss_notation":
		d.enhancedGaussNotation = v
	case "jones_polynomial":
		d.jonesPolynomial = v
	}
}

// Raw returns the raw string for the given property name, or "" if the
// column is unknown. For "component", returns the decimal-formatted
// value.
func (d *Diagram) Raw(col string) string {
	switch col {
	case "name":
		return d.name
	case "crossing_number":
		return d.crossingNumber
	case "unknotting_number":
		return d.unknottingNumber
	case "bridge_index":
		return d.bridgeIndex
	case "symmetry_type":
		return d.symmetryType
	case "pd_notation":
		return d.pdNotation
	case "dt_notation":
		return d.dtNotation
	case "conway_notation":
		return d.conwayNotation
	case "gauss_notation":
		return d.gaussNotation
	case "enhanced_gauss_notation":
		return d.enhancedGaussNotation
	case "jones_polynomial":
		return d.jonesPolynomial
	case "component":
		return strconv.Itoa(d.component)
	}
	return ""
}

// ----- integer getters -----

func (d *Diagram) GetCrossingNumber() int   { return parseIntLenient(d.crossingNumber) }
func (d *Diagram) GetUnknottingNumber() int { return parseIntLenient(d.unknottingNumber) }
func (d *Diagram) GetBridgeIndex() int      { return parseIntLenient(d.bridgeIndex) }
func (d *Diagram) GetComponent() int        { return d.component }

// ----- []int8 notation getters -----

func (d *Diagram) GetDtNotation() []int8            { return parseInt8List(d.dtNotation) }
func (d *Diagram) GetGaussNotation() []int8         { return parseInt8List(d.gaussNotation) }
func (d *Diagram) GetEnhancedGaussNotation() []int8 { return parseInt8List(d.enhancedGaussNotation) }

// ----- [][4]int8 notation getters -----

func (d *Diagram) GetPdNotation() [][4]int8 { return parseInt8Tuples4(d.pdNotation) }

// ----- string getters -----

func (d *Diagram) GetName() string            { return d.name }
func (d *Diagram) GetSymmetryType() string    { return d.symmetryType }
func (d *Diagram) GetConwayNotation() string  { return d.conwayNotation }
func (d *Diagram) GetJonesPolynomial() string { return d.jonesPolynomial }

// parseIntLenient parses a decimal integer, returning 0 for empty or
// unparseable input. KnotInfo sometimes stores ranges (e.g. "[1,2]") for
// uncertain values; those parse as 0 here — fall back to Raw() if the
// exact string matters.
func parseIntLenient(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// parseInt8List parses a flat list of signed integers formatted as
// "[a, b, c, ...]" into a []int8. Returns nil for empty input.
func parseInt8List(s string) []int8 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]int8, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.ParseInt(p, 10, 8)
		if err != nil {
			return nil
		}
		out = append(out, int8(n))
	}
	return out
}

// parseInt8Tuples4 parses a list of 4-tuples formatted as
// "[[a,b,c,d],[e,f,g,h],...]" into a [][4]int8. Returns nil for empty input
// or when any tuple does not have length 4.
func parseInt8Tuples4(s string) [][4]int8 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out [][4]int8
	depth := 0
	start := -1
	for i, r := range s {
		switch r {
		case '[':
			if depth == 0 {
				start = i + 1
			}
			depth++
		case ']':
			depth--
			if depth == 0 && start >= 0 {
				tuple := s[start:i]
				parts := strings.Split(tuple, ",")
				if len(parts) != 4 {
					return nil
				}
				var t [4]int8
				for j, p := range parts {
					p = strings.TrimSpace(p)
					n, err := strconv.ParseInt(p, 10, 8)
					if err != nil {
						return nil
					}
					t[j] = int8(n)
				}
				out = append(out, t)
				start = -1
			}
		}
	}
	if depth != 0 {
		return nil
	}
	return out
}
