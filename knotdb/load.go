package knotdb

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sxls "github.com/shakinm/xlsReader/xls"
)

// BuildKnotInfoJSON reads the KnotInfo xls spreadsheet at xlsPath and
// writes a zipped knot_info.json.zip into the current dataset directory
// (see SetDir). Returns the number of knot rows written.
//
// The JSON payload is a {"columns": [...], "knots": {name: {col: value}}}
// object. Empty values are omitted from each knot's map to keep the file
// small for sparse rows; callers get "" back from FindKnotRow for those.
// The JSON is then stored as the sole entry (knot_info.json) inside a
// zip archive — at ~7x compression this dramatically shrinks the file
// we ship and fetch over HTTP from wasm builds. The in-memory cache is
// invalidated so the next lookup reloads the freshly written file.
func BuildKnotInfoJSON(xlsPath string) (int, error) {
	headers, data, err := readKnotInfoXLS(xlsPath)
	if err != nil {
		return 0, err
	}
	knots, err := rowsToKnots(headers, data, headers)
	if err != nil {
		return 0, err
	}
	if err := writeKnotInfoZip(KnotInfoFile, KnotInfoJSONEntry, headers, knots); err != nil {
		return 0, err
	}
	Reset()
	return len(knots), nil
}

// BuildKnotInfoSmallJSON reads the KnotInfo xls spreadsheet at xlsPath
// and writes a zipped knot_info_small.json.zip into the current dataset
// directory (see SetDir). It has the same shape as knot_info.json.zip
// but restricts each row's column map to SmallColumns. Returns the
// number of knot rows written.
func BuildKnotInfoSmallJSON(xlsPath string) (int, error) {
	headers, data, err := readKnotInfoXLS(xlsPath)
	if err != nil {
		return 0, err
	}

	// Verify every small column actually exists in the xls schema so we
	// don't silently write out empty fields.
	have := make(map[string]bool, len(headers))
	for _, h := range headers {
		have[h] = true
	}
	for _, c := range SmallColumns {
		if !have[c] {
			return 0, fmt.Errorf("xls missing small column %q", c)
		}
	}

	small := make([]string, len(SmallColumns))
	copy(small, SmallColumns)

	knots, err := rowsToKnots(headers, data, small)
	if err != nil {
		return 0, err
	}
	if err := writeKnotInfoZip(KnotInfoSmallFile, KnotInfoSmallJSONEntry, small, knots); err != nil {
		return 0, err
	}
	Reset()
	return len(knots), nil
}

// rowsToKnots indexes xls rows by name, retaining only the columns in
// keep. Empty values are omitted. Returns an error if any knot name
// appears twice or if the headers don't include "name".
func rowsToKnots(headers []string, data [][]string, keep []string) (map[string]map[string]string, error) {
	nameCol := -1
	for i, h := range headers {
		if h == "name" {
			nameCol = i
			break
		}
	}
	if nameCol < 0 {
		return nil, fmt.Errorf("xls has no 'name' column")
	}

	// Precompute indices of columns we keep, in xls order.
	keepSet := make(map[string]bool, len(keep))
	for _, c := range keep {
		keepSet[c] = true
	}

	knots := make(map[string]map[string]string, len(data))
	for r, row := range data {
		name := ""
		if nameCol < len(row) {
			name = strings.TrimSpace(row[nameCol])
		}
		if name == "" {
			continue
		}
		if _, dup := knots[name]; dup {
			return nil, fmt.Errorf("duplicate knot name %q at xls row %d", name, r)
		}
		k := make(map[string]string, len(keep))
		for i, h := range headers {
			if !keepSet[h] {
				continue
			}
			var v string
			if i < len(row) {
				v = row[i]
			}
			if v != "" {
				k[h] = v
			}
		}
		knots[name] = k
	}
	return knots, nil
}

// writeKnotInfoZip marshals {columns, knots} to JSON, stores it in a
// zip archive as jsonEntry, and writes the archive to Dir()/zipName.
func writeKnotInfoZip(zipName, jsonEntry string, columns []string, knots map[string]map[string]string) error {
	outPath := filepath.Join(Dir(), zipName)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	raw, err := json.Marshal(knotInfo{Columns: columns, Knots: knots})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entry, err := zw.Create(jsonEntry)
	if err != nil {
		return fmt.Errorf("zip entry: %w", err)
	}
	if _, err := entry.Write(raw); err != nil {
		return fmt.Errorf("zip write: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("zip close: %w", err)
	}
	if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outPath, err)
	}
	return nil
}

// readKnotInfoXLS opens the xls file and extracts the schema + data rows.
//
// KnotInfo's spreadsheet uses two header rows:
//   - row 0: short ASCII code names for every column (used as JSON keys)
//   - row 1: human-readable labels in the even-indexed columns only
//
// Each property occupies two adjacent columns: the even-indexed column
// holds the property value, the odd-indexed column holds a
// description/note/link. We keep only the even-indexed columns of row 0
// as headers and of rows 2..N as data.
func readKnotInfoXLS(xlsPath string) ([]string, [][]string, error) {
	wb, err := sxls.OpenFile(xlsPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open xls %s: %w", xlsPath, err)
	}
	if wb.GetNumberSheets() == 0 {
		return nil, nil, fmt.Errorf("xls has no sheets")
	}
	sheet, err := wb.GetSheet(0)
	if err != nil {
		return nil, nil, fmt.Errorf("get sheet 0: %w", err)
	}

	headerRow, err := sheet.GetRow(0)
	if err != nil {
		return nil, nil, fmt.Errorf("read header row: %w", err)
	}
	headerCols := headerRow.GetCols()
	if len(headerCols) < 2 {
		return nil, nil, fmt.Errorf("header row has too few columns: %d", len(headerCols))
	}

	var pickIdx []int
	var headers []string
	seen := map[string]int{}
	for i := 0; i < len(headerCols); i += 2 {
		name := strings.TrimSpace(headerCols[i].GetString())
		if name == "" {
			continue
		}
		if n, ok := seen[name]; ok {
			seen[name] = n + 1
			name = fmt.Sprintf("%s_%d", name, n+1)
		} else {
			seen[name] = 1
		}
		pickIdx = append(pickIdx, i)
		headers = append(headers, name)
	}
	if len(headers) == 0 {
		return nil, nil, fmt.Errorf("no header columns found in row 0")
	}

	totalRows := sheet.GetNumberRows()
	data := make([][]string, 0, totalRows)
	for ri := 2; ri < totalRows; ri++ {
		row, err := sheet.GetRow(ri)
		if err != nil {
			return nil, nil, fmt.Errorf("read row %d: %w", ri, err)
		}
		cols := row.GetCols()
		if isEmptyRowCells(cols) {
			continue
		}
		dst := make([]string, len(pickIdx))
		for i, ci := range pickIdx {
			if ci < len(cols) {
				dst[i] = cols[ci].GetString()
			}
		}
		data = append(data, dst)
	}
	return headers, data, nil
}

type cellStringer interface {
	GetString() string
}

func isEmptyRowCells[T cellStringer](cells []T) bool {
	for _, c := range cells {
		if strings.TrimSpace(c.GetString()) != "" {
			return false
		}
	}
	return true
}
