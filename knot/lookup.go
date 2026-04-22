package knot

import "github.com/fengttt/knotty/knotdb"

// FindKnotByName looks up the knot with the given name in the current
// small knot_info table (see knotdb.SetDir) and returns a populated
// *Diagram. Returns knotdb.ErrKnotNotFound if no such row exists.
func FindKnotByName(name string) (*Diagram, error) {
	cols, vals, err := knotdb.FindKnotRow(name)
	if err != nil {
		return nil, err
	}
	return NewFromRow(cols, vals)
}
