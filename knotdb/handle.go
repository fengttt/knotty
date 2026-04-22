package knotdb

import (
	"os"
	"sync"
)

// DefaultDir is the default dataset directory, relative to the current
// working directory. Override with SetDir or the KNOTTY_DATASET_DIR env var.
const DefaultDir = "dataset"

var (
	dirMu  sync.Mutex
	curDir = resolveDefaultDir()

	infoMu       sync.Mutex
	curInfo      *knotInfo
	curInfoSmall *knotInfo
)

func resolveDefaultDir() string {
	if p := os.Getenv("KNOTTY_DATASET_DIR"); p != "" {
		return p
	}
	return DefaultDir
}

// SetDir redirects knotdb to the given dataset directory. If an in-memory
// knot_info cache is populated, it is cleared so the next query reloads
// from the new directory.
func SetDir(p string) {
	dirMu.Lock()
	changed := curDir != p
	curDir = p
	dirMu.Unlock()

	if changed {
		Reset()
	}
}

// Dir returns the dataset directory knotdb will read on the next query.
func Dir() string {
	dirMu.Lock()
	defer dirMu.Unlock()
	return curDir
}

// Reset clears the in-memory knot_info caches, if any. The next lookup
// will reload from disk.
func Reset() {
	infoMu.Lock()
	defer infoMu.Unlock()
	curInfo = nil
	curInfoSmall = nil
}

// info returns the lazily-loaded full knot_info for the current dataset dir.
func info() (*knotInfo, error) {
	infoMu.Lock()
	defer infoMu.Unlock()
	if curInfo != nil {
		return curInfo, nil
	}
	ki, err := loadKnotInfoJSON(Dir())
	if err != nil {
		return nil, err
	}
	curInfo = ki
	return ki, nil
}

// infoSmall returns the lazily-loaded small knot_info for the current
// dataset dir.
func infoSmall() (*knotInfo, error) {
	infoMu.Lock()
	defer infoMu.Unlock()
	if curInfoSmall != nil {
		return curInfoSmall, nil
	}
	ki, err := loadKnotInfoSmallJSON(Dir())
	if err != nil {
		return nil, err
	}
	curInfoSmall = ki
	return ki, nil
}
