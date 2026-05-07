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
	StyleDiagram ImageType = iota
	StyleDiagramMirror
	StyleSnappy
	StyleSnappyMirror
	StyleGrid
)

// ImageKind is the file format of an image: PNG for rasterized diagrams,
// SVG for the grid diagram, GIF for user-saved animated/static GIFs.
type ImageKind string

const (
	PNG ImageKind = "png"
	SVG ImageKind = "svg"
	GIF ImageKind = "gif"
)

// Column returns the knot_img column name that stores this ImageType.
func (t ImageType) Column() string {
	switch t {
	case StyleDiagram:
		return "diagram"
	case StyleDiagramMirror:
		return "diagram_mirror"
	case StyleSnappy:
		return "snappy"
	case StyleSnappyMirror:
		return "snappy_mirror"
	case StyleGrid:
		return "grid"
	}
	return ""
}

// Kind returns the file format of this image. All diagrams are PNG except
// Grid, which is SVG.
func (t ImageType) Kind() ImageKind {
	if t == StyleGrid {
		return SVG
	}
	return PNG
}

func (t ImageType) String() string {
	switch t {
	case StyleDiagram:
		return "Diagram"
	case StyleDiagramMirror:
		return "DiagramMirror"
	case StyleSnappy:
		return "Snappy"
	case StyleSnappyMirror:
		return "SnappyMirror"
	case StyleGrid:
		return "Grid"
	}
	return fmt.Sprintf("ImageType(%d)", int(t))
}

// LoadImage reads the raw bytes of this knot's image of type t from the
// current knot_img table (see knotdb.SetDir). Returns (nil, kind, nil)
// if the knot has no stored image of this type (for example, the unknot
// has no images).
func (d *Diagram) LoadImage(t ImageType) ([]byte, ImageKind, error) {
	col := t.Column()
	if col == "" {
		return nil, "", fmt.Errorf("unknown ImageType %d", int(t))
	}
	if d.name == "" {
		return nil, t.Kind(), fmt.Errorf("knot has no name; cannot load image")
	}
	data, err := knotdb.LoadImageBlob(d.name, col)
	if err != nil {
		return nil, t.Kind(), err
	}
	return data, t.Kind(), nil
}
