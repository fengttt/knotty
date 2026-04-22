package knotdb

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
)

// KnotInfoFile is the on-disk filename, under the dataset directory, of
// the zipped JSON-encoded full knot_info table. The zip wraps a single
// entry named KnotInfoJSONEntry.
//
// KnotInfoSmallFile is the small companion file: the same shape, but
// with only the columns listed in SmallColumns. It's what the app and
// wasm client fetch by default; the full file is kept for callers that
// need every knot_info property.
const (
	KnotInfoFile           = "knot_info.json.zip"
	KnotInfoJSONEntry      = "knot_info.json"
	KnotInfoSmallFile      = "knot_info_small.json.zip"
	KnotInfoSmallJSONEntry = "knot_info_small.json"
)

// SmallColumns lists the knot_info columns exposed by the small lookup.
// Keep this list stable: it is the schema returned by KnotInfoColumns and
// the set of fields persisted in KnotInfoSmallFile.
var SmallColumns = []string{
	"name",
	"crossing_number",
	"unknotting_number",
	"bridge_index",
	"symmetry_type",
	"pd_notation",
	"dt_notation",
	"conway_notation",
	"gauss_notation",
	"enhanced_gauss_notation",
	"jones_polynomial",
}

// knotInfo is the in-memory form of a zipped knot_info JSON (full or small).
//
// The inner JSON pairs a column ordering with a map from knot name to
// that knot's property values. Columns omitted from a knot's map are
// treated as empty strings — this keeps the file small for sparse rows.
type knotInfo struct {
	Columns []string                     `json:"columns"`
	Knots   map[string]map[string]string `json:"knots"`

	// sortedNames is the set of Knots keys in lexicographic order,
	// precomputed for RandomKnotName and iteration helpers.
	sortedNames []string
}

// loadKnotInfoJSON reads dir/knot_info.json.zip, unzips the inner
// knot_info.json in memory, and returns the indexed knotInfo.
func loadKnotInfoJSON(dir string) (*knotInfo, error) {
	return loadKnotInfoZip(dir, KnotInfoFile, KnotInfoJSONEntry)
}

// loadKnotInfoSmallJSON reads dir/knot_info_small.json.zip and returns
// the indexed small knotInfo.
func loadKnotInfoSmallJSON(dir string) (*knotInfo, error) {
	return loadKnotInfoZip(dir, KnotInfoSmallFile, KnotInfoSmallJSONEntry)
}

// loadKnotInfoZip reads dir/zipName, extracts the inner jsonEntry, and
// returns the indexed knotInfo.
func loadKnotInfoZip(dir, zipName, jsonEntry string) (*knotInfo, error) {
	p := filepath.Join(dir, zipName)
	zipped, err := readFile(p)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", p, err)
	}

	zr, err := zip.NewReader(bytes.NewReader(zipped), int64(len(zipped)))
	if err != nil {
		return nil, fmt.Errorf("unzip %s: %w", p, err)
	}

	var entry *zip.File
	for _, f := range zr.File {
		if f.Name == jsonEntry {
			entry = f
			break
		}
	}
	if entry == nil {
		return nil, fmt.Errorf("%s: no %q entry", p, jsonEntry)
	}
	rc, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("open %s in %s: %w", entry.Name, p, err)
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return nil, fmt.Errorf("read %s in %s: %w", entry.Name, p, err)
	}

	var ki knotInfo
	if err := json.Unmarshal(data, &ki); err != nil {
		return nil, fmt.Errorf("parse %s: %w", entry.Name, err)
	}
	if len(ki.Columns) == 0 {
		return nil, fmt.Errorf("%s has no columns", entry.Name)
	}
	if len(ki.Knots) == 0 {
		return nil, fmt.Errorf("%s has no knots", entry.Name)
	}
	ki.sortedNames = make([]string, 0, len(ki.Knots))
	for n := range ki.Knots {
		ki.sortedNames = append(ki.sortedNames, n)
	}
	sort.Strings(ki.sortedNames)
	return &ki, nil
}
