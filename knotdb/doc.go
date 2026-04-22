// Package knotdb loads and indexes the knot dataset (knot_info.json +
// image files) from the repo's dataset directory, and exposes lookup by
// name along with access to diagrams and pre-computed invariants.
//
// The knot_info table is stored as JSON (knot_info.json) and loaded into
// an in-memory map on first access. Images are read directly from the
// filesystem by constructing a path from the knot name and style.
package knotdb
