package knot

import "github.com/fengttt/knotty/knotdb"

// FindKnotByName looks up the knot with the given name in the current
// knot_info table (see knotdb.SetPath) and returns a populated *Knot.
// Returns knotdb.ErrKnotNotFound if no such row exists.
func FindKnotByName(name string) (*Knot, error) {
	cols, vals, err := knotdb.FindKnotRow(name)
	if err != nil {
		return nil, err
	}
	return NewFromRow(cols, vals)
}
