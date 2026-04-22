package knotdb

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Image style identifiers. These double as the second argument to
// LoadImageBlob and as the stable names shared with the knot package.
const (
	StyleDiagram       = "diagram"
	StyleDiagramMirror = "diagram_mirror"
	StyleSnappy        = "snappy"
	StyleSnappyMirror  = "snappy_mirror"
	StyleGrid          = "grid"
)

// imagePath returns the on-disk path for the given knot name and style,
// relative to the dataset directory dir.
func imagePath(dir, name, style string) (string, error) {
	switch style {
	case StyleDiagram:
		return filepath.Join(dir, "diagrams", name+".png"), nil
	case StyleDiagramMirror:
		return filepath.Join(dir, "diagrams", name+"mirror.png"), nil
	case StyleSnappy:
		return filepath.Join(dir, "diagrams_snappy", "snappyKnot"+name+".png"), nil
	case StyleSnappyMirror:
		return filepath.Join(dir, "diagrams_snappy", "snappyMirrorKnot"+name+".png"), nil
	case StyleGrid:
		return filepath.Join(dir, "GridDiagramSVG_D", "grid"+name+".svg"), nil
	default:
		return "", fmt.Errorf("unknown image style %q", style)
	}
}

// LoadImageBlob reads the raw bytes of the named knot's image in the
// given style from the dataset directory. Returns (nil, nil) when the
// file does not exist (for example, the unknot has no diagrams).
func LoadImageBlob(name, style string) ([]byte, error) {
	path, err := imagePath(Dir(), name, style)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}
