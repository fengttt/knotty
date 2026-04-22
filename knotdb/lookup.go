package knotdb

import (
	"errors"
	"math/rand/v2"
)

// ErrKnotNotFound is returned when a knot name is not present in
// knot_info.json.
var ErrKnotNotFound = errors.New("knot not found")

// FindKnotRow returns the column names and string values for the knot
// with the given name. Columns absent from the JSON row are returned as
// empty strings. Returns ErrKnotNotFound if no such knot exists.
func FindKnotRow(name string) (cols []string, vals []string, err error) {
	ki, err := info()
	if err != nil {
		return nil, nil, err
	}
	row, ok := ki.Knots[name]
	if !ok {
		return nil, nil, ErrKnotNotFound
	}
	cols = make([]string, len(ki.Columns))
	copy(cols, ki.Columns)
	vals = make([]string, len(ki.Columns))
	for i, c := range ki.Columns {
		vals[i] = row[c]
	}
	return cols, vals, nil
}

// RandomKnotName returns the name of a randomly chosen knot. Returns
// ErrKnotNotFound if knot_info.json is empty.
func RandomKnotName() (string, error) {
	ki, err := info()
	if err != nil {
		return "", err
	}
	if len(ki.sortedNames) == 0 {
		return "", ErrKnotNotFound
	}
	return ki.sortedNames[rand.IntN(len(ki.sortedNames))], nil
}

// KnotInfoColumns returns the knot_info column names in declaration order.
func KnotInfoColumns() ([]string, error) {
	ki, err := info()
	if err != nil {
		return nil, err
	}
	out := make([]string, len(ki.Columns))
	copy(out, ki.Columns)
	return out, nil
}
