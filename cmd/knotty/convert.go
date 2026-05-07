package main

import (
	"fmt"
	"image"
	"math"
	"sort"
)

// Constants from knotfolio's constants.mjs.
const (
	spurLength       = 5
	diagramLineWidth = 3
	epsilon          = 1e-6
)

// pixbuf is a single-color raster: 0 = empty, 1 = ink, -1 = error marker.
// We use only one color since dataset PNGs are monochrome knot diagrams —
// the multi-color component encoding in knotfolio (one color per link
// component) isn't available here.
type pixbuf struct {
	w, h int
	buf  []int8
}

func newPixbuf(w, h int) *pixbuf {
	return &pixbuf{w: w, h: h, buf: make([]int8, w*h)}
}

func (p *pixbuf) at(x, y int) int8 {
	if x < 0 || y < 0 || x >= p.w || y >= p.h {
		return 0
	}
	return p.buf[y*p.w+x]
}

func (p *pixbuf) clone() *pixbuf {
	cp := newPixbuf(p.w, p.h)
	copy(cp.buf, p.buf)
	return cp
}

// binaryFromImage produces a 1-channel ink/empty buffer from an image.
// Pixels that are opaque and darker than 50% luma count as ink; transparent
// or light pixels are empty. This handles both opaque-white-bg PNGs and
// alpha-on-transparent rasterized SVGs.
func binaryFromImage(img image.Image) *pixbuf {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	p := newPixbuf(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, a := img.At(x+b.Min.X, y+b.Min.Y).RGBA()
			if a < 0x8000 {
				continue
			}
			if (r+g+bl)/3 < 0x8000 {
				p.buf[y*w+x] = 1
			}
		}
	}
	for x := 0; x < w; x++ {
		p.buf[x] = 0
		p.buf[(h-1)*w+x] = 0
	}
	for y := 0; y < h; y++ {
		p.buf[y*w] = 0
		p.buf[y*w+w-1] = 0
	}
	return p
}

// sameNeighbors counts 8-connected same-color neighbors (excluding center).
func (p *pixbuf) sameNeighbors(x, y int) int {
	c := p.buf[y*p.w+x]
	if c <= 0 {
		return 0
	}
	count := 0
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			if p.at(x+dx, y+dy) == c {
				count++
			}
		}
	}
	return count
}

// thin is the morphological thinning pass — port of knotfolio clean_up()
// (minus strip_errors and boundary-clearing, which binaryFromImage already
// handles). Iterates mthin(3,4) until stable, retries mthin(2,6), and ends
// with a one-pass tip trim.
func (p *pixbuf) thin() {
	var nbuf [9]int8
	w := p.w
	mthin := func(minPcount, maxPcount int) bool {
		changed := false
		for y := 1; y < p.h-1; y++ {
			for x := 1; x < w-1; x++ {
				c := p.buf[y*w+x]
				if c <= 0 {
					continue
				}
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if p.buf[(y+dy)*w+(x+dx)] == c {
							nbuf[3*(dy+1)+(dx+1)] = 1
						} else {
							nbuf[3*(dy+1)+(dx+1)] = 0
						}
					}
				}
				if nbuf[3*0+1] != 0 {
					if nbuf[3*1+0] != 0 {
						nbuf[3*0+0] = 1
					}
					if nbuf[3*1+2] != 0 {
						nbuf[3*0+2] = 1
					}
				}
				if nbuf[3*2+1] != 0 {
					if nbuf[3*1+0] != 0 {
						nbuf[3*2+0] = 1
					}
					if nbuf[3*1+2] != 0 {
						nbuf[3*2+2] = 1
					}
				}
				state := nbuf[3*1+2]
				pcount := 0
				ccount := 0
				step := func(dx, dy int) {
					c2 := nbuf[3*(1+dy)+(1+dx)]
					if c2 != state {
						ccount++
						state = c2
					}
					if c2 > 0 {
						pcount++
					}
				}
				step(1, 1)
				step(0, 1)
				step(-1, 1)
				step(-1, 0)
				step(-1, -1)
				step(0, -1)
				step(1, -1)
				step(1, 0)
				if pcount == 0 {
					p.buf[y*w+x] = 0
				} else if ccount == 2 && minPcount <= pcount && pcount <= maxPcount {
					p.buf[y*w+x] = 0
					changed = true
				}
			}
		}
		return changed
	}
	for changed := true; changed; {
		changed = mthin(3, 4)
		if !changed {
			changed = mthin(2, 6)
		}
	}
	tbuf := make([]int8, p.w*p.h)
	for y := 1; y < p.h-1; y++ {
		for x := 1; x < w-1; x++ {
			c := p.buf[y*w+x]
			if c <= 0 {
				continue
			}
			icount := -1
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if p.buf[(y+dy)*w+(x+dx)] == c {
						icount++
					}
				}
			}
			if icount == 1 {
				tbuf[y*w+x] = 1
			}
		}
	}
	for i, v := range tbuf {
		if v > 0 {
			p.buf[i] = 0
		}
	}
}

// deleteSpurs removes dead-end branches shorter than spurLength pixels,
// then strips any isolated pixels. Port of knotfolio convert() pre-match
// cleanup.
func (p *pixbuf) deleteSpurs() {
	var rec func(x, y, gas int) bool
	rec = func(x, y, gas int) bool {
		if gas <= 0 {
			return false
		}
		n := p.sameNeighbors(x, y)
		if n == 0 {
			return false
		}
		if n == 1 {
			c := p.buf[y*p.w+x]
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					if p.at(x+dx, y+dy) == c {
						p.buf[y*p.w+x] = 0
						ok := rec(x+dx, y+dy, gas-1)
						if ok {
							return true
						}
						p.buf[y*p.w+x] = c
						return false
					}
				}
			}
			return false
		}
		return true
	}
	for y := 1; y < p.h-1; y++ {
		for x := 1; x < p.w-1; x++ {
			if p.buf[y*p.w+x] > 0 && p.sameNeighbors(x, y) == 1 {
				rec(x, y, spurLength)
			}
		}
	}
	for y := 1; y < p.h-1; y++ {
		for x := 1; x < p.w-1; x++ {
			if p.buf[y*p.w+x] > 0 && p.sameNeighbors(x, y) == 0 {
				p.buf[y*p.w+x] = 0
			}
		}
	}
}

// junctions returns the pixel locations with more than 2 same-color
// neighbors — these mean understrand fused to overstrand (user error in
// knotfolio's world; for rendered diagrams it usually means the skeleton
// couldn't cleanly resolve a small crossing).
func (p *pixbuf) junctions() []image.Point {
	var pts []image.Point
	for y := 1; y < p.h-1; y++ {
		for x := 1; x < p.w-1; x++ {
			if p.buf[y*p.w+x] > 0 && p.sameNeighbors(x, y) > 2 {
				pts = append(pts, image.Point{X: x, Y: y})
			}
		}
	}
	return pts
}

// thicken expands every ink region by 1 pixel in its 8-neighborhood. Port
// of knotfolio thicken(). Used on a *copy* of the skeleton to build the
// scoring grid for endpoint matching.
func (p *pixbuf) thicken() {
	tbuf := make([]int8, p.w*p.h)
	for y := 0; y < p.h; y++ {
		for x := 0; x < p.w; x++ {
			if p.buf[y*p.w+x] > 0 {
				continue
			}
			var c int8
		find:
			for y2 := y - 1; y2 <= y+1; y2++ {
				if y2 < 0 || y2 >= p.h {
					continue
				}
				for x2 := x - 1; x2 <= x+1; x2++ {
					if x2 < 0 || x2 >= p.w {
						continue
					}
					if v := p.buf[y2*p.w+x2]; v > 0 {
						c = v
						break find
					}
				}
			}
			tbuf[y*p.w+x] = c
		}
	}
	for i, v := range tbuf {
		if v > 0 {
			p.buf[i] = v
		}
	}
}

// findEndpoints returns every pixel with exactly one same-color neighbor.
func (p *pixbuf) findEndpoints() []image.Point {
	var eps []image.Point
	for y := 1; y < p.h-1; y++ {
		for x := 1; x < p.w-1; x++ {
			if p.buf[y*p.w+x] > 0 && p.sameNeighbors(x, y) == 1 {
				eps = append(eps, image.Pt(x, y))
			}
		}
	}
	return eps
}

func absi(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// bresenham yields pixel points along the line from p1 to p2 inclusive.
func bresenham(p1, p2 image.Point) []image.Point {
	x1, y1 := p1.X, p1.Y
	x2, y2 := p2.X, p2.Y
	dx := absi(x2 - x1)
	sx := 1
	if x1 >= x2 {
		sx = -1
		if x1 == x2 {
			sx = 0
		}
	}
	dy := -absi(y2 - y1)
	sy := 1
	if y1 >= y2 {
		sy = -1
		if y1 == y2 {
			sy = 0
		}
	}
	err := dx + dy
	var pts []image.Point
	for {
		pts = append(pts, image.Pt(x1, y1))
		if x1 == x2 && y1 == y2 {
			return pts
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x1 += sx
		}
		if e2 <= dx {
			err += dx
			y1 += sy
		}
	}
}

// fPoint is a floating-point 2D position, used for intersection math.
type fPoint struct{ x, y float64 }

func fpt(p image.Point) fPoint { return fPoint{float64(p.X), float64(p.Y)} }

func (p fPoint) near(q fPoint) bool {
	return math.Abs(p.x-q.x) < epsilon && math.Abs(p.y-q.y) < epsilon
}

func (p fPoint) toImage() image.Point {
	return image.Pt(int(math.Round(p.x)), int(math.Round(p.y)))
}

// linesIntersect returns the intersection of the lines through (a,b) and
// (c,d), or (_, false) if the lines are parallel.
func linesIntersect(a, b, c, d fPoint) (fPoint, bool) {
	det := (a.x-b.x)*(c.y-d.y) - (a.y-b.y)*(c.x-d.x)
	if math.Abs(det) < epsilon {
		return fPoint{}, false
	}
	d1 := a.x*b.y - a.y*b.x
	d2 := c.x*d.y - c.y*d.x
	return fPoint{
		x: (d1*(c.x-d.x) - d2*(a.x-b.x)) / det,
		y: (d1*(c.y-d.y) - d2*(a.y-b.y)) / det,
	}, true
}

func segmentDistance(a, b, q fPoint) float64 {
	if a.near(b) {
		return math.Hypot(q.x-a.x, q.y-a.y)
	}
	vx, vy := b.x-a.x, b.y-a.y
	wx, wy := q.x-a.x, q.y-a.y
	norm2 := vx*vx + vy*vy
	t := (vx*wx + vy*wy) / norm2
	if t < 0 {
		return math.Hypot(q.x-a.x, q.y-a.y)
	}
	if t > 1 {
		return math.Hypot(q.x-b.x, q.y-b.y)
	}
	return math.Abs((-vy*wx + vx*wy) / math.Sqrt(norm2))
}

func segmentsIntersect(a, b, c, d fPoint) (fPoint, bool) {
	pt, ok := linesIntersect(a, b, c, d)
	if !ok {
		return fPoint{}, false
	}
	if segmentDistance(a, b, pt) > epsilon || segmentDistance(c, d, pt) > epsilon {
		return fPoint{}, false
	}
	return pt, true
}

func segmentContains(a, b, q fPoint) bool {
	if a.near(b) {
		return a.near(q)
	}
	return segmentDistance(a, b, q) < epsilon
}

// projectOntoSegment returns the closest point on segment a-b to q. When
// a and b coincide it returns a.
func projectOntoSegment(a, b, q fPoint) fPoint {
	if a.near(b) {
		return a
	}
	vx, vy := b.x-a.x, b.y-a.y
	wx, wy := q.x-a.x, q.y-a.y
	t := (vx*wx + vy*wy) / (vx*vx + vy*vy)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return fPoint{a.x + t*vx, a.y + t*vy}
}

// matchEndpoints runs knotfolio's greedy min-weight matching + 2-opt over
// all endpoint pairs. Each candidate pair's score is (distance − line-width
// × crossed-strands) / crossed-strands, rejecting pairs that don't cross at
// least one other strand (those are backtracking, not under-passages).
// Returns pairs as indices into points, or nil if the matching is not
// perfect.
func matchEndpoints(thick *pixbuf, points []image.Point) [][2]int {
	n := len(points)
	if n == 0 || n%2 != 0 {
		return nil
	}
	graph := make([][]float64, n)
	for i := range graph {
		graph[i] = make([]float64, n)
		for j := range graph[i] {
			graph[i][j] = math.Inf(1)
		}
	}
	type edge struct {
		i, j  int
		score float64
	}
	var all []edge
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			p1, p2 := points[i], points[j]
			d := math.Hypot(float64(p1.X-p2.X), float64(p1.Y-p2.Y))
			pcount := 0
			var state int8 = -2
			for _, lp := range bresenham(p1, p2) {
				c := thick.at(lp.X, lp.Y)
				if c == 0 {
					c = thick.at(lp.X+1, lp.Y)
				}
				if c == 0 {
					c = thick.at(lp.X, lp.Y+1)
				}
				if c != state {
					if c > 0 {
						pcount++
					}
					state = c
				}
			}
			if pcount > 1 {
				count := pcount - 1
				if count > 2 {
					count = 2
				}
				score := (d - float64(diagramLineWidth*count)) / float64(count)
				if score < 0 {
					score = 0
				}
				graph[i][j] = score
				graph[j][i] = score
				all = append(all, edge{i, j, score})
			}
		}
	}
	sort.Slice(all, func(a, b int) bool { return all[a].score < all[b].score })
	used := make([]bool, n)
	var edges [][2]int
	for _, e := range all {
		if used[e.i] || used[e.j] {
			continue
		}
		edges = append(edges, [2]int{e.i, e.j})
		used[e.i] = true
		used[e.j] = true
	}
	if 2*len(edges) < n {
		return nil
	}
	for keepGoing := true; keepGoing; {
		keepGoing = false
		for i := 0; i < len(edges); i++ {
			for j := i + 1; j < len(edges); j++ {
				p1, p2 := edges[i][0], edges[i][1]
				q1, q2 := edges[j][0], edges[j][1]
				d1 := graph[p1][p2] + graph[q1][q2]
				d2 := graph[p1][q1] + graph[p2][q2]
				d3 := graph[p1][q2] + graph[q1][p2]
				if d2 < d1 && d2 <= d3 {
					edges[i][1] = q1
					edges[j][0] = p2
					keepGoing = true
				} else if d3 < d1 && d3 <= d2 {
					edges[i][1] = q2
					edges[j][1] = p2
					keepGoing = true
				}
			}
		}
	}
	// Uncrossing pass: score-based 2-opt only swaps on strict improvement,
	// so a matching whose minimum-score assignment happens to be the
	// self-crossing one among the three ways to pair 4 points will remain
	// crossing. For 4 points in general position, exactly one of the three
	// partitionings is self-crossing, so an alternative always exists.
	// Accept a higher score to trade crossings away.
	for changed := true; changed; {
		changed = false
		for i := 0; i < len(edges); i++ {
			for j := i + 1; j < len(edges); j++ {
				p1, p2 := edges[i][0], edges[i][1]
				q1, q2 := edges[j][0], edges[j][1]
				if _, ok := segmentsIntersect(
					fpt(points[p1]), fpt(points[p2]),
					fpt(points[q1]), fpt(points[q2])); !ok {
					continue
				}
				type opt struct {
					a1, a2, b1, b2 int
					score          float64
				}
				opts := []opt{
					{p1, q1, p2, q2, graph[p1][q1] + graph[p2][q2]},
					{p1, q2, p2, q1, graph[p1][q2] + graph[p2][q1]},
				}
				sort.Slice(opts, func(a, b int) bool { return opts[a].score < opts[b].score })
				for _, o := range opts {
					if math.IsInf(o.score, 1) {
						continue
					}
					if _, ok := segmentsIntersect(
						fpt(points[o.a1]), fpt(points[o.a2]),
						fpt(points[o.b1]), fpt(points[o.b2])); ok {
						continue
					}
					edges[i] = [2]int{o.a1, o.a2}
					edges[j] = [2]int{o.b1, o.b2}
					changed = true
					break
				}
			}
		}
	}
	return edges
}

// convEdge is an atomic straight-line piece of the planar graph. Over
// strands come from walking the skeleton (over=true); matched under-strand
// segments are injected with over=false.
type convEdge struct {
	v1, v2 int
	over   bool
}

// walkPath walks one skeleton component from (sx,sy), destructively
// zeroing visited pixels, emitting a straight edge each time direction
// changes. Mirrors knotfolio walk_path.
func walkPath(p *pixbuf, sx, sy int, verts *[]fPoint, edges *[]convEdge) {
	x, y := sx, sy
	pt1 := len(*verts)
	*verts = append(*verts, fPoint{float64(x), float64(y)})
	last := -1
	if p.sameNeighbors(x, y) == 2 {
		last = pt1
	}
	w := p.w
	for p.buf[y*w+x] == 1 {
		p.buf[y*w+x] = 0
		stepped := false
		var dx0, dy0 int
		for dy := -1; dy <= 1 && !stepped; dy++ {
			for dx := -1; dx <= 1 && !stepped; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				if p.at(x+dx, y+dy) == 1 {
					dx0, dy0 = dx, dy
					x += dx
					y += dy
					for p.at(x+dx0, y+dy0) == 1 {
						p.buf[y*w+x] = 0
						x += dx0
						y += dy0
					}
					pt2 := len(*verts)
					*verts = append(*verts, fPoint{float64(x), float64(y)})
					*edges = append(*edges, convEdge{pt1, pt2, true})
					pt1 = pt2
					stepped = true
				}
			}
		}
		if !stepped {
			if last >= 0 {
				*edges = append(*edges, convEdge{pt1, last, true})
			}
			return
		}
	}
}

func findVertID(verts []fPoint, v fPoint) int {
	for i, u := range verts {
		if u.near(v) {
			return i
		}
	}
	return -1
}

// FusedJunctionsError reports that the thinned skeleton contains pixels
// with more than two same-color neighbors AND those pixels form a cluster
// the convert pipeline cannot interpret as a single 4-valent crossing —
// e.g. a Y-junction with 3 exits or a tangle with 5+ exits. Clean
// "solid X" crossings (4 exits) are now resolved as ambiguous crossings
// rather than being rejected here. Junctions holds the pixel coordinates
// of the offending cluster pixels so the caller can render them for
// debugging.
type FusedJunctionsError struct {
	Junctions []image.Point
}

func (e *FusedJunctionsError) Error() string {
	return fmt.Sprintf("%d fused junctions in skeleton — cannot interpret", len(e.Junctions))
}

// BadTopologyError is returned when convert builds a graph that the arc
// walker can't traverse: a self-loop edge or a vertex with degree
// outside {1, 2, 4}. Locations are the offending vertex coordinates so
// callers can highlight them in the UI.
type BadTopologyError struct {
	Locations []image.Point
	Reason    string
}

func (e *BadTopologyError) Error() string {
	return fmt.Sprintf("bad convert topology at %d location(s): %s", len(e.Locations), e.Reason)
}

// junctionCluster is one 8-connected group of skeleton junction pixels
// (sameNeighbors > 2) that the convert pipeline has chosen to interpret
// as a single 4-valent ambiguous crossing — a "solid X" with no detected
// over/under gap. exits are the four skeleton pixels just outside the
// cluster that the four arms enter through; centroid is the geometric
// center used as the crossing's vertex position.
type junctionCluster struct {
	pixels   []image.Point
	centroid fPoint
	exits    []image.Point
}

// junctionMergeDist is the maximum skeleton-path distance (in 8-connected
// steps) between two junction pixels for them to be considered part of
// the same ambiguous crossing. Thinning a thick "solid X" tends to
// produce two Y-junctions joined by a short bridge (a "bowtie"), and
// that bridge is typically the line thickness in length. 15 covers
// hand-drawn strokes up to ~12 px wide; larger strokes will not merge
// and will still report as fused-junction errors.
const junctionMergeDist = 15

// findJunctionClusters groups junction pixels (sameNeighbors > 2) into
// merged clusters, treating each cluster as a single ambiguous 4-valent
// crossing.
//
// Clustering is two-stage: junctions are first grouped by 8-connectivity
// (handles compact multi-pixel junctions) and then merged across short
// skeleton bridges of at most junctionMergeDist hops (handles "bowtie"
// patterns from thinning a thick X). For each merged cluster we then
// add the bridging skeleton pixels themselves to the cluster, so that
// erasing the cluster from the skeleton leaves exactly four outward
// stub-arms whose tip pixels become the cluster's exits.
//
// Clusters whose exit count is exactly four are returned in resolvable;
// everything else is reported in unresolvable so the caller can bail
// with FusedJunctionsError. p is not modified — the caller erases
// cluster pixels later, after findEndpoints and matchEndpoints have
// run on the original skeleton.
func findJunctionClusters(p *pixbuf) (resolvable []junctionCluster, unresolvable []image.Point) {
	isJunc := func(x, y int) bool {
		return p.buf[y*p.w+x] > 0 && p.sameNeighbors(x, y) > 2
	}

	// Step 1: enumerate junction pixels.
	var juncList []image.Point
	for y := 1; y < p.h-1; y++ {
		for x := 1; x < p.w-1; x++ {
			if isJunc(x, y) {
				juncList = append(juncList, image.Point{X: x, Y: y})
			}
		}
	}
	if len(juncList) == 0 {
		return nil, nil
	}
	juncIdxOf := make(map[int]int, len(juncList))
	for i, pt := range juncList {
		juncIdxOf[pt.Y*p.w+pt.X] = i
	}

	// Step 2: union-find over junctions. Two junctions merge if their
	// skeleton-path distance is within junctionMergeDist; this also
	// covers 8-adjacent junctions (distance 1) so the old 8-connected
	// case is subsumed.
	parent := make([]int, len(juncList))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}
	for ji, jpt := range juncList {
		seen := make(map[int]bool)
		startIdx := jpt.Y*p.w + jpt.X
		seen[startIdx] = true
		type bfsItem struct{ idx, d int }
		queue := []bfsItem{{startIdx, 0}}
		for h := 0; h < len(queue); h++ {
			cur := queue[h]
			if cur.d >= junctionMergeDist {
				continue
			}
			cx, cy := cur.idx%p.w, cur.idx/p.w
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx, ny := cx+dx, cy+dy
					if nx < 0 || ny < 0 || nx >= p.w || ny >= p.h {
						continue
					}
					nidx := ny*p.w + nx
					if p.buf[nidx] <= 0 || seen[nidx] {
						continue
					}
					seen[nidx] = true
					if jj, ok := juncIdxOf[nidx]; ok && jj != ji {
						union(ji, jj)
					}
					queue = append(queue, bfsItem{nidx, cur.d + 1})
				}
			}
		}
	}

	// Step 3: group junctions by union-find root.
	groups := make(map[int][]int)
	for i := range juncList {
		r := find(i)
		groups[r] = append(groups[r], i)
	}

	// Step 4: For each group, build the cluster pixel set as the
	// junctions plus the shortest skeleton path between every pair of
	// merged junctions, then derive centroid and exits.
	for _, members := range groups {
		clusterIdx := make(map[int]image.Point)
		for _, mi := range members {
			pt := juncList[mi]
			clusterIdx[pt.Y*p.w+pt.X] = pt
		}
		for i := 0; i < len(members); i++ {
			for j := i + 1; j < len(members); j++ {
				path := bfsShortestPath(p, juncList[members[i]], juncList[members[j]], junctionMergeDist)
				for _, pt := range path {
					clusterIdx[pt.Y*p.w+pt.X] = pt
				}
			}
		}
		// Absorb cells whose only skeleton neighbors are already inside
		// the cluster — thinning a sharp ~90° corner often leaves a tiny
		// 2-pixel diagonal junction with a single bridge cell between
		// the diagonal pair. That bridge cell looks like a third "exit"
		// even though it goes nowhere outside the cluster. Iterate until
		// stable.
		var exitMap map[int]image.Point
		for {
			exitMap = make(map[int]image.Point)
			for _, pt := range clusterIdx {
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if dx == 0 && dy == 0 {
							continue
						}
						nx, ny := pt.X+dx, pt.Y+dy
						if nx < 0 || ny < 0 || nx >= p.w || ny >= p.h {
							continue
						}
						nidx := ny*p.w + nx
						if _, in := clusterIdx[nidx]; in {
							continue
						}
						if p.buf[nidx] > 0 {
							exitMap[nidx] = image.Point{X: nx, Y: ny}
						}
					}
				}
			}
			absorbed := false
			for nidx, pt := range exitMap {
				outCount := 0
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if dx == 0 && dy == 0 {
							continue
						}
						nx, ny := pt.X+dx, pt.Y+dy
						if nx < 0 || ny < 0 || nx >= p.w || ny >= p.h {
							continue
						}
						mIdx := ny*p.w + nx
						if p.buf[mIdx] <= 0 {
							continue
						}
						if _, in := clusterIdx[mIdx]; in {
							continue
						}
						outCount++
					}
				}
				if outCount == 0 {
					clusterIdx[nidx] = pt
					absorbed = true
				}
			}
			if !absorbed {
				break
			}
		}
		var pix []image.Point
		for _, pt := range clusterIdx {
			pix = append(pix, pt)
		}
		// Clusters with ≤ 2 exits are not real junctions — they are
		// thinning artifacts (e.g. a sharp corner) the walker can
		// traverse as normal skeleton path. Leave them in p; they
		// neither block convert nor splice in as crossings.
		if len(exitMap) <= 2 {
			continue
		}
		if len(exitMap) != 4 {
			unresolvable = append(unresolvable, pix...)
			continue
		}
		var sumX, sumY float64
		for _, pt := range pix {
			sumX += float64(pt.X)
			sumY += float64(pt.Y)
		}
		cl := junctionCluster{
			pixels:   pix,
			centroid: fPoint{sumX / float64(len(pix)), sumY / float64(len(pix))},
		}
		for _, ex := range exitMap {
			cl.exits = append(cl.exits, ex)
		}
		resolvable = append(resolvable, cl)
	}
	return resolvable, unresolvable
}

// relaxFusedJunctions erases each cluster pixel in `pix` plus a short
// stretch of every arm leading out of it, turning a "fused" Y-junction
// (a strand grazing another strand) into a clean gap that the standard
// endpoint matcher can bridge across an over-strand. Returns true when
// at least one pixel was erased.
//
// The walked stretch length is deliberately small (a few pixels): wide
// enough to disambiguate the touch-point geometry, narrow enough that
// the resulting endpoints stay close to the original junction so the
// later under-strand match doesn't drift. Pixels that already lie on a
// (now-disconnected) endpoint are not extended further — that endpoint
// is already where matchEndpoints needs it.
func relaxFusedJunctions(p *pixbuf, pix []image.Point) bool {
	if len(pix) == 0 {
		return false
	}
	const armErase = 3
	cluster := make(map[int]bool, len(pix))
	for _, pt := range pix {
		cluster[pt.Y*p.w+pt.X] = true
	}
	// Collect arm-tip exits from cluster: skeleton pixels adjacent to
	// (but not inside) the cluster.
	type seed struct {
		x, y int
	}
	seeds := make([]seed, 0, 4)
	seenSeed := make(map[int]bool)
	for _, pt := range pix {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				nx, ny := pt.X+dx, pt.Y+dy
				if nx < 0 || ny < 0 || nx >= p.w || ny >= p.h {
					continue
				}
				nidx := ny*p.w + nx
				if cluster[nidx] {
					continue
				}
				if p.buf[nidx] <= 0 {
					continue
				}
				if seenSeed[nidx] {
					continue
				}
				seenSeed[nidx] = true
				seeds = append(seeds, seed{nx, ny})
			}
		}
	}
	// Erase the cluster pixels themselves.
	for _, pt := range pix {
		p.buf[pt.Y*p.w+pt.X] = 0
	}
	// Walk each arm armErase pixels outward and erase. Stop early when
	// the path branches (we only want to nibble back one strand).
	for _, s := range seeds {
		x, y := s.x, s.y
		for step := 0; step < armErase; step++ {
			if p.buf[y*p.w+x] <= 0 {
				break
			}
			p.buf[y*p.w+x] = 0
			// Find the next skeleton neighbor (there should be at most
			// one once the cluster is erased; if more, we've hit a real
			// junction and stop).
			nextX, nextY := -1, -1
			candidates := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx, ny := x+dx, y+dy
					if nx < 0 || ny < 0 || nx >= p.w || ny >= p.h {
						continue
					}
					if p.buf[ny*p.w+nx] > 0 {
						nextX, nextY = nx, ny
						candidates++
					}
				}
			}
			if candidates != 1 {
				break
			}
			x, y = nextX, nextY
		}
	}
	return true
}

// bfsShortestPath returns the shortest 8-connected skeleton path from
// src to dst (inclusive of both endpoints), bounded by maxDist hops.
// Returns nil if no path exists within the bound. Used to identify the
// bridging skeleton pixels that join two junction clusters in a bowtie
// pattern, so those pixels can be absorbed into the merged cluster.
func bfsShortestPath(p *pixbuf, src, dst image.Point, maxDist int) []image.Point {
	type node struct{ idx, parent int }
	startIdx := src.Y*p.w + src.X
	dstIdx := dst.Y*p.w + dst.X
	if startIdx == dstIdx {
		return []image.Point{src}
	}
	nodes := []node{{startIdx, -1}}
	dist := map[int]int{startIdx: 0}
	for h := 0; h < len(nodes); h++ {
		cur := nodes[h]
		d := dist[cur.idx]
		if d >= maxDist {
			continue
		}
		cx, cy := cur.idx%p.w, cur.idx/p.w
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				nx, ny := cx+dx, cy+dy
				if nx < 0 || ny < 0 || nx >= p.w || ny >= p.h {
					continue
				}
				nidx := ny*p.w + nx
				if p.buf[nidx] <= 0 {
					continue
				}
				if _, seen := dist[nidx]; seen {
					continue
				}
				dist[nidx] = d + 1
				nodes = append(nodes, node{nidx, h})
				if nidx == dstIdx {
					var path []image.Point
					for ci := len(nodes) - 1; ci >= 0; ci = nodes[ci].parent {
						idx := nodes[ci].idx
						path = append(path, image.Point{X: idx % p.w, Y: idx / p.w})
					}
					return path
				}
			}
		}
	}
	return nil
}

// convertImage runs the full knotfolio-style pipeline on img and returns
// a polyline-level Diagram. The algorithm expects a clean knot diagram
// with visual gaps at under-crossings (KnotInfo Diagram / Snappy styles
// work; grid diagrams do not).
func convertImage(img image.Image) (*Diagram, error) {
	if img == nil {
		return nil, fmt.Errorf("nil image")
	}
	p := binaryFromImage(img)
	p.thin()
	p.deleteSpurs()
	clusters, unresolvable := findJunctionClusters(p)
	if len(unresolvable) > 0 {
		// Try to recover: a 3-exit Y typically arises when a
		// rendering artifact makes one strand graze another (so the
		// expected gap collapsed). Erase the junction pixel and a
		// short section of each arm, then re-thin so the resulting
		// endpoints sit a few pixels apart — matchEndpoints can then
		// bridge them across the over-strand like any other under-
		// crossing. If the recovery doesn't help (for example a real
		// degree-3 vertex), fall back to the existing error.
		erased := relaxFusedJunctions(p, unresolvable)
		if !erased {
			return nil, &FusedJunctionsError{Junctions: unresolvable}
		}
		p.deleteSpurs()
		clusters, unresolvable = findJunctionClusters(p)
		if len(unresolvable) > 0 {
			return nil, &FusedJunctionsError{Junctions: unresolvable}
		}
	}

	thick := p.clone()
	thick.thicken()

	eps := p.findEndpoints()
	if len(eps)%2 != 0 {
		return nil, fmt.Errorf("odd endpoint count (%d)", len(eps))
	}

	var matches [][2]image.Point
	if len(eps) > 0 {
		pairs := matchEndpoints(thick, eps)
		if pairs == nil {
			return nil, fmt.Errorf("could not match %d endpoints", len(eps))
		}
		for i := 0; i < len(pairs); i++ {
			ai, aj := fpt(eps[pairs[i][0]]), fpt(eps[pairs[i][1]])
			for j := 0; j < i; j++ {
				bi, bj := fpt(eps[pairs[j][0]]), fpt(eps[pairs[j][1]])
				if _, ok := segmentsIntersect(ai, aj, bi, bj); ok {
					return nil, fmt.Errorf("matched under-strands intersect each other")
				}
			}
		}
		for _, pr := range pairs {
			matches = append(matches, [2]image.Point{eps[pr[0]], eps[pr[1]]})
		}
	}

	// Erase resolved junction clusters from p so the walker sees their
	// exit pixels as fresh endpoints. We do this AFTER findEndpoints /
	// matchEndpoints so the natural under-strand bridge matching isn't
	// fooled by the new exit endpoints. The cluster's centroid vertex
	// and four stub edges are spliced in after walkPath completes.
	for _, cl := range clusters {
		for _, pt := range cl.pixels {
			p.buf[pt.Y*p.w+pt.X] = 0
		}
	}

	var verts []fPoint
	var edges []convEdge
	work := p.clone()
	for y := 1; y < work.h-1; y++ {
		for x := 1; x < work.w-1; x++ {
			if work.buf[y*work.w+x] > 0 && work.sameNeighbors(x, y) == 1 {
				walkPath(work, x, y, &verts, &edges)
			}
		}
	}
	for y := 1; y < work.h-1; y++ {
		for x := 1; x < work.w-1; x++ {
			if work.buf[y*work.w+x] > 0 {
				walkPath(work, x, y, &verts, &edges)
			}
		}
	}

	for _, m := range matches {
		a, b := fpt(m[0]), fpt(m[1])
		va, vb := findVertID(verts, a), findVertID(verts, b)
		if va < 0 || vb < 0 {
			return nil, fmt.Errorf("match endpoint missing from skeleton verts")
		}
		seg := []int{va, vb}
		for vi, v := range verts {
			for i := 0; i+1 < len(seg); i++ {
				if seg[i] == vi || seg[i+1] == vi {
					continue
				}
				if segmentContains(verts[seg[i]], verts[seg[i+1]], v) {
					ns := make([]int, 0, len(seg)+1)
					ns = append(ns, seg[:i+1]...)
					ns = append(ns, vi)
					ns = append(ns, seg[i+1:]...)
					seg = ns
					break
				}
			}
		}
		var newEdges []convEdge
		// Pre-insertion (above) already placed any existing vertex that lies
		// on the match segment into seg. Those become 4-valent when we emit
		// the match edges, so they're already guaranteed crossings — treat
		// that as "produced a crossing" so we don't fall back unnecessarily.
		producedCrossing := len(seg) > 2
		for ei := range edges {
			edge := &edges[ei]
			for i := 0; i+1 < len(seg); i++ {
				pt, ok := segmentsIntersect(verts[edge.v1], verts[edge.v2],
					verts[seg[i]], verts[seg[i+1]])
				if !ok {
					continue
				}
				intID := -1
				ensure := func() int {
					if intID >= 0 {
						return intID
					}
					intID = len(verts)
					verts = append(verts, pt)
					return intID
				}
				if !pt.near(verts[edge.v1]) && !pt.near(verts[edge.v2]) {
					id := ensure()
					newEdges = append(newEdges, convEdge{id, edge.v2, true})
					edge.v2 = id
					producedCrossing = true
				}
				if !pt.near(verts[seg[i]]) && !pt.near(verts[seg[i+1]]) {
					id := ensure()
					ns := make([]int, 0, len(seg)+1)
					ns = append(ns, seg[:i+1]...)
					ns = append(ns, id)
					ns = append(ns, seg[i+1:]...)
					seg = ns
					producedCrossing = true
				}
				break
			}
		}
		if !producedCrossing {
			// Discretization fallback: for very short match segments the
			// straight-line injection can miss the skeleton geometrically
			// even though the endpoints clearly flank an over-strand in the
			// thickened raster. Snap to the nearest over-skeleton edge.
			mid := fPoint{x: (a.x + b.x) / 2, y: (a.y + b.y) / 2}
			bestEi := -1
			bestDist := math.Inf(1)
			var bestPt fPoint
			for ei := range edges {
				if !edges[ei].over {
					continue
				}
				e1, e2 := verts[edges[ei].v1], verts[edges[ei].v2]
				if d := segmentDistance(e1, e2, mid); d < bestDist {
					bestDist = d
					bestEi = ei
					bestPt = projectOntoSegment(e1, e2, mid)
				}
			}
			tolerance := float64(diagramLineWidth * 2)
			if bestEi >= 0 && bestDist <= tolerance {
				e1pt := verts[edges[bestEi].v1]
				e2pt := verts[edges[bestEi].v2]
				var crossID int
				switch {
				case bestPt.near(e1pt):
					crossID = edges[bestEi].v1
				case bestPt.near(e2pt):
					crossID = edges[bestEi].v2
				default:
					crossID = len(verts)
					verts = append(verts, bestPt)
					newEdges = append(newEdges, convEdge{crossID, edges[bestEi].v2, true})
					edges[bestEi].v2 = crossID
				}
				insertAt := len(seg) / 2
				ns := make([]int, 0, len(seg)+1)
				ns = append(ns, seg[:insertAt]...)
				ns = append(ns, crossID)
				ns = append(ns, seg[insertAt:]...)
				seg = ns
			}
		}
		edges = append(edges, newEdges...)
		for i := 0; i+1 < len(seg); i++ {
			edges = append(edges, convEdge{seg[i], seg[i+1], false})
		}
	}

	// Splice each resolved junction cluster as a single 4-valent vertex:
	// add the centroid as a new vertex, then add four straight stub edges
	// to the four exit-endpoints that walkPath produced. The stubs are
	// emitted with over=true so all four darts at the centroid carry
	// Endpoint.Over=true — the switch tool detects that 0/4 imbalance as
	// "ambiguous" and lets the user randomly assign the over-strand on
	// first click.
	for _, cl := range clusters {
		centID := len(verts)
		verts = append(verts, cl.centroid)
		for _, ex := range cl.exits {
			exID := findVertID(verts, fpt(ex))
			if exID < 0 {
				return nil, fmt.Errorf("ambiguous-crossing exit (%d,%d) missing from skeleton verts", ex.X, ex.Y)
			}
			edges = append(edges, convEdge{centID, exID, true})
		}
	}

	adj := make([][]int, len(verts))
	var bad []image.Point
	for i, e := range edges {
		if e.v1 == e.v2 {
			bad = append(bad, verts[e.v1].toImage())
		}
		adj[e.v1] = append(adj[e.v1], i+1)
		adj[e.v2] = append(adj[e.v2], -(i + 1))
	}
	if len(bad) > 0 {
		return nil, &BadTopologyError{Locations: bad, Reason: "self-loop edge"}
	}
	// Validate degrees: the arc walker assumes every vertex is either an
	// endpoint (1), a path point (2), or a 4-valent crossing. A vertex
	// with any other degree means upstream produced a malformed graph
	// (e.g. two match edges snapping to the same existing vertex), and
	// the walker would loop forever bouncing between its darts.
	for vi, list := range adj {
		switch len(list) {
		case 1, 2, 4:
		default:
			bad = append(bad, verts[vi].toImage())
		}
	}
	if len(bad) > 0 {
		return nil, &BadTopologyError{Locations: bad, Reason: "vertex degree not in {1,2,4}"}
	}

	crossOf := make(map[int]int)
	var crossings []image.Point
	for vi, list := range adj {
		if len(list) == 4 {
			crossOf[vi] = len(crossings)
			crossings = append(crossings, verts[vi].toImage())
		}
	}

	dartEnd := func(dart int) int {
		if dart > 0 {
			return edges[dart-1].v2
		}
		return edges[-dart-1].v1
	}
	usedEdge := make([]bool, len(edges))
	var arcs []Arc
	for vi, ci := range crossOf {
		for _, startDart := range adj[vi] {
			ei := absi(startDart) - 1
			if usedEdge[ei] {
				continue
			}
			startOver := edges[ei].over
			poly := []image.Point{verts[vi].toImage()}
			dart := startDart
			var lastEi int
			for {
				lastEi = absi(dart) - 1
				usedEdge[lastEi] = true
				end := dartEnd(dart)
				poly = append(poly, verts[end].toImage())
				if endCi, isCrossing := crossOf[end]; isCrossing {
					arcs = append(arcs, Arc{
						Polyline: poly,
						Start:    Endpoint{Crossing: ci, Over: startOver},
						End:      Endpoint{Crossing: endCi, Over: edges[lastEi].over},
					})
					break
				}
				next := 0
				for _, d := range adj[end] {
					if absi(d) != absi(dart) {
						next = d
						break
					}
				}
				if next == 0 {
					break
				}
				dart = next
			}
		}
	}

	return &Diagram{Crossings: crossings, Arcs: arcs}, nil
}
