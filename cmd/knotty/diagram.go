package main

import (
	"image"
	"image/color"
)

// Endpoint is an arc's connection to a crossing: which crossing it meets
// and whether the arc is the over-strand (true) or under-strand (false)
// at that crossing.
type Endpoint struct {
	Crossing int
	Over     bool
}

// Arc is a polyline running between two crossings. Polyline[0] sits at
// Start.Crossing, Polyline[len-1] at End.Crossing. Color is the per-arc
// stroke color set via ToolColor; the zero value (alpha 0) means the
// renderer uses its default stroke.
type Arc struct {
	Polyline []image.Point
	Start    Endpoint
	End      Endpoint
	Color    color.NRGBA
}

// Diagram is the polyline-level knot diagram extracted from a raster image
// by convertImage. Crossings are 4-valent vertices in pixel coordinates;
// Arcs connect them. Loops carries free-floating closed curves (no
// crossings, no endpoints) — the way the unknot drawn as a plain
// circle is represented after R1 simplification removes a diagram's
// last crossing. Each loop is a closed polyline in pixel coords.
type Diagram struct {
	Crossings []image.Point
	Arcs      []Arc
	Loops     [][]image.Point
}
