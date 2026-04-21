package knotdb

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// KnotImgTable is the name of the table populated by LoadKnotImages.
const KnotImgTable = "knot_img"

// MaxImageCrossing is the upper bound (inclusive) on crossing number for
// which images are loaded.
const MaxImageCrossing = 12

// imageLayout describes where to find each kind of image for a knot named
// ${name}, relative to a dataset root.
type imageLayout struct {
	datasetDir string
}

func (l imageLayout) diagramPath(name string) string {
	return filepath.Join(l.datasetDir, "diagrams", name+".png")
}
func (l imageLayout) diagramMirrorPath(name string) string {
	return filepath.Join(l.datasetDir, "diagrams", name+"mirror.png")
}
func (l imageLayout) snappyPath(name string) string {
	return filepath.Join(l.datasetDir, "diagrams_snappy", "snappyKnot"+name+".png")
}
func (l imageLayout) snappyMirrorPath(name string) string {
	return filepath.Join(l.datasetDir, "diagrams_snappy", "snappyMirrorKnot"+name+".png")
}
func (l imageLayout) gridPath(name string) string {
	return filepath.Join(l.datasetDir, "GridDiagramSVG_D", "grid"+name+".svg")
}

// LoadKnotImages iterates the knot_info table for knots with crossing number
// <= MaxImageCrossing, reads each knot's five image files from datasetDir,
// and loads them into knot_img. The table is dropped and recreated. Missing
// files are stored as NULL. Returns the number of rows inserted.
func LoadKnotImages(datasetDir, dbPath string) (int, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return 0, fmt.Errorf("open sqlite %s: %w", dbPath, err)
	}
	defer db.Close()

	names, err := selectKnotNamesUpTo(db, MaxImageCrossing)
	if err != nil {
		return 0, err
	}

	if _, err := db.Exec("DROP TABLE IF EXISTS " + KnotImgTable); err != nil {
		return 0, fmt.Errorf("drop table: %w", err)
	}
	createSQL := `CREATE TABLE "` + KnotImgTable + `" (
		name TEXT PRIMARY KEY,
		diagram BLOB,
		diagram_mirror BLOB,
		snappy BLOB,
		snappy_mirror BLOB,
		grid BLOB
	)`
	if _, err := db.Exec(createSQL); err != nil {
		return 0, fmt.Errorf("create table: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	stmt, err := tx.Prepare(`INSERT INTO "` + KnotImgTable +
		`" (name, diagram, diagram_mirror, snappy, snappy_mirror, grid) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	defer stmt.Close()

	layout := imageLayout{datasetDir: datasetDir}
	inserted := 0
	for _, name := range names {
		diagram, err := readFileMaybe(layout.diagramPath(name))
		if err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		diagramMirror, err := readFileMaybe(layout.diagramMirrorPath(name))
		if err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		snappy, err := readFileMaybe(layout.snappyPath(name))
		if err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		snappyMirror, err := readFileMaybe(layout.snappyMirrorPath(name))
		if err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		grid, err := readFileMaybe(layout.gridPath(name))
		if err != nil {
			_ = tx.Rollback()
			return 0, err
		}

		if _, err := stmt.Exec(name,
			nullableBlob(diagram),
			nullableBlob(diagramMirror),
			nullableBlob(snappy),
			nullableBlob(snappyMirror),
			nullableBlob(grid),
		); err != nil {
			_ = tx.Rollback()
			return 0, fmt.Errorf("insert %s: %w", name, err)
		}
		inserted++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return inserted, nil
}

// selectKnotNamesUpTo returns names of knots with crossing number <= max,
// in knot_info insertion order.
func selectKnotNamesUpTo(db *sql.DB, max int) ([]string, error) {
	rows, err := db.Query(`SELECT name, crossing_number FROM "` + KnotInfoTable + `"`)
	if err != nil {
		return nil, fmt.Errorf("select knot names: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name, crossing string
		if err := rows.Scan(&name, &crossing); err != nil {
			return nil, err
		}
		cn, err := strconv.Atoi(strings.TrimSpace(crossing))
		if err != nil {
			// Skip unparseable crossing numbers rather than failing.
			continue
		}
		if cn <= max {
			names = append(names, name)
		}
	}
	return names, rows.Err()
}

// readFileMaybe returns file contents, or (nil, nil) if the file does not
// exist. Other errors are returned.
func readFileMaybe(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

// nullableBlob returns nil (stored as SQL NULL) when the slice is empty,
// so the column stays NULL rather than an empty BLOB.
func nullableBlob(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}
