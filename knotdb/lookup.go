package knotdb

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// ErrKnotNotFound is returned when a knot name is not present in knot_info.
var ErrKnotNotFound = errors.New("knot not found")

// EnsureIndexes creates indexes on knot_info.name and knot_img.name in the
// current sqlite database (see SetPath) if they do not already exist. Run
// this once after loading or whenever you want to make sure lookups by name
// are fast. Both indexes are UNIQUE since each knot name is a primary
// identifier.
func EnsureIndexes() error {
	h, err := db()
	if err != nil {
		return err
	}
	stmts := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_knot_info_name ON "` + KnotInfoTable + `" (name)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_knot_img_name  ON "` + KnotImgTable + `" (name)`,
	}
	for _, s := range stmts {
		if _, err := h.Exec(s); err != nil {
			// knot_img may not exist if images were never loaded; that's ok.
			if isMissingTableErr(err) {
				continue
			}
			return fmt.Errorf("ensure index: %w (%s)", err, s)
		}
	}
	return nil
}

func isMissingTableErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no such table")
}

// FindKnotRow returns the column names and string values of the knot_info
// row with the given name. NULL columns are returned as the empty string.
// Returns ErrKnotNotFound if no such row exists.
func FindKnotRow(name string) (cols []string, vals []string, err error) {
	h, err := db()
	if err != nil {
		return nil, nil, err
	}

	cols, err = knotInfoColumns(h)
	if err != nil {
		return nil, nil, err
	}

	var b strings.Builder
	b.WriteString("SELECT ")
	for i, c := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(quoteIdent(c))
	}
	b.WriteString(` FROM "`)
	b.WriteString(KnotInfoTable)
	b.WriteString(`" WHERE name = ? LIMIT 1`)

	row := h.QueryRow(b.String(), name)

	ns := make([]sql.NullString, len(cols))
	dest := make([]any, len(cols))
	for i := range ns {
		dest[i] = &ns[i]
	}
	if err := row.Scan(dest...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrKnotNotFound
		}
		return nil, nil, fmt.Errorf("scan: %w", err)
	}

	vals = make([]string, len(ns))
	for i, v := range ns {
		if v.Valid {
			vals[i] = v.String
		}
	}
	return cols, vals, nil
}

// RandomKnotName returns the name of a randomly chosen row from knot_info.
// Uses SQLite's built-in random() so the pick is uniform over the full
// table. Returns ErrKnotNotFound if knot_info is empty.
func RandomKnotName() (string, error) {
	h, err := db()
	if err != nil {
		return "", err
	}
	var name string
	err = h.QueryRow(`SELECT name FROM "` + KnotInfoTable + `" ORDER BY random() LIMIT 1`).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrKnotNotFound
	}
	if err != nil {
		return "", fmt.Errorf("random knot: %w", err)
	}
	return name, nil
}

// LoadImageBlob reads a single image column from knot_img for the named
// knot. Returns (nil, nil) if the row exists but the column is NULL or the
// row does not exist. Caller is responsible for column-name validation.
func LoadImageBlob(name, column string) ([]byte, error) {
	h, err := db()
	if err != nil {
		return nil, err
	}
	var data []byte
	err = h.QueryRow(`SELECT "`+column+`" FROM "`+KnotImgTable+`" WHERE name = ?`, name).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load %s for %s: %w", column, name, err)
	}
	return data, nil
}

// knotInfoColumns lists the knot_info columns from an open handle.
func knotInfoColumns(h *sql.DB) ([]string, error) {
	rows, err := h.Query(`SELECT name FROM pragma_table_info(?) ORDER BY cid`, KnotInfoTable)
	if err != nil {
		return nil, fmt.Errorf("table_info: %w", err)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		cols = append(cols, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("table %s has no columns", KnotInfoTable)
	}
	return cols, nil
}
