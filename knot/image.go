package knot

import (
	"fmt"

	"github.com/fengttt/knotty/knotdb"
)

// ImageType identifies one of the five image views stored in the knot_img
// table: planar diagram, its mirror, the Snappy-rendered diagram, its
// mirror, and the grid-diagram SVG.
type ImageType int

const (
	Diagram ImageType = iota
	DiagramMirror
	Snappy
	SnappyMirror
	Grid
)

// ImageKind is the file format of an image: PNG for rasterized diagrams,
// SVG for the grid diagram.
type ImageKind string

const (
	PNG ImageKind = "png"
	SVG ImageKind = "svg"
)

// Column returns the knot_img column name that stores this ImageType.
func (t ImageType) Column() string {
	switch t {
	case Diagram:
		return "diagram"
	case DiagramMirror:
		return "diagram_mirror"
	case Snappy:
		return "snappy"
	case SnappyMirror:
		return "snappy_mirror"
	case Grid:
		return "grid"
	}
	return ""
}

// Kind returns the file format of this image. All diagrams are PNG except
// Grid, which is SVG.
func (t ImageType) Kind() ImageKind {
	if t == Grid {
		return SVG
	}
	return PNG
}

func (t ImageType) String() string {
	switch t {
	case Diagram:
		return "Diagram"
	case DiagramMirror:
		return "DiagramMirror"
	case Snappy:
		return "Snappy"
	case SnappyMirror:
		return "SnappyMirror"
	case Grid:
		return "Grid"
	}
	return fmt.Sprintf("ImageType(%d)", int(t))
}

// LoadImage reads the raw bytes of this knot's image of type t from the
// current knot_img table (see knotdb.SetPath). Returns (nil, kind, nil) if
// the knot has no stored image of this type (for example, the unknot has
// no images).
func (k *Knot) LoadImage(t ImageType) ([]byte, ImageKind, error) {
	col := t.Column()
	if col == "" {
		return nil, "", fmt.Errorf("unknown ImageType %d", int(t))
	}
	if k.name == "" {
		return nil, t.Kind(), fmt.Errorf("knot has no name; cannot load image")
	}
	data, err := knotdb.LoadImageBlob(k.name, col)
	if err != nil {
		return nil, t.Kind(), err
	}
	return data, t.Kind(), nil
}
