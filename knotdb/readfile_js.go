//go:build js

package knotdb

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
)

// notFoundError wraps an HTTP 404 so the rest of knotdb can treat it the
// same as a missing file on a native filesystem.
type notFoundError struct{ path string }

func (e *notFoundError) Error() string { return "not found: " + e.path }

// readFile fetches path over HTTP. In js/wasm Go's http client is backed
// by the browser's fetch API, so a relative path like
// "dataset/knot_info.json" resolves against the page's base URL — which
// is typically what the user wants when serving the wasm bundle
// alongside the dataset.
func readFile(path string) ([]byte, error) {
	resp, err := http.Get(path)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, &notFoundError{path: path}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", path, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// isNotExist reports whether err indicates the requested path was absent
// (either a filesystem ENOENT or an HTTP 404).
func isNotExist(err error) bool {
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	var nf *notFoundError
	return errors.As(err, &nf)
}
