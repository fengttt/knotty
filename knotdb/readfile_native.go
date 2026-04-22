//go:build !js

package knotdb

import (
	"errors"
	"io/fs"
	"os"
)

// readFile reads a file from the local filesystem. This is the native
// (non-js) build; see readfile_js.go for the browser variant which
// fetches over HTTP.
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// isNotExist reports whether err indicates the requested path was absent.
func isNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}
