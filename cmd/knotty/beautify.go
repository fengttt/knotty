package main

import (
	"fmt"
	"image"
	"log"
	"math"
)

// Beautify recomputes crossing positions and arc polylines via the Tutte
// embedding of the medial-barycentric subdivision of the underlying planar
// graph. The result is a topology-equivalent Diagram with rounder, more
// regular geometry. canvasW and canvasH bound the output rectangle; the
// beautified diagram is placed centered with a small margin.
//
// Implementation tracks a planar graph through three medial subdivisions,
// finds the largest "fvv"-typed vertex (= centroid of an original face) as
// the outer face, pins its perimeter on a regular polygon, and solves the
// barycentric (Tutte) linear system for the remaining vertex positions.
func (d *Diagram) Beautify(canvasW, canvasH int) (*Diagram, error) {
	orig := d
	if len(d.Crossings) == 0 {
		return d, nil
	}
	g, err := newDartGraph(d)
	if err != nil {
		return nil, err
	}

	bg := newBgraphFromDart(g, len(d.Crossings))
	bg0 := bg

	// Track each arc as an alternating sequence of vertex IDs and the dart
	// connecting consecutive verts. Initially: [startCrossing, endCrossing]
	// linked by the original dart +(arcID+1).
	chains := make([]bchain, len(d.Arcs))
	for i := range d.Arcs {
		sd := i + 1
		chains[i] = bchain{
			verts: []int{bg.startVert(sd), bg.endVert(sd)},
			darts: []int{sd},
		}
	}

	cur := bg
	for k := 0; k < 3; k++ {
		nxt, rm := cur.medial()
		for i := range chains {
			chains[i] = chains[i].afterMedial(rm)
		}
		cur = nxt
	}

	// Pick the outer face: type-"fvv" vertex with maximum degree.
	outer := -1
	bestDeg := -1
	for vid, v := range cur.verts {
		if v.typ == "fvv" && len(v.darts) > bestDeg {
			bestDeg = len(v.darts)
			outer = vid
		}
	}
	if outer < 0 {
		return nil, fmt.Errorf("beautify: no fvv vertex found")
	}

	// Walk the outer face's perimeter the way KnotFolio does. For each
	// dart d at outer, the face on d's right is a small cell touching
	// outer. Collect the verts on that cell except the second-to-last
	// (which is outer itself for the canonical face_darts order).
	outerOf := map[int]bool{}
	var perimeter []int
	for _, d := range cur.verts[outer].darts {
		fd := cur.faceDarts(d)
		if fd == nil {
			dumpBeautifyFailure(orig, bg0, cur, outer, d)
			return nil, fmt.Errorf("beautify: face walk failed at outer dart %d", d)
		}
		verts := make([]int, len(fd))
		for i, dd := range fd {
			verts[i] = cur.dartVert[dd]
		}
		idx := -1
		for i, vv := range verts {
			if vv == outer {
				idx = i
				break
			}
		}
		if idx < 0 {
			return nil, fmt.Errorf("beautify: outer not on its own boundary")
		}
		rotated := make([]int, 0, len(verts)-1)
		rotated = append(rotated, verts[idx+1:]...)
		rotated = append(rotated, verts[:idx]...)
		// drop the trailing entry (matches KnotFolio's verts.pop()).
		if len(rotated) > 0 {
			rotated = rotated[:len(rotated)-1]
		}
		perimeter = append(perimeter, rotated...)
		for _, vv := range rotated {
			outerOf[vv] = true
		}
	}
	if len(perimeter) < 3 {
		return nil, fmt.Errorf("beautify: degenerate outer perimeter (%d)", len(perimeter))
	}

	// Solve Tutte. Unknowns are all verts EXCEPT outer; that gives n-1
	// columns. We index columns 0..n-2 by skipping outer in the row-and-
	// column ordering.
	n := len(cur.verts)
	colOf := make([]int, n)
	for i, j := 0, 0; i < n; i++ {
		if i == outer {
			colOf[i] = -1
			continue
		}
		colOf[i] = j
		j++
	}
	cols := n - 1
	matX := make([][]float64, 0, cols)
	matY := make([][]float64, 0, cols)
	addRow := func(rowX, rowY []float64) {
		matX = append(matX, rowX)
		matY = append(matY, rowY)
	}

	// Pin perimeter verts on a regular polygon of unit radius.
	for i, vid := range perimeter {
		if vid == outer {
			return nil, fmt.Errorf("beautify: outer in perimeter")
		}
		angle := 2 * math.Pi * float64(i) / float64(len(perimeter))
		rowX := make([]float64, cols+1)
		rowY := make([]float64, cols+1)
		rowX[colOf[vid]] = 1
		rowY[colOf[vid]] = 1
		rowX[cols] = math.Cos(angle)
		rowY[cols] = math.Sin(angle)
		addRow(rowX, rowY)
	}

	// Interior equations: x_v = average of x_neighbours, written as
	// (sum x_u) - deg(v)*x_v = 0.
	for vid, v := range cur.verts {
		if vid == outer || outerOf[vid] {
			continue
		}
		row := make([]float64, cols+1)
		for _, dd := range v.darts {
			u := cur.endVert(dd)
			if u == outer {
				continue
			}
			row[colOf[u]] += 1
			row[colOf[vid]] -= 1
		}
		addRow(row, append([]float64(nil), row...))
	}

	if len(matX) != cols {
		return nil, fmt.Errorf("beautify: matrix is %d×%d, expected %d×%d",
			len(matX), cols+1, cols, cols+1)
	}

	if err := rowReduce(matX); err != nil {
		return nil, fmt.Errorf("beautify: cannot solve layout — diagram is "+
			"too simple for Tutte embedding (the underlying graph is not "+
			"3-connected; try a more complex starting diagram or fewer "+
			"R1 simplifications). Inner error: %v", err)
	}
	if err := rowReduce(matY); err != nil {
		return nil, fmt.Errorf("beautify: cannot solve layout — diagram is "+
			"too simple for Tutte embedding. Inner error: %v", err)
	}

	// Extract solved positions.
	posX := make([]float64, n)
	posY := make([]float64, n)
	for vid := 0; vid < n; vid++ {
		if vid == outer {
			continue
		}
		j := colOf[vid]
		if math.Abs(matX[j][j]-1) > 1e-6 || math.Abs(matY[j][j]-1) > 1e-6 {
			return nil, fmt.Errorf("beautify: pivot missing for vert %d", vid)
		}
		posX[vid] = matX[j][cols]
		posY[vid] = matY[j][cols]
	}

	// Compute the bounding box over all vertex chain points (= what we'll
	// actually draw), then map to the canvas centered with a 5% margin.
	minX, maxX := math.Inf(1), math.Inf(-1)
	minY, maxY := math.Inf(1), math.Inf(-1)
	consider := func(vid int) {
		if vid == outer {
			return
		}
		x, y := posX[vid], posY[vid]
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}
	for _, c := range chains {
		for _, vid := range c.verts {
			consider(vid)
		}
	}
	if !(maxX > minX) || !(maxY > minY) {
		return nil, fmt.Errorf("beautify: degenerate bbox")
	}
	margin := 0.05
	cx := float64(canvasW) / 2
	cy := float64(canvasH) / 2
	span := math.Max(maxX-minX, maxY-minY)
	r := math.Min(float64(canvasW), float64(canvasH)) * (1 - 2*margin) / 2
	scale := r / (span / 2)
	mx := (minX + maxX) / 2
	my := (minY + maxY) / 2
	toCanvas := func(vid int) image.Point {
		x := cx + (posX[vid]-mx)*scale
		y := cy + (posY[vid]-my)*scale
		return image.Point{X: int(math.Round(x)), Y: int(math.Round(y))}
	}

	// Assemble the new Diagram.
	out := &Diagram{
		Crossings: make([]image.Point, len(d.Crossings)),
		Arcs:      make([]Arc, len(d.Arcs)),
	}
	for v := range d.Crossings {
		out.Crossings[v] = toCanvas(v)
	}
	for i, c := range chains {
		poly := make([]image.Point, len(c.verts))
		for j, vid := range c.verts {
			poly[j] = toCanvas(vid)
		}
		out.Arcs[i] = Arc{
			Polyline: poly,
			Start:    d.Arcs[i].Start,
			End:      d.Arcs[i].End,
		}
	}
	return out, nil
}

// ----- bgraph: planar graph with darts and CCW rotation system -----

type bvert struct {
	darts []int
	typ   string
}

type bgraph struct {
	verts    []bvert
	dartVert map[int]int // dart d -> id of vertex where d starts
}

func (g *bgraph) startVert(d int) int { return g.dartVert[d] }
func (g *bgraph) endVert(d int) int   { return g.dartVert[-d] }

// newBgraphFromDart builds the initial bgraph from an existing dartGraph.
// nVerts is the number of crossings (= len(dg.adj)).
func newBgraphFromDart(dg *dartGraph, nVerts int) *bgraph {
	bg := &bgraph{verts: make([]bvert, nVerts), dartVert: make(map[int]int, 4*nVerts)}
	for v := 0; v < nVerts; v++ {
		adj := dg.adj[v]
		bg.verts[v].darts = []int{adj[0], adj[1], adj[2], adj[3]}
		for _, dd := range adj {
			bg.dartVert[dd] = v
		}
	}
	return bg
}

// faceDarts walks the face on the right of d. Returns the cyclic sequence of
// darts ending with d itself. Returns nil if the walk fails to close (a bug
// signal — should not happen for a well-formed planar graph).
func (g *bgraph) faceDarts(d int) []int {
	var out []int
	cur := d
	for i := 0; i < 1000000; i++ {
		v := g.endVert(cur)
		adj := g.verts[v].darts
		idx := -1
		for k, dd := range adj {
			if dd == -cur {
				idx = k
				break
			}
		}
		if idx < 0 {
			return nil
		}
		cur = adj[(idx+len(adj)-1)%len(adj)]
		out = append(out, cur)
		if cur == d {
			return out
		}
	}
	return nil
}

// faceCanonDart returns the smallest signed dart on the face containing d.
// Used as a stable identifier for the face.
func (g *bgraph) faceCanonDart(d int) int {
	fd := g.faceDarts(d)
	if fd == nil {
		return 0
	}
	best := fd[0]
	for _, dd := range fd[1:] {
		if dd < best {
			best = dd
		}
	}
	return best
}

// medialRemap records how darts and verts of an old graph map into a new
// graph produced by a single medial-barycentric subdivision.
type medialRemap struct {
	vertRemap map[int]int // old vid -> new vid
	edgeMid   map[int]int // |old dart| -> new vid of the edge midpoint
	leftDart  map[int]int // old dart d -> new dart at vertRemap[startVert(d)] toward edge midpoint
	rightDart map[int]int // old dart d -> new dart at edgeMid pointing toward endVert(d)
}

// medial returns the medial-barycentric subdivision of g together with the
// dart/vertex remap. The new graph contains: a renamed copy of every old
// vertex; one new vertex on each old edge; and one new vertex inside each
// old face.
func (g *bgraph) medial() (*bgraph, medialRemap) {
	out := &bgraph{dartVert: make(map[int]int)}
	rm := medialRemap{
		vertRemap: make(map[int]int, len(g.verts)),
		edgeMid:   make(map[int]int),
		leftDart:  make(map[int]int),
		rightDart: make(map[int]int),
	}

	nextDart := 1
	dartByKey := make(map[string]int)
	dartFor := func(key string) int {
		if d, ok := dartByKey[key]; ok {
			return d
		}
		d := nextDart
		nextDart++
		dartByKey[key] = d
		return d
	}
	// vertEdgeKey is keyed on the *signed* dart, not |dart|. For a
	// regular edge between distinct endpoints, +d and -d live at
	// different vertices so the unsigned key suffices. For a self-loop
	// edge, +d and -d live at the *same* vertex, so an unsigned key
	// would collide and produce the same fresh dart for both —
	// breaking the post-subdivision rotation system. Signed keys
	// distinguish them.
	vertEdgeKey := func(vid, d int) string { return fmt.Sprintf("V%d:D%d", vid, d) }
	edgeFaceKey := func(ae, fc int) string { return fmt.Sprintf("E%d:F%d", ae, fc) }

	// Phase 1: rename old verts. Each old vert v becomes a new vert with
	// the same id. Each old dart d in v's CCW list becomes a new dart at v
	// pointing toward the edge midpoint of |d|.
	for vid, v := range g.verts {
		nv := bvert{typ: v.typ + "v"}
		for _, d := range v.darts {
			nd := dartFor(vertEdgeKey(vid, d))
			nv.darts = append(nv.darts, nd)
			out.dartVert[nd] = vid
			rm.leftDart[d] = nd
		}
		rm.vertRemap[vid] = vid
		out.verts = append(out.verts, nv)
	}

	// Phase 2: edge midpoints. CCW order at the midpoint:
	//   0: toward F_R(d) (face on the right of d)
	//   1: back to startVert(d)
	//   2: toward F_L(d) (face on the left of d)
	//   3: back to endVert(d)
	seenEdge := make(map[int]bool)
	for vid, v := range g.verts {
		for _, d := range v.darts {
			ae := absi(d)
			if seenEdge[ae] {
				continue
			}
			seenEdge[ae] = true

			// Use d (whatever sign it has) as our canonical orientation.
			u := vid
			uOther := g.endVert(d)
			fR := g.faceCanonDart(d)
			fL := g.faceCanonDart(-d)

			// Phase 1 named the dart at u for this edge using the
			// signed dart d (which lives at u). Use the matching key
			// here so d1 (the midpoint's "back to u" dart) negates
			// the same fresh ID. d3 mirrors that for -d at uOther.
			d0 := dartFor(edgeFaceKey(ae, fR))
			d1 := -dartFor(vertEdgeKey(u, d))
			d2 := dartFor(edgeFaceKey(ae, fL))
			d3 := -dartFor(vertEdgeKey(uOther, -d))

			mvid := len(out.verts)
			out.verts = append(out.verts, bvert{typ: "e", darts: []int{d0, d1, d2, d3}})
			out.dartVert[d0] = mvid
			out.dartVert[d1] = mvid
			out.dartVert[d2] = mvid
			out.dartVert[d3] = mvid

			rm.edgeMid[ae] = mvid
			// rightDart[d] = dart at midpoint pointing toward endVert(d).
			// d3 points toward uOther = endVert(d).
			// d1 points toward u = startVert(d).
			rm.rightDart[d] = d3
			rm.rightDart[-d] = d1
		}
	}

	// Phase 3: face centers. For each face f, walk its darts and add one
	// dart at the face center for each boundary dart, in walking order.
	// The face center's darts each go to the corresponding edge midpoint.
	seenFace := make(map[int]bool)
	for _, v := range g.verts {
		for _, d := range v.darts {
			fc := g.faceCanonDart(d)
			if seenFace[fc] {
				continue
			}
			seenFace[fc] = true

			fd := g.faceDarts(d)
			fvid := len(out.verts)
			fv := bvert{typ: "f"}
			for _, fdd := range fd {
				// On this face's boundary, fdd is oriented with this
				// face on the right (faceDarts walks the right face).
				// The dart at the edge midpoint pointing toward this
				// face is dartFor("E<|fdd|>:F<fc>"), and the face
				// center sees it from the other end (negate).
				key := edgeFaceKey(absi(fdd), fc)
				neg := -dartFor(key)
				fv.darts = append(fv.darts, neg)
				out.dartVert[neg] = fvid
			}
			out.verts = append(out.verts, fv)
		}
	}

	return out, rm
}

// ----- arc-chain tracking through medial subdivisions -----

// bchain records the sequence of vertex IDs an arc passes through at the
// current iteration, plus, for each consecutive pair, the dart used at the
// previous-iteration graph (so parallel edges can be disambiguated when
// inserting midpoints).
type bchain struct {
	verts []int
	darts []int // len(darts) == len(verts) - 1
}

func (c bchain) afterMedial(rm medialRemap) bchain {
	out := bchain{verts: []int{rm.vertRemap[c.verts[0]]}}
	for j, d := range c.darts {
		out.darts = append(out.darts, rm.leftDart[d])
		out.verts = append(out.verts, rm.edgeMid[absi(d)])
		out.darts = append(out.darts, rm.rightDart[d])
		out.verts = append(out.verts, rm.vertRemap[c.verts[j+1]])
	}
	return out
}

// ----- dense Gauss–Jordan with partial pivoting -----

// rowReduce reduces matrix in place to reduced row-echelon form. matrix is
// rows × (rows+1); we expect a square coefficient block plus one RHS column.
// Returns an error if the system is singular.
func rowReduce(m [][]float64) error {
	rows := len(m)
	if rows == 0 {
		return nil
	}
	cols := len(m[0])
	i, j := 0, 0
	for i < rows && j < cols {
		bestI := i
		for k := i + 1; k < rows; k++ {
			if math.Abs(m[k][j]) > math.Abs(m[bestI][j]) {
				bestI = k
			}
		}
		if bestI != i {
			m[i], m[bestI] = m[bestI], m[i]
		}
		if math.Abs(m[i][j]) < 1e-12 {
			j++
			continue
		}
		c := m[i][j]
		for l := j; l < cols; l++ {
			m[i][l] /= c
		}
		for k := 0; k < rows; k++ {
			if k == i {
				continue
			}
			c := m[k][j]
			if c == 0 {
				continue
			}
			m[k][j] = 0
			for l := j + 1; l < cols; l++ {
				m[k][l] -= c * m[i][l]
			}
		}
		i++
		j++
	}
	if i < rows {
		return fmt.Errorf("singular: %d/%d pivots", i, rows)
	}
	return nil
}

// dumpBeautifyFailure logs the originating Diagram, the un-subdivided
// bgraph (bg0), and the failure-stage bgraph (cur) when a face-walk
// can't close. Called from Beautify on the unrecoverable error path.
func dumpBeautifyFailure(d *Diagram, bg0 *bgraph, cur *bgraph, outer, badDart int) {
	log.Printf("BFY: face walk failed at outer=%d dart=%d", outer, badDart)
	log.Printf("BFY: orig diagram: %d crossings, %d arcs", len(d.Crossings), len(d.Arcs))
	for i, c := range d.Crossings {
		log.Printf("BFY:   C%d @ (%d,%d)", i, c.X, c.Y)
	}
	for i, a := range d.Arcs {
		log.Printf("BFY:   A%d C%d(%s)→C%d(%s) %d pts",
			i, a.Start.Crossing, overTag(a.Start.Over),
			a.End.Crossing, overTag(a.End.Over), len(a.Polyline))
	}
	log.Printf("BFY: bg0 (pre-subdiv): %d verts", len(bg0.verts))
	for i, v := range bg0.verts {
		log.Printf("BFY:   bg0 V%d typ=%q darts=%v", i, v.typ, v.darts)
	}
	// Trace the failing face walk one step at a time so we see exactly
	// which dart's negative is missing.
	log.Printf("BFY: cur (post-subdiv): %d verts", len(cur.verts))
	log.Printf("BFY: outer V%d darts: %v", outer, cur.verts[outer].darts)
	cur2 := badDart
	for step := 0; step < 8; step++ {
		v := cur.endVert(cur2)
		log.Printf("BFY: walk step %d: cur=%d, end=V%d, adj=%v",
			step, cur2, v, cur.verts[v].darts)
		idx := -1
		for k, dd := range cur.verts[v].darts {
			if dd == -cur2 {
				idx = k
				break
			}
		}
		if idx < 0 {
			log.Printf("BFY:   FAIL: -%d not in V%d.darts", cur2, v)
			return
		}
		cur2 = cur.verts[v].darts[(idx+len(cur.verts[v].darts)-1)%len(cur.verts[v].darts)]
	}
}

// dumpDiagram logs a Diagram's structure. Invoked from doReidemeister
// after a successful R1/R2 rewrite so we can compare states before
// and after a Beautify failure.
func dumpDiagramState(label string, d *Diagram) {
	if d == nil {
		log.Printf("DUMP %s: nil", label)
		return
	}
	log.Printf("DUMP %s: %d crossings, %d arcs", label, len(d.Crossings), len(d.Arcs))
	for i, c := range d.Crossings {
		log.Printf("DUMP %s:   C%d @ (%d,%d)", label, i, c.X, c.Y)
	}
	for i, a := range d.Arcs {
		log.Printf("DUMP %s:   A%d C%d(%s)→C%d(%s) %d pts",
			label, i, a.Start.Crossing, overTag(a.Start.Over),
			a.End.Crossing, overTag(a.End.Over), len(a.Polyline))
	}
}
