package main

import (
	"fmt"
	"sort"
	"strings"
)

// HOMFLY-PT polynomial via the skein-tree algorithm.
//
// Skein relation: a * P(L+) - a^(-1) * P(L-) = z * P(L0)
// P(unknot) = 1, P(n-unlink) = ((a - a^(-1))/z)^(n-1).
//
// The descending-diagram approach:
// 1. Orient and walk the link.
// 2. A crossing is "bad" if first encountered on its under-strand.
// 3. If no bad crossings → descending diagram → unlink → return delta^(n-1).
// 4. Otherwise, at the first bad crossing:
//    - "change" flips over/under WITHOUT changing strand connectivity
//      → makes that crossing good, reducing badness by 1
//    - "smooth" resolves the crossing → reduces crossing count by 1
//    Both strictly decrease (badness, #crossings), guaranteeing termination.

// poly2 is a sparse two-variable Laurent polynomial in (a, z).
type poly2 map[[2]int]int

func (p poly2) add(q poly2) poly2 {
	r := make(poly2, len(p)+len(q))
	for k, v := range p {
		r[k] += v
	}
	for k, v := range q {
		r[k] += v
	}
	for k, v := range r {
		if v == 0 {
			delete(r, k)
		}
	}
	return r
}

func (p poly2) scale(aExp, zExp, coef int) poly2 {
	if coef == 0 {
		return poly2{}
	}
	r := make(poly2, len(p))
	for k, v := range p {
		r[[2]int{k[0] + aExp, k[1] + zExp}] = v * coef
	}
	return r
}

func poly2Mul(p, q poly2) poly2 {
	r := make(poly2, len(p)*len(q))
	for kp, vp := range p {
		for kq, vq := range q {
			key := [2]int{kp[0] + kq[0], kp[1] + kq[1]}
			r[key] += vp * vq
		}
	}
	for k, v := range r {
		if v == 0 {
			delete(r, k)
		}
	}
	return r
}

// hlCrossing stores a crossing.
// Strands are labelled A (ports 0-in, 2-out) and B (ports 1-in, 3-out).
// The walk always follows connectivity: enter at in-port → exit at corresponding out-port.
// "over" tracks which strand goes over: false = B is over (standard PD), true = A is over.
type hlCrossing struct {
	arcs [4]int // strand A: arcs[0]=in, arcs[2]=out; strand B: arcs[1]=in, arcs[3]=out
	sign int
	over bool // true = strand A is over, false = strand B is over
}

type hlDiagram struct {
	crossings []hlCrossing
	numArcs   int
	freeLoops int // number of free loops (components with no crossings)
}

func (d *hlDiagram) clone() *hlDiagram {
	nd := &hlDiagram{
		crossings: make([]hlCrossing, len(d.crossings)),
		numArcs:   d.numArcs,
		freeLoops: d.freeLoops,
	}
	copy(nd.crossings, d.crossings)
	return nd
}

// isUnderPort reports whether arriving at port p of crossing ci is
// arriving on the under-strand.
func (d *hlDiagram) isUnderPort(ci int, port int) bool {
	c := &d.crossings[ci]
	strandA := port%2 == 0 // ports 0,2 are strand A
	if !c.over {
		// B is over → A is under
		return strandA
	}
	// A is over → B is under
	return !strandA
}

// countComponents counts link components by following strand connectivity.
func (d *hlDiagram) countComponents() int {
	if len(d.crossings) == 0 {
		return max(1, d.freeLoops)
	}

	type portRef struct {
		ci   int
		port int
	}
	arcEnd := make(map[int]portRef)
	for ci, c := range d.crossings {
		arcEnd[c.arcs[0]] = portRef{ci, 0}
		arcEnd[c.arcs[1]] = portRef{ci, 1}
	}

	visited := make(map[int]bool)
	comp := 0
	for _, c := range d.crossings {
		for _, startPort := range []int{2, 3} {
			arc := c.arcs[startPort]
			if visited[arc] {
				continue
			}
			comp++
			cur := arc
			for !visited[cur] {
				visited[cur] = true
				ep, ok := arcEnd[cur]
				if !ok {
					break
				}
				var outPort int
				if ep.port == 0 {
					outPort = 2
				} else {
					outPort = 3
				}
				cur = d.crossings[ep.ci].arcs[outPort]
			}
		}
	}
	comp += d.freeLoops
	if comp == 0 {
		return 1
	}
	return comp
}

// walkBadCrossing walks the link and returns the first crossing reached
// on its under-strand, or -1 if the diagram is descending.
func (d *hlDiagram) walkBadCrossing() int {
	if len(d.crossings) == 0 {
		return -1
	}

	type portRef struct {
		ci   int
		port int
	}
	arcEnd := make(map[int]portRef)
	for ci, c := range d.crossings {
		arcEnd[c.arcs[0]] = portRef{ci, 0}
		arcEnd[c.arcs[1]] = portRef{ci, 1}
	}

	visited := make(map[int]bool)
	firstVisit := make(map[int]bool) // set of crossings already first-visited

	for _, c := range d.crossings {
		for _, startPort := range []int{2, 3} {
			arc := c.arcs[startPort]
			if visited[arc] {
				continue
			}
			cur := arc
			for !visited[cur] {
				visited[cur] = true
				ep, ok := arcEnd[cur]
				if !ok {
					break
				}
				if !firstVisit[ep.ci] {
					firstVisit[ep.ci] = true
					if d.isUnderPort(ep.ci, ep.port) {
						return ep.ci
					}
				}
				var outPort int
				if ep.port == 0 {
					outPort = 2
				} else {
					outPort = 3
				}
				cur = d.crossings[ep.ci].arcs[outPort]
			}
		}
	}
	return -1
}

// changeCrossing flips which strand is over at crossing ci.
// Does NOT change connectivity — the walk path stays the same.
func (d *hlDiagram) changeCrossing(ci int) *hlDiagram {
	nd := d.clone()
	nd.crossings[ci].over = !nd.crossings[ci].over
	nd.crossings[ci].sign = -nd.crossings[ci].sign
	return nd
}

// smoothCrossing removes crossing ci via oriented resolution.
// The resolution connects: under_in's source → over_out's dest,
// and over_in's source → under_out's dest.
func (d *hlDiagram) smoothCrossing(ci int) *hlDiagram {
	c := d.crossings[ci]
	var underIn, underOut, overIn, overOut int
	if !c.over {
		underIn, underOut = 0, 2
		overIn, overOut = 1, 3
	} else {
		underIn, underOut = 1, 3
		overIn, overOut = 0, 2
	}

	// Oriented resolution: merge under_in arc with over_out arc,
	// merge over_in arc with under_out arc.
	rep := make(map[int]int)
	extraLoops := 0

	if c.arcs[underIn] == c.arcs[overOut] {
		extraLoops++
	} else {
		surv1 := min(c.arcs[underIn], c.arcs[overOut])
		rep[c.arcs[underIn]] = surv1
		rep[c.arcs[overOut]] = surv1
	}
	if c.arcs[overIn] == c.arcs[underOut] {
		extraLoops++
	} else {
		surv2 := min(c.arcs[overIn], c.arcs[underOut])
		rep[c.arcs[overIn]] = surv2
		rep[c.arcs[underOut]] = surv2
	}

	nd := &hlDiagram{numArcs: d.numArcs, freeLoops: d.freeLoops + extraLoops}
	for i, oc := range d.crossings {
		if i == ci {
			continue
		}
		nc := hlCrossing{sign: oc.sign, over: oc.over}
		for p := 0; p < 4; p++ {
			a := oc.arcs[p]
			if r, ok := rep[a]; ok {
				nc.arcs[p] = r
			} else {
				nc.arcs[p] = a
			}
		}
		nd.crossings = append(nd.crossings, nc)
	}
	return nd
}

// homfly computes P(L) via the descending-diagram skein tree.
func (d *hlDiagram) homfly() poly2 {
	if len(d.crossings) == 0 {
		nc := d.countComponents()
		return unlinkPoly(nc)
	}

	ci := d.walkBadCrossing()
	if ci < 0 {
		nc := d.countComponents()
		return unlinkPoly(nc)
	}

	sign := d.crossings[ci].sign
	changed := d.changeCrossing(ci)
	smoothed := d.smoothCrossing(ci)

	pChanged := changed.homfly()
	pSmoothed := smoothed.homfly()

	// Skein: a*P(L+) - a^(-1)*P(L-) = z*P(L0)
	if sign == 1 {
		// This is L+. P(L+) = a^(-2)*P(L-) + a^(-1)*z*P(L0)
		return pChanged.scale(-2, 0, 1).add(pSmoothed.scale(-1, 1, 1))
	}
	// This is L-. P(L-) = a^2*P(L+) - a*z*P(L0)
	return pChanged.scale(2, 0, 1).add(pSmoothed.scale(1, 1, -1))
}

// unlinkPoly returns P(O_n) = delta^(n-1) where delta = (a - a^(-1))/z.
func unlinkPoly(n int) poly2 {
	if n <= 0 {
		n = 1
	}
	if n == 1 {
		return poly2{[2]int{0, 0}: 1}
	}
	delta := poly2{
		[2]int{1, -1}:  1,
		[2]int{-1, -1}: -1,
	}
	result := delta
	for i := 2; i < n; i++ {
		result = poly2Mul(result, delta)
	}
	return result
}

// homflyFromPD computes the HOMFLY polynomial directly from a PD code.
// PD convention: positions 0,2 = under-strand, positions 1,3 = over-strand, CCW.
func homflyFromPD(pd [][4]int) poly2 {
	c := len(pd)
	if c == 0 {
		return poly2{[2]int{0, 0}: 1}
	}

	maxArc := 0
	for _, e := range pd {
		for _, x := range e {
			if x > maxArc {
				maxArc = x
			}
		}
	}

	type arcPos struct {
		crossing int
		pos      int
	}
	arcPositions := make([][]arcPos, maxArc+1)
	for v, e := range pd {
		for pos, arcID := range e {
			arcPositions[arcID] = append(arcPositions[arcID], arcPos{v, pos})
		}
	}

	// Orient: walk circuits to find in/out direction for each strand.
	type crossingInfo struct {
		underIn int // position on under-strand that is "in" (entering)
		overIn  int // position on over-strand that is "in" (entering)
	}
	info := make([]crossingInfo, c)

	visitedArc := make([]bool, maxArc+1)
	for startArc := 1; startArc <= maxArc; startArc++ {
		if visitedArc[startArc] {
			continue
		}
		first := arcPositions[startArc][0]
		enterCrossing := first.crossing
		enterPos := first.pos
		cur := startArc
		for {
			if visitedArc[cur] {
				break
			}
			visitedArc[cur] = true
			v := enterCrossing
			pos := enterPos
			if pos%2 == 0 {
				info[v].underIn = pos
			} else {
				info[v].overIn = pos
			}
			outPos := (pos + 2) % 4
			nextArc := pd[v][outPos]
			for _, ap := range arcPositions[nextArc] {
				if ap.crossing != v || ap.pos != outPos {
					enterCrossing = ap.crossing
					enterPos = ap.pos
					break
				}
			}
			cur = nextArc
		}
	}

	// Build hlDiagram. In the PD convention, positions 0,2 are the
	// under-strand and 1,3 are the over-strand. We label the under-strand
	// as "A" and over-strand as "B" initially (over=false means B is over,
	// which matches PD convention). Wait — let me be careful:
	//
	// hlCrossing has strand A at ports 0(in),2(out) and strand B at ports 1(in),3(out).
	// over=false means B is over. We want the PD under-strand to map to strand A
	// so that B (the PD over-strand) is over. Then over=false is the default.
	//
	// Strand A = PD under-strand: arcs[0] = under_in arc, arcs[2] = under_out arc
	// Strand B = PD over-strand: arcs[1] = over_in arc, arcs[3] = over_out arc
	d := &hlDiagram{
		crossings: make([]hlCrossing, c),
		numArcs:   maxArc,
	}
	for v := range c {
		ui := info[v].underIn
		oi := info[v].overIn
		uo := (ui + 2) % 4
		oo := (oi + 2) % 4
		d.crossings[v].arcs[0] = pd[v][ui] - 1  // strand A (under) in
		d.crossings[v].arcs[2] = pd[v][uo] - 1  // strand A (under) out
		d.crossings[v].arcs[1] = pd[v][oi] - 1  // strand B (over) in
		d.crossings[v].arcs[3] = pd[v][oo] - 1  // strand B (over) out
		d.crossings[v].over = false              // B is over (standard)
		if oi == (ui+3)%4 {
			d.crossings[v].sign = 1
		} else {
			d.crossings[v].sign = -1
		}
	}
	return d.homfly()
}

// HOMFLY computes the HOMFLY-PT polynomial P(a, z) of a link diagram.
func (dia *Diagram) HOMFLY() (string, error) {
	g, err := newDartGraph(dia)
	if err != nil {
		return "", err
	}
	pd := g.PD()
	p := homflyFromPD(pd)
	return formatPoly2(p, "a", "z"), nil
}

// formatPoly2 renders a two-variable Laurent polynomial with terms
// sorted by first variable exponent (ascending), then second.
func formatPoly2(p poly2, v1, v2 string) string {
	if len(p) == 0 {
		return "0"
	}
	type term struct {
		key  [2]int
		coef int
	}
	var terms []term
	for k, c := range p {
		if c != 0 {
			terms = append(terms, term{k, c})
		}
	}
	if len(terms) == 0 {
		return "0"
	}
	sort.Slice(terms, func(i, j int) bool {
		if terms[i].key[0] != terms[j].key[0] {
			return terms[i].key[0] < terms[j].key[0]
		}
		return terms[i].key[1] < terms[j].key[1]
	})

	var b strings.Builder
	for i, t := range terms {
		c := t.coef
		switch {
		case i == 0 && c < 0:
			b.WriteString("-")
		case i > 0 && c >= 0:
			b.WriteString(" + ")
		case i > 0 && c < 0:
			b.WriteString(" - ")
		}
		ac := c
		if ac < 0 {
			ac = -ac
		}
		b.WriteString(formatTerm2(ac, v1, t.key[0], v2, t.key[1]))
	}
	return b.String()
}

func formatTerm2(absCoef int, v1 string, e1 int, v2 string, e2 int) string {
	var parts []string
	if e1 != 0 {
		parts = append(parts, fmtVarExp(v1, e1))
	}
	if e2 != 0 {
		parts = append(parts, fmtVarExp(v2, e2))
	}
	if len(parts) == 0 {
		return fmt.Sprint(absCoef)
	}
	var coefStr string
	if absCoef != 1 {
		coefStr = fmt.Sprint(absCoef)
	}
	return coefStr + strings.Join(parts, "*")
}

func fmtVarExp(v string, exp int) string {
	switch {
	case exp == 1:
		return v
	case exp > 0:
		return fmt.Sprintf("%s^%d", v, exp)
	default:
		return fmt.Sprintf("%s^(%d)", v, exp)
	}
}
