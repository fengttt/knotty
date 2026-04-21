package knotdb

import (
	"database/sql"
	"os"
	"sync"

	_ "modernc.org/sqlite"
)

// DefaultPath is the default sqlite database path, relative to the current
// working directory. Override with SetPath or the KNOTTY_DB_PATH env var.
const DefaultPath = "dataset/knotty.sqlite3"

var (
	pathMu  sync.Mutex
	curPath = resolveDefaultPath()

	dbMu sync.Mutex
	curDB *sql.DB
)

func resolveDefaultPath() string {
	if p := os.Getenv("KNOTTY_DB_PATH"); p != "" {
		return p
	}
	return DefaultPath
}

// SetPath redirects knotdb to the given sqlite file. If a handle is already
// open, it is closed so the next call reopens with the new path.
func SetPath(p string) {
	pathMu.Lock()
	if curPath == p {
		pathMu.Unlock()
		return
	}
	curPath = p
	pathMu.Unlock()

	_ = Close()
}

// Path returns the path knotdb will open on the next query.
func Path() string {
	pathMu.Lock()
	defer pathMu.Unlock()
	return curPath
}

// Close closes the underlying handle, if any. Safe to call multiple times.
// The next query will reopen lazily.
func Close() error {
	dbMu.Lock()
	defer dbMu.Unlock()
	if curDB == nil {
		return nil
	}
	err := curDB.Close()
	curDB = nil
	return err
}

// db returns the lazily-opened sqlite handle for the current path.
func db() (*sql.DB, error) {
	dbMu.Lock()
	defer dbMu.Unlock()
	if curDB != nil {
		return curDB, nil
	}
	pathMu.Lock()
	p := curPath
	pathMu.Unlock()

	dsn := p + "?_pragma=busy_timeout(10000)"
	h, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	curDB = h
	return curDB, nil
}
