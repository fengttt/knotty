package main

import "image"

// Endpoint is an arc's connection to a crossing: which crossing it meets
// and whether the arc is the over-strand (true) or under-strand (false)
// at that crossing.
type Endpoint struct {
	Crossing int
	Over     bool
}

// Arc is a polyline running between two crossings. Polyline[0] sits at
// Start.Crossing, Polyline[len-1] at End.Crossing.
type Arc struct {
	Polyline []image.Point
	Start    Endpoint
	End      Endpoint
}

// Diagram is the polyline-level knot diagram extracted from a raster image
// by convertImage. Crossings are 4-valent vertices in pixel coordinates;
// Arcs connect them.
type Diagram struct {
	Crossings []image.Point
	Arcs      []Arc
}
