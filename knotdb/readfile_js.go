//go:build js

package knotdb

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
)

// WasmBaseURL is the HTTPS origin the wasm client fetches dataset files
// from. Paths passed to readFile are rewritten to <WasmBaseURL>/<rel>,
// where rel is the path with the leading dataset directory stripped —
// so a native lookup like "dataset/knot_info_small.json.zip" becomes
// https://.../knotty/knot_info_small.json.zip in the browser.
const WasmBaseURL = "https://fengttt01.s3.us-east-2.amazonaws.com/knotty/"

// notFoundError wraps an HTTP 404 so the rest of knotdb can treat it the
// same as a missing file on a native filesystem.
type notFoundError struct{ url string }

func (e *notFoundError) Error() string { return "not found: " + e.url }

// readFile fetches path from WasmBaseURL. In js/wasm Go's http client is
// backed by the browser's fetch API. The incoming path has the form
// "<dataset_dir>/..." (see Dir()); we strip that prefix so the S3 object
// key matches what the loader expects.
func readFile(path string) ([]byte, error) {
	rel := path
	if dir := Dir(); dir != "" {
		rel = strings.TrimPrefix(rel, dir+"/")
	}
	url := WasmBaseURL + rel
	resp, err := http.Get(url)
	if err != nil {
		// A cross-origin fetch that lacks Access-Control-Allow-Origin
		// surfaces here as a generic fetch error, not an HTTP status.
		// Make that case diagnosable rather than letting it look like
		// a missing file.
		return nil, fmt.Errorf("fetch %s (check S3 CORS if this is a cross-origin request): %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, &notFoundError{url: url}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// isNotExist reports whether err indicates the requested object was
// absent (either a filesystem ENOENT or an HTTP 404).
func isNotExist(err error) bool {
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	var nf *notFoundError
	return errors.As(err, &nf)
}
