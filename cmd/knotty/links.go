package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/fengttt/knotty/knot"
)

// linkRecord matches one entry in dataset/links/links.json: the name,
// the planar-diagram code (each crossing is [a,b,c,d]), the Gauss code
// as one list-of-signed-ints per component, and Jones / HOMFLY as
// LaTeX-string polynomials.
type linkRecord struct {
	Name   string  `json:"name"`
	PD     [][]int `json:"pd"`
	Gauss  [][]int `json:"gauss"`
	Jones  string  `json:"jones"`
	HOMFLY string  `json:"homfly"`
}

const linksDatasetDir = "dataset/links"

var (
	linksOnce  sync.Once
	linksByKey map[string]*linkRecord
	linksErr   error
)

func loadLinksDataset() (map[string]*linkRecord, error) {
	linksOnce.Do(func() {
		path := filepath.Join(linksDatasetDir, "links.json")
		data, err := os.ReadFile(path)
		if err != nil {
			linksErr = fmt.Errorf("read %s: %w", path, err)
			return
		}
		var recs []linkRecord
		if err := json.Unmarshal(data, &recs); err != nil {
			linksErr = fmt.Errorf("parse %s: %w", path, err)
			return
		}
		m := make(map[string]*linkRecord, len(recs))
		for i := range recs {
			r := &recs[i]
			m[r.Name] = r
		}
		linksByKey = m
	})
	return linksByKey, linksErr
}

// findLinkByName looks up a link by exact name in the loaded
// dataset/links/links.json table.
func findLinkByName(name string) (*linkRecord, error) {
	m, err := loadLinksDataset()
	if err != nil {
		return nil, err
	}
	r, ok := m[name]
	if !ok {
		return nil, fmt.Errorf("link %q not found in %s/links.json", name, linksDatasetDir)
	}
	return r, nil
}

// loadLink loads a Thistlethwaite-table link by name (e.g. "L2a1"):
// blits dataset/links/<name>.gif onto the canvas and shows the
// link's PD / Gauss / Jones / HOMFLY in the properties area.
func (g *game) loadLink(name string) {
	rec, err := findLinkByName(name)
	if err != nil {
		g.nameLabel.Label = name + " (not found)"
		g.propsArea.SetText(err.Error() + "\n")
		return
	}
	gifPath := filepath.Join(linksDatasetDir, name+".gif")
	data, err := os.ReadFile(gifPath)
	if err != nil {
		g.nameLabel.Label = name + " (image missing)"
		g.propsArea.SetText(fmt.Sprintf("loadLink: %v\n", err))
		return
	}
	img, _, err := decodeKnotImage(data, knot.GIF)
	if err != nil {
		g.propsArea.SetText(fmt.Sprintf("loadLink decode: %v\n", err))
		return
	}
	g.currentKnot = nil
	g.blitKnotOnCanvas(img)
	g.imageWidget.DebugCrossings = nil
	g.imageWidget.DebugArcs = nil
	g.imageWidget.DebugJunctions = nil
	g.imageWidget.Diagram = nil
	g.pendingAttach = true
	g.nameLabel.Label = name
	g.input.SetText(name)
	g.propsArea.SetText(formatLinkProps(rec))
}

func formatLinkProps(r *linkRecord) string {
	var b []byte
	b = append(b, "name:\n"...)
	b = append(b, r.Name...)
	b = append(b, "\n\n"...)
	b = append(b, "PD:\n"...)
	for i, c := range r.PD {
		if i > 0 {
			b = append(b, ' ')
		}
		b = append(b, fmt.Sprintf("X[%d,%d,%d,%d]", c[0], c[1], c[2], c[3])...)
	}
	b = append(b, "\n\n"...)
	b = append(b, "Gauss code:\n"...)
	for i, comp := range r.Gauss {
		if i > 0 {
			b = append(b, ", "...)
		}
		b = append(b, '{')
		for j, n := range comp {
			if j > 0 {
				b = append(b, ", "...)
			}
			b = append(b, fmt.Sprintf("%d", n)...)
		}
		b = append(b, '}')
	}
	b = append(b, "\n\n"...)
	b = append(b, "Jones polynomial (V):\n"...)
	b = append(b, r.Jones...)
	b = append(b, "\n\n"...)
	b = append(b, "HOMFLY-PT polynomial (P):\n"...)
	b = append(b, r.HOMFLY...)
	b = append(b, '\n')
	return string(b)
}
