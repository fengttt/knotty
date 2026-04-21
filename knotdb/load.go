package knotdb

import (
	"fmt"
	"strings"

	sxls "github.com/shakinm/xlsReader/xls"
	_ "modernc.org/sqlite"
)

// KnotInfoTable is the name of the table populated by LoadKnotInfo.
const KnotInfoTable = "knot_info"

// LoadKnotInfo reads the KnotInfo xls spreadsheet at xlsPath, drops and
// recreates the knot_info table in the current sqlite database (see SetPath),
// and loads every data row into it.
//
// The spreadsheet pairs each property with a description/note column; only the
// first column of each pair is loaded, using the spreadsheet's header text as
// the SQL column name. All columns are TEXT. Returns the number of data rows
// inserted.
func LoadKnotInfo(xlsPath string) (int, error) {
	headers, data, err := readKnotInfoXLS(xlsPath)
	if err != nil {
		return 0, err
	}

	h, err := db()
	if err != nil {
		return 0, err
	}

	if _, err := h.Exec("DROP TABLE IF EXISTS " + KnotInfoTable); err != nil {
		return 0, fmt.Errorf("drop table: %w", err)
	}
	if _, err := h.Exec(createTableStmt(KnotInfoTable, headers)); err != nil {
		return 0, fmt.Errorf("create table: %w", err)
	}

	tx, err := h.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare(insertStmt(KnotInfoTable, headers))
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	defer stmt.Close()

	args := make([]any, len(headers))
	inserted := 0
	for r, row := range data {
		for i := range args {
			if i < len(row) {
				args[i] = row[i]
			} else {
				args[i] = ""
			}
		}
		if _, err := stmt.Exec(args...); err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("insert row %d: %w", r, err)
		}
		inserted++
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}

// readKnotInfoXLS opens the xls file and extracts the schema + data rows.
//
// KnotInfo's spreadsheet uses two header rows:
//   - row 0: short ASCII code names for every column (used as SQL identifiers)
//   - row 1: human-readable labels in the even-indexed columns only
//
// Each property occupies two adjacent columns: the even-indexed column holds
// the property value, the odd-indexed column holds a description/note/link.
// We keep only the even-indexed columns of row 0 as headers and of rows 2..N
// as data.
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

	// Data begins at row 2: row 0 = codes, row 1 = human-readable labels.
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

func createTableStmt(table string, headers []string) string {
	var b strings.Builder
	b.WriteString(`CREATE TABLE "`)
	b.WriteString(table)
	b.WriteString(`" (`)
	for i, h := range headers {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(quoteIdent(h))
		b.WriteString(" TEXT")
	}
	b.WriteString(")")
	return b.String()
}

func insertStmt(table string, headers []string) string {
	var b strings.Builder
	b.WriteString(`INSERT INTO "`)
	b.WriteString(table)
	b.WriteString(`" (`)
	for i, h := range headers {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(quoteIdent(h))
	}
	b.WriteString(") VALUES (")
	for i := range headers {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString("?")
	}
	b.WriteString(")")
	return b.String()
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// KnotInfoColumns returns the column names of the knot_info table in the
// current sqlite database (see SetPath), in declaration order.
func KnotInfoColumns() ([]string, error) {
	h, err := db()
	if err != nil {
		return nil, err
	}
	rows, err := h.Query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, KnotInfoTable)
	if err != nil {
		return nil, fmt.Errorf("table_info: %w", err)
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		cols = append(cols, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("table %s has no columns (does it exist?)", KnotInfoTable)
	}
	return cols, nil
}
