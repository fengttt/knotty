package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Kauffman bracket and Jones polynomial.
//
// The Kauffman bracket <K> is a Laurent polynomial in A computed by a
// state-sum over all 2^c smoothings of the c crossings:
//
//   <K> = sum_{state s} A^(numA - numB) * delta^(loops(s) - 1)
//
// where delta = -A^2 - A^-2, and at each crossing the state picks
// either the A-smoothing (joins adj[0]-adj[1] and adj[2]-adj[3]) or
// the B-smoothing (joins adj[1]-adj[2] and adj[3]-adj[0]).
//
// The Jones polynomial in t is then
//
//   V_K(t) = ((-A)^(-3w) * <K>) | A = t^(-1/4)
//
// where w is the writhe (sum of crossing signs). For a single-
// component knot every exponent in (-A)^(-3w) * <K> is divisible by 4,
// so the substitution lands in integer powers of t.
//
// Writhe needs an orientation. Any orientation works for a knot —
// reversing it doesn't change V_K — so we use the orientation
// implicit in dartCircuit (which walks each component once, assigning
// directions to all darts).

// poly is a sparse Laurent polynomial: exponent → coefficient. Used
// for both bracket polynomials in A and Jones polynomials in t.
type poly map[int]int

// bracketFromPD computes the Kauffman bracket polynomial of a
// diagram given its PD code in adj-CCW order (under at 0/2, over at
// 1/3). Pure function — no dart graph needed.
//
// At each crossing entry pd[i] = [w, x, y, z]:
//   - A-smoothing pairs (w, x) and (y, z)
//   - B-smoothing pairs (x, y) and (z, w)
//
// We walk all 2^c smoothing assignments, count loops via union-find
// over the 2c arc IDs, and add the state's contribution to the
// running sum.
func bracketFromPD(pd [][4]int) poly {
	c := len(pd)
	out := poly{}
	if c == 0 {
		out[0] = 1
		return out
	}
	// Arc IDs are 1..2c; pre-size union-find for those.
	maxID := 0
	for _, e := range pd {
		for _, x := range e {
			if x > maxID {
				maxID = x
			}
		}
	}
	parent := make([]int, maxID+1)
	for state := 0; state < (1 << c); state++ {
		for i := 0; i <= maxID; i++ {
			parent[i] = i
		}
		find := func(x int) int {
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
		numA := 0
		for i, e := range pd {
			if (state>>i)&1 == 1 {
				numA++
				union(e[0], e[1])
				union(e[2], e[3])
			} else {
				union(e[1], e[2])
				union(e[3], e[0])
			}
		}
		numB := c - numA
		seen := make(map[int]bool, maxID)
		for i := 1; i <= maxID; i++ {
			seen[find(i)] = true
		}
		loops := len(seen)

		// A^(numA - numB) * delta^(loops - 1)
		addDeltaPower(out, numA-numB, loops-1)
	}
	for k, v := range out {
		if v == 0 {
			delete(out, k)
		}
	}
	return out
}

// bracket is the dart-graph wrapper around bracketFromPD.
func (g *dartGraph) bracket() poly {
	return bracketFromPD(g.PD())
}

// addDeltaPower adds A^shift * (-A^2 - A^-2)^k to p.
//
// Expansion: (-A^2 - A^-2)^k = sum_{i=0..k} C(k,i) * (-1)^k * A^(4i - 2k)
// (the (-1)^i from (-A^2)^i and the (-1)^(k-i) from (-A^-2)^(k-i)
// combine to (-1)^k regardless of i).
func addDeltaPower(p poly, shift, k int) {
	if k < 0 {
		return
	}
	if k == 0 {
		p[shift]++
		return
	}
	sign := 1
	if k%2 == 1 {
		sign = -1
	}
	binom := 1
	for i := 0; i <= k; i++ {
		exp := 4*i - 2*k + shift
		p[exp] += sign * binom
		// Next binomial coefficient by recurrence:
		//   C(k, i+1) = C(k, i) * (k - i) / (i + 1)
		binom = binom * (k - i) / (i + 1)
	}
}

// writhe returns the sum of crossing signs.
//
// At each crossing the under and over strands each have one
// "incoming" dart (negative-signed at this vertex; the dart points
// back toward the strand's earlier position) and one "outgoing"
// dart. With CCW adj order placing under at positions 0/2 and over
// at positions 1/3, the cross product of the two strand directions
// is +1 iff over_in_pos == (under_in_pos + 3) mod 4, else -1.
//
// Rationale: position the crossing so adj[k] points at angle
// k*90° CCW. Then under-strand direction is from under_in's
// position toward the diametrically-opposite under_out (the strand
// passes straight through). Over-strand direction is the same idea
// for positions 1/3. Working out the four cases gives the formula
// above.
func (g *dartGraph) writhe() int {
	w := 0
	for _, ad := range g.adj {
		underIn, overIn := -1, -1
		for i, d := range ad {
			if d < 0 {
				if i%2 == 0 {
					underIn = i
				} else {
					overIn = i
				}
			}
		}
		if underIn < 0 || overIn < 0 {
			continue
		}
		if overIn == (underIn+3)%4 {
			w++
		} else {
			w--
		}
	}
	return w
}

// jonesFromBracketWrithe substitutes A = t^(-1/4) into
// (-A)^(-3w) * <K> to produce the Jones polynomial in t. Pure
// function — used by tests and by the dart-graph wrapper below.
//
// Returns an error if the substitution lands on a non-integer
// exponent of t (which would indicate a malformed bracket /
// writhe pairing for a knot).
func jonesFromBracketWrithe(br poly, w int) (poly, error) {
	// f(A) = (-A)^(-3w) * <K> = (-1)^(-3w) * A^(-3w) * <K>
	//      = (-1)^w * A^(-3w) * <K>     (since -3w ≡ w mod 2)
	sign := 1
	if w%2 != 0 {
		sign = -1
	}
	shift := -3 * w
	jp := poly{}
	for exp, coef := range br {
		if coef == 0 {
			continue
		}
		fExp := exp + shift
		if fExp%4 != 0 {
			return nil, fmt.Errorf("jones: A^%d not divisible by 4 (writhe %d, bracket exp %d)", fExp, w, exp)
		}
		jp[-fExp/4] += sign * coef
	}
	for k, v := range jp {
		if v == 0 {
			delete(jp, k)
		}
	}
	return jp, nil
}

// jones computes the Jones polynomial V_K(t) of a single-component
// knot from its dart graph. Returns an error when the diagram has
// more than one component (the Jones polynomial is defined for
// links too, but we only support knots here).
func (g *dartGraph) jones() (poly, error) {
	if g.NumComponents() != 1 {
		return nil, fmt.Errorf("jones requires a single-component knot, got %d components", g.NumComponents())
	}
	return jonesFromBracketWrithe(g.bracket(), g.writhe())
}

// Jones is the Diagram-level convenience: returns the Jones
// polynomial as a formatted string in t. Errors propagate from
// dartGraph construction or non-knot input.
func (d *Diagram) Jones() (string, error) {
	g, err := newDartGraph(d)
	if err != nil {
		return "", err
	}
	jp, err := g.jones()
	if err != nil {
		return "", err
	}
	return formatPoly(jp, "t"), nil
}

// formatPoly renders p as a Laurent polynomial in v with terms in
// ascending exponent order, e.g. "t^(-2) - t^(-1) + 1 - t + t^2".
func formatPoly(p poly, v string) string {
	if len(p) == 0 {
		return "0"
	}
	keys := make([]int, 0, len(p))
	for k, c := range p {
		if c != 0 {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return "0"
	}
	sort.Ints(keys)
	var b strings.Builder
	for i, k := range keys {
		c := p[k]
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
		b.WriteString(formatTerm(ac, v, k))
	}
	return b.String()
}

// formatTerm formats a single positive-coefficient term as
// "[coef][var[^exp]]" with the standard Laurent conventions:
// no coefficient when it's 1 (and exponent isn't 0), no variable
// when exp is 0, "v" alone when exp is 1, "v^(neg)" with parens
// for negative exponents.
func formatTerm(absCoef int, v string, exp int) string {
	var coefStr string
	if absCoef != 1 || exp == 0 {
		coefStr = fmt.Sprint(absCoef)
	}
	switch {
	case exp == 0:
		return fmt.Sprint(absCoef)
	case exp == 1:
		return coefStr + v
	case exp > 0:
		return fmt.Sprintf("%s%s^%d", coefStr, v, exp)
	default:
		return fmt.Sprintf("%s%s^(%d)", coefStr, v, exp)
	}
}

// formatPD renders a PD code as a comma-separated sequence of
// X[a,b,c,d] entries, matching the conventional notation seen in
// the dataset and KnotInfo.
func formatPD(pd [][4]int) string {
	parts := make([]string, len(pd))
	for i, e := range pd {
		parts[i] = fmt.Sprintf("X[%d,%d,%d,%d]", e[0], e[1], e[2], e[3])
	}
	return strings.Join(parts, ", ")
}

// parseJonesPoly parses a Jones polynomial in the loose KnotInfo
// format — terms separated by + or -, each term `[<coef>][t[^<exp>]]`
// where exp is either a bare positive integer (`t^3`) or
// parenthesized for negative or signed values (`t^(-3)`).
// Whitespace is ignored. Empty input parses to the zero polynomial.
//
// Used by the wide knot-database comparison test to put dataset
// strings into the same poly representation our pipeline produces,
// so structural comparison (with optional mirror) is straightforward.
func parseJonesPoly(s string) (poly, error) {
	clean := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, s)
	out := poly{}
	if clean == "" {
		return out, nil
	}
	i := 0
	for i < len(clean) {
		sign := 1
		switch clean[i] {
		case '+':
			i++
		case '-':
			sign = -1
			i++
		}
		// Optional unsigned coefficient.
		coefStart := i
		for i < len(clean) && clean[i] >= '0' && clean[i] <= '9' {
			i++
		}
		coef := 1
		hadCoef := i > coefStart
		if hadCoef {
			n, err := strconv.Atoi(clean[coefStart:i])
			if err != nil {
				return nil, fmt.Errorf("parseJonesPoly: bad coef at %d: %v", coefStart, err)
			}
			coef = n
		}
		// Optional variable t with optional exponent.
		exp := 0
		hadVar := false
		if i < len(clean) && clean[i] == 't' {
			hadVar = true
			i++
			exp = 1
			if i < len(clean) && clean[i] == '^' {
				i++
				if i < len(clean) && clean[i] == '(' {
					end := strings.IndexByte(clean[i:], ')')
					if end < 0 {
						return nil, fmt.Errorf("parseJonesPoly: unmatched '(' at %d", i)
					}
					n, err := strconv.Atoi(clean[i+1 : i+end])
					if err != nil {
						return nil, fmt.Errorf("parseJonesPoly: bad exp at %d: %v", i, err)
					}
					exp = n
					i += end + 1
				} else {
					expStart := i
					if i < len(clean) && (clean[i] == '+' || clean[i] == '-') {
						i++
					}
					for i < len(clean) && clean[i] >= '0' && clean[i] <= '9' {
						i++
					}
					if i == expStart {
						return nil, fmt.Errorf("parseJonesPoly: missing exp at %d", expStart)
					}
					n, err := strconv.Atoi(clean[expStart:i])
					if err != nil {
						return nil, fmt.Errorf("parseJonesPoly: bad exp at %d: %v", expStart, err)
					}
					exp = n
				}
			}
		}
		if !hadCoef && !hadVar {
			return nil, fmt.Errorf("parseJonesPoly: empty term at %d in %q", i, clean)
		}
		out[exp] += sign * coef
	}
	for k, v := range out {
		if v == 0 {
			delete(out, k)
		}
	}
	return out, nil
}

// mirrorPoly returns p with every exponent negated, i.e. the
// substitution t → 1/t. For knots, V_K(1/t) is the Jones polynomial
// of the mirror knot; the wide DB-comparison test accepts either
// V(t) or V(1/t) since the converted image's chirality is not
// guaranteed to match the dataset's canonical chirality.
func mirrorPoly(p poly) poly {
	out := poly{}
	for k, v := range p {
		out[-k] = v
	}
	return out
}
