package main

import (
	"fmt"
	"image"
	"math"
	"sort"
)

// dartGraph is the crossing-level planar graph derived from a Diagram.
// Vertices are crossings (all 4-valent). At each crossing, adj holds the
// four incident darts in counter-clockwise order (on screen, y-down);
// adj[0] and adj[2] are on the under-strand, adj[1] and adj[3] are on
// the over-strand.
//
// A dart is a signed arc id: +(i+1) means arcs[i].Start is at this
// vertex; -(i+1) means arcs[i].End is at this vertex.
type dartGraph struct {
	diagram *Diagram
	adj     [][4]int
}

// tangentAngle estimates the outgoing direction of a polyline from its
// start, walking forward until the accumulated distance from the start
// point exceeds minDist. This filters out sharp bends near a crossing
// that would otherwise make the immediate next point a bad proxy for
// the strand's tangent direction. If the polyline ends before reaching
// minDist (e.g. self-loop arcs where both endpoints are the crossing),
// we fall back to the first polyline point at non-zero distance —
// that still distinguishes the two darts of a self-loop, whereas the
// farthest-point fallback collapses them to the same direction.
func tangentAngle(poly []image.Point, forward bool, minDist float64) float64 {
	var i0, step, end int
	if forward {
		i0, step, end = 0, 1, len(poly)
	} else {
		i0, step, end = len(poly)-1, -1, -1
	}
	p0 := poly[i0]
	firstNonZero := -1
	chosen := -1
	for i := i0 + step; i != end; i += step {
		dx := float64(poly[i].X - p0.X)
		dy := float64(poly[i].Y - p0.Y)
		d := math.Hypot(dx, dy)
		if d > 0 && firstNonZero < 0 {
			firstNonZero = i
		}
		if d >= minDist {
			chosen = i
			break
		}
	}
	if chosen < 0 {
		chosen = firstNonZero
	}
	if chosen < 0 {
		return 0
	}
	dx := float64(poly[chosen].X - p0.X)
	dy := float64(poly[chosen].Y - p0.Y)
	return math.Atan2(-dy, dx)
}

func newDartGraph(d *Diagram) (*dartGraph, error) {
	g := &dartGraph{
		diagram: d,
		adj:     make([][4]int, len(d.Crossings)),
	}
	type incident struct {
		dart  int
		angle float64
		over  bool
	}
	const tangentDist = 5.0
	bins := make([][]incident, len(d.Crossings))
	for ai, a := range d.Arcs {
		if len(a.Polyline) < 2 {
			return nil, fmt.Errorf("arc %d: polyline has %d points", ai, len(a.Polyline))
		}
		bins[a.Start.Crossing] = append(bins[a.Start.Crossing], incident{
			dart:  ai + 1,
			angle: tangentAngle(a.Polyline, true, tangentDist),
			over:  a.Start.Over,
		})
		bins[a.End.Crossing] = append(bins[a.End.Crossing], incident{
			dart:  -(ai + 1),
			angle: tangentAngle(a.Polyline, false, tangentDist),
			over:  a.End.Over,
		})
	}
	for v, incs := range bins {
		if len(incs) != 4 {
			return nil, fmt.Errorf("crossing %d: %d incident darts (want 4)", v, len(incs))
		}
		sort.SliceStable(incs, func(i, j int) bool {
			return incs[i].angle < incs[j].angle
		})
		if incs[0].over {
			incs = append(incs[1:], incs[0])
		}
		if incs[0].over || !incs[1].over || incs[2].over || !incs[3].over {
			return nil, fmt.Errorf(
				"crossing %d: over/under does not alternate around adj (%v,%v,%v,%v)",
				v, incs[0].over, incs[1].over, incs[2].over, incs[3].over)
		}
		g.adj[v] = [4]int{incs[0].dart, incs[1].dart, incs[2].dart, incs[3].dart}
	}
	return g, nil
}

// dartStart returns the crossing at which dart d resides in adj.
func (g *dartGraph) dartStart(d int) int {
	ai := absi(d) - 1
	if d > 0 {
		return g.diagram.Arcs[ai].Start.Crossing
	}
	return g.diagram.Arcs[ai].End.Crossing
}

// dartEnd returns the crossing at the far end of dart d.
func (g *dartGraph) dartEnd(d int) int {
	ai := absi(d) - 1
	if d > 0 {
		return g.diagram.Arcs[ai].End.Crossing
	}
	return g.diagram.Arcs[ai].Start.Crossing
}

// adjIndex returns the position of dart d in adj[dartStart(d)], or -1.
func (g *dartGraph) adjIndex(d int) int {
	v := g.dartStart(d)
	for i, dd := range g.adj[v] {
		if dd == d {
			return i
		}
	}
	return -1
}

// throughDart crosses the edge and continues on the same strand.
// At the far vertex v, it locates -d in adj[v] and returns the dart two
// CCW steps away (the diagonally-opposite same-strand dart).
func (g *dartGraph) throughDart(d int) int {
	v := g.dartEnd(d)
	for i, dd := range g.adj[v] {
		if dd == -d {
			return g.adj[v][(i+2)%4]
		}
	}
	return 0
}

// nextDart rotates CCW one step at dartStart(d).
func (g *dartGraph) nextDart(d int) int {
	i := g.adjIndex(d)
	if i < 0 {
		return 0
	}
	v := g.dartStart(d)
	return g.adj[v][(i+1)%4]
}

// prevDart rotates CW one step at dartStart(d).
func (g *dartGraph) prevDart(d int) int {
	i := g.adjIndex(d)
	if i < 0 {
		return 0
	}
	v := g.dartStart(d)
	return g.adj[v][(i+3)%4]
}

// dartIsOver reports whether d is on the over-strand at its crossing.
// Under-strand darts sit at adj positions 0 and 2; over-strand at 1 and 3.
func (g *dartGraph) dartIsOver(d int) bool {
	return g.adjIndex(d)%2 == 1
}

// dartCircuit returns the sequence of darts obtained by repeatedly
// applying throughDart until we cycle back to d.
func (g *dartGraph) dartCircuit(d int) []int {
	var path []int
	cur := d
	for {
		path = append(path, cur)
		cur = g.throughDart(cur)
		if cur == d {
			return path
		}
		if len(path) > 4*len(g.diagram.Arcs) {
			return nil
		}
	}
}

// NumComponents returns the number of link components.
func (g *dartGraph) NumComponents() int {
	seen := make([]bool, len(g.diagram.Arcs))
	n := 0
	for ai := range g.diagram.Arcs {
		if seen[ai] {
			continue
		}
		n++
		for _, d := range g.dartCircuit(ai + 1) {
			seen[absi(d)-1] = true
		}
	}
	return n
}

// PD returns the unoriented planar-diagram notation: one entry per
// crossing, each entry four arc ids in adj order. Arc ids are 1-based
// and labelled 1..2c by walking each component's circuit; arcs 0 and 2
// within an entry are on the under-strand, 1 and 3 on the over-strand.
func (g *dartGraph) PD() [][4]int {
	arcID := make([]int, len(g.diagram.Arcs))
	next := 1
	for ai := range g.diagram.Arcs {
		if arcID[ai] != 0 {
			continue
		}
		for _, d := range g.dartCircuit(ai + 1) {
			arcID[absi(d)-1] = next
			next++
		}
	}
	pd := make([][4]int, len(g.diagram.Crossings))
	for v := range g.diagram.Crossings {
		for i, dd := range g.adj[v] {
			pd[v][i] = arcID[absi(dd)-1]
		}
	}
	return pd
}

// DT returns a canonical Dowker-Thistlethwaite code, or nil if the
// diagram is not a single-component knot. For a knot with c crossings
// the code has length c; entries are signed even integers.
func (g *dartGraph) DT() []int {
	if g.NumComponents() != 1 {
		return nil
	}
	if len(g.diagram.Crossings) == 0 {
		return []int{}
	}

	codeFrom := func(circuit []int, k int) []int {
		n := len(circuit)
		pos := make(map[int]int, n)
		for i, d := range circuit {
			pos[d] = (i + n - k) % n
		}
		getPartner := func(d int) int {
			if p, ok := pos[g.nextDart(d)]; ok {
				return p
			}
			if p, ok := pos[g.prevDart(d)]; ok {
				return p
			}
			return -1
		}
		code := make([]int, 0, n/2)
		for i := 0; i < n; i += 2 {
			d := circuit[(i+k)%n]
			j := getPartner(d)
			if g.dartIsOver(d) {
				code = append(code, -j-1)
			} else {
				code = append(code, j+1)
			}
		}
		if len(code) > 0 && code[0] < 0 {
			for i := range code {
				code[i] = -code[i]
			}
		}
		return code
	}

	var codes [][]int
	for _, start := range []int{1, -1} {
		circuit := g.dartCircuit(start)
		for k := 0; k < len(circuit); k++ {
			codes = append(codes, codeFrom(circuit, k))
		}
	}
	sort.Slice(codes, func(i, j int) bool {
		return compareIntSlices(codes[i], codes[j]) < 0
	})
	return codes[0]
}

func compareIntSlices(a, b []int) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

// ---- Diagram convenience methods ----

func (d *Diagram) NumComponents() (int, error) {
	g, err := newDartGraph(d)
	if err != nil {
		return 0, err
	}
	return g.NumComponents(), nil
}

func (d *Diagram) PD() ([][4]int, error) {
	g, err := newDartGraph(d)
	if err != nil {
		return nil, err
	}
	return g.PD(), nil
}

func (d *Diagram) DT() ([]int, error) {
	g, err := newDartGraph(d)
	if err != nil {
		return nil, err
	}
	return g.DT(), nil
}
