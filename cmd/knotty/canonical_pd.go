package main

// Canonical PD form.
//
// A PD code with c crossings and 2c arcs depends on two arbitrary
// labelling choices: which arc is "1" (and which way the walk goes
// around the strand) and which adj position is "first" within each
// 4-tuple. There are 4c (crossing, exit-position) starting darts;
// each one fully determines the entire numbering when we walk the
// strand and assign arc IDs 1, 2, 3, ... in encounter order, then
// rotate every crossing's 4-tuple so the arc that the walk first
// touched at that crossing sits at position 0.
//
// The canonical form is the lex-smallest of these 4c candidates,
// where lex compares the flattened 4*c integers first by absolute
// value (so high arc-IDs sort after low ones regardless of sign)
// and uses signed value as the tiebreaker for entries with equal
// absolute value (so a negative entry sorts before its positive
// counterpart). For positive-only PD codes (the format used by
// KnotInfo and by our own dartGraph.PD()) the sign tiebreaker is
// inert; the rule is included for resilience against dataset
// variants that encode signs.
//
// Two PD codes describe the same unoriented diagram iff their
// canonical forms are identical, so canonical PD is the right
// comparison key for testing the convert→PD pipeline against the
// knot database.

// canonicalPD returns the canonical (lex-smallest) PD code over all
// (start_crossing, exit_position) choices. Input is a PD with each
// arc id appearing exactly twice across the entries.
//
// Returns nil if pd is empty.
func canonicalPD(pd [][4]int) [][4]int {
	c := len(pd)
	if c == 0 {
		return nil
	}
	arcLocs := indexArcs(pd)

	var best [][4]int
	for v0 := 0; v0 < c; v0++ {
		for pos0 := 0; pos0 < 4; pos0++ {
			cand := pdFromWalk(pd, arcLocs, v0, pos0)
			if cand == nil {
				continue
			}
			if best == nil || lessPDAbsThenSign(cand, best) {
				best = cand
			}
		}
	}
	return best
}

// indexArcs builds arcID → list of (crossing, position) occurrences.
// Every arcID in a well-formed PD has exactly 2 occurrences (an arc
// has two endpoints in the diagram). When an arc id appears more or
// fewer times indexArcs still records what it sees; pdFromWalk
// returns nil if the walk later runs into a malformed entry.
func indexArcs(pd [][4]int) map[int][][2]int {
	out := make(map[int][][2]int)
	for v, e := range pd {
		for i, a := range e {
			out[a] = append(out[a], [2]int{v, i})
		}
	}
	return out
}

// pdFromWalk simulates the strand traversal starting at the dart
// "exiting v0 at adj position pos0". As the walk passes through the
// 2c arcs of the diagram (in 2c steps), arc IDs are renumbered in
// encounter order; crossings are placed in the order their first
// visit happens; and within each crossing's 4-tuple we rotate so
// the position at which the walk first touched the crossing sits at
// index 0.
//
// The walk is deterministic given the start: at any state (v, exit)
// the arc going out is pd[v][exit]; we find that arc's other
// occurrence (v', enter) and step through the crossing v' from
// enter to (enter + 2) mod 4 (the diagonally-opposite same-strand
// dart). Loop until the start state recurs or 4c iterations elapse
// (a safety bound; a connected single-component knot returns to the
// start in 2c steps).
func pdFromWalk(pd [][4]int, arcLocs map[int][][2]int, v0, pos0 int) [][4]int {
	c := len(pd)
	type entry struct {
		order  int
		entryP int // adj position at which the walk first entered this v
	}
	crossingEntry := make(map[int]entry, c)
	arcRenumber := make(map[int]int)

	v, pos := v0, pos0
	for step := 0; step < 4*c; step++ {
		arc := pd[v][pos]
		if _, seen := arcRenumber[arc]; !seen {
			arcRenumber[arc] = len(arcRenumber) + 1
		}
		// Find the other occurrence of this arc.
		locs := arcLocs[arc]
		var nextV, nextPos int
		switch {
		case len(locs) == 2:
			if locs[0][0] == v && locs[0][1] == pos {
				nextV, nextPos = locs[1][0], locs[1][1]
			} else if locs[1][0] == v && locs[1][1] == pos {
				nextV, nextPos = locs[0][0], locs[0][1]
			} else {
				return nil
			}
		case len(locs) == 4 && pd[locs[0][0]][locs[0][1]] == arc:
			// Self-loop arc: an arc whose two ends are at the same
			// crossing — both ends recorded with the same (v, pos)
			// duplication isn't expected in well-formed PDs, but
			// guard against degenerate inputs.
			return nil
		default:
			return nil
		}
		// Record first-entry position at the new crossing.
		if _, seen := crossingEntry[nextV]; !seen {
			crossingEntry[nextV] = entry{order: len(crossingEntry), entryP: nextPos}
		}
		v, pos = nextV, (nextPos+2)%4
		if v == v0 && pos == pos0 {
			break
		}
	}
	if len(crossingEntry) != c {
		// Walk didn't reach every crossing — this only happens for
		// multi-component links (where the chosen start_dart's
		// circuit covers just one component). Skip; callers can
		// canonicalize each component separately if needed.
		return nil
	}

	out := make([][4]int, c)
	for cv, info := range crossingEntry {
		var rotated [4]int
		for k := 0; k < 4; k++ {
			rotated[k] = arcRenumber[pd[cv][(info.entryP+k)%4]]
		}
		out[info.order] = rotated
	}
	return out
}

// lessPDAbsThenSign reports whether a < b under the canonical PD
// comparison: lex over the flattened 4c integers, comparing first
// by |x| and tiebreaking by signed value (negatives sort before
// positives at equal absolute value).
//
// Both PDs must have the same shape (same length, every entry
// 4 ints).
func lessPDAbsThenSign(a, b [][4]int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		for j := 0; j < 4; j++ {
			ax, bx := a[i][j], b[i][j]
			aa, bb := absInt(ax), absInt(bx)
			if aa != bb {
				return aa < bb
			}
			if ax != bx {
				return ax < bx
			}
		}
	}
	return len(a) < len(b)
}

// equalPD compares two PDs entry-by-entry, returning true iff every
// integer at every position matches. Used after canonicalization
// where different equivalent inputs are expected to round-trip to
// the same canonical output.
func equalPD(a, b [][4]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		for j := 0; j < 4; j++ {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
