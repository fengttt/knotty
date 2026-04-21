# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project: Knotty

A tool for studying and playing with knots (knot theory). Written in Go. GUI/animation uses the Ebiten engine (`github.com/hajimehoshi/ebiten/v2`).

Status: early / greenfield. The repo currently contains only a stub README and `.gitignore` — no Go module, source, or tests exist yet. When bootstrapping, initialize a Go module and add source under the three-component layout below.

## Architecture (intended)

Knotty is organized around **three components**. Keep them separable — the algorithms and database layers must not depend on the GUI, so they remain usable as a library and from tests.

1. **Knot database** — a dataset directory holding a table of knots (Excel / CSV / text) plus images of knots. Code in this layer loads the dataset, indexes it, and exposes search/retrieval: look up a knot by name or notation, fetch its diagram, invariants, and any pre-rendered drawings. Treat the dataset directory as source data checked into the repo, not generated output.

2. **Knot algorithms** — a pure-Go library of operations on knots. Scope includes: converting between knot notations (e.g. Gauss code, DT code, PD notation, braid word), computing invariants and invariant polynomials (e.g. Alexander, Jones, HOMFLY, knot group), and rendering a knot diagram to SVG. This package should have no GUI dependency and should be the primary surface that tests exercise.

3. **GUI / game** — built on Ebiten. Displays a knot in different notations/formats and hosts an interactive game: the user picks a crossing or edge and performs operations (switch over/under at a crossing, drag an edge, move it around) to morph one diagram into another diagram of the same knot. The GUI consumes the algorithms package; equivalence checks during morphing should go through invariant computations from component 2.

## Conventions

- **Go** is the implementation language. Use standard `go build` / `go test ./...` once a module exists.
- **Ebiten** is the only approved GUI/animation engine for this project — do not introduce a second UI toolkit.
- Dataset files live in a top-level dataset directory and are read at runtime; do not hardcode knot data into Go source.
- Knot math is subtle: when adding a new invariant or notation converter, include test cases drawn from the dataset (known knots with known invariants) rather than only synthetic inputs.
