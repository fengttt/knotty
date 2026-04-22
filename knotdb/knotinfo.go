package knotdb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// KnotInfoFile is the on-disk filename, under the dataset directory,
// of the JSON-encoded knot_info table.
const KnotInfoFile = "knot_info.json"

// knotInfo is the in-memory form of knot_info.json.
//
// The file pairs a column ordering with a map from knot name to that
// knot's property values. Columns omitted from a knot's map are treated
// as empty strings — this keeps the file small for sparse rows.
type knotInfo struct {
	Columns []string                     `json:"columns"`
	Knots   map[string]map[string]string `json:"knots"`

	// sortedNames is the set of Knots keys in lexicographic order,
	// precomputed for RandomKnotName and iteration helpers.
	sortedNames []string
}

// loadKnotInfoJSON reads dir/knot_info.json and builds an indexed knotInfo.
func loadKnotInfoJSON(dir string) (*knotInfo, error) {
	p := filepath.Join(dir, KnotInfoFile)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", p, err)
	}
	var ki knotInfo
	if err := json.Unmarshal(data, &ki); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	if len(ki.Columns) == 0 {
		return nil, fmt.Errorf("%s has no columns", p)
	}
	if len(ki.Knots) == 0 {
		return nil, fmt.Errorf("%s has no knots", p)
	}
	ki.sortedNames = make([]string, 0, len(ki.Knots))
	for n := range ki.Knots {
		ki.sortedNames = append(ki.sortedNames, n)
	}
	sort.Strings(ki.sortedNames)
	return &ki, nil
}
