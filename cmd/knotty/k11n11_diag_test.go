package main

import (
	"bytes"
	"errors"
	"image"
	stddraw "image/draw"
	_ "image/gif"
	"os"
	"sort"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	xdraw "golang.org/x/image/draw"
)

// TestK11n11Convert mirrors the canvas pipeline that the UI runs:
// gif → ebiten.Image → blit onto a 540×540 canvas with white fill →
// canvasToImage-style readback → convertImage. The direct decode →
// convertImage path passes; this is the path that fails.
func TestK11n11Convert(t *testing.T) {
	data, err := os.ReadFile("../../saved/k11n11.gif")
	if err != nil {
		t.Skipf("no fixture: %v", err)
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	t.Logf("src bounds: %v", src.Bounds())

	// Reproduce blitKnotOnCanvas: 540x540 white canvas, scale src to fit
	// preserving aspect ratio, draw centered.
	const cw, ch = 540, 540
	canvas := image.NewRGBA(image.Rect(0, 0, cw, ch))
	white := image.NewUniform(image.White)
	stddraw.Draw(canvas, canvas.Bounds(), white, image.Point{}, stddraw.Src)
	ib := src.Bounds()
	iw, ih := ib.Dx(), ib.Dy()
	scale := float64(cw) / float64(iw)
	if s := float64(ch) / float64(ih); s < scale {
		scale = s
	}
	dw := int(float64(iw) * scale)
	dh := int(float64(ih) * scale)
	dst := image.Rect((cw-dw)/2, (ch-dh)/2, (cw+dw)/2, (ch+dh)/2)
	xdraw.BiLinear.Scale(canvas, dst, src, ib, xdraw.Over, nil)

	// Now run convert exactly like the canvas->image readback path would.
	d, err := convertImage(canvas)
	if err == nil {
		t.Logf("convert ok: %d crossings %d arcs", len(d.Crossings), len(d.Arcs))
		return
	}
	var pts []image.Point
	var fje *FusedJunctionsError
	var bte *BadTopologyError
	switch {
	case errors.As(err, &fje):
		pts = append([]image.Point(nil), fje.Junctions...)
	case errors.As(err, &bte):
		pts = append([]image.Point(nil), bte.Locations...)
	default:
		t.Fatalf("unexpected error type: %v", err)
	}
	sort.Slice(pts, func(i, j int) bool {
		if pts[i].Y != pts[j].Y {
			return pts[i].Y < pts[j].Y
		}
		return pts[i].X < pts[j].X
	})
	t.Logf("convert error: %v (%d points)", err, len(pts))
	for _, p := range pts {
		t.Logf("  unresolvable junction px (%d,%d)", p.X, p.Y)
	}
	pb := pixbufFromImage(canvas)
	resolvable, unresolvable := findJunctionClusters(pb)
	t.Logf("findJunctionClusters: resolvable=%d unresolvable=%d", len(resolvable), len(unresolvable))
	for i, c := range resolvable {
		t.Logf("  R%d centroid=(%.1f,%.1f) pixels=%d exits=%d",
			i, c.centroid.x, c.centroid.y, len(c.pixels), len(c.exits))
	}
	if len(unresolvable) > 0 {
		groupAndDump(pb, unresolvable, t)
	}
	_ = ebiten.Image{}
}

func pixbufFromImage(img image.Image) *pixbuf {
	p := binaryFromImage(img)
	p.thin()
	p.deleteSpurs()
	return p
}

func groupAndDump(p *pixbuf, pts []image.Point, t *testing.T) {
	in := make(map[int]bool, len(pts))
	for _, pt := range pts {
		in[pt.Y*p.w+pt.X] = true
	}
	visited := make(map[int]bool, len(pts))
	clusterID := 0
	for _, start := range pts {
		sidx := start.Y*p.w + start.X
		if visited[sidx] {
			continue
		}
		queue := []image.Point{start}
		visited[sidx] = true
		var members []image.Point
		exits := make(map[int]image.Point)
		for h := 0; h < len(queue); h++ {
			cur := queue[h]
			members = append(members, cur)
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx, ny := cur.X+dx, cur.Y+dy
					if nx < 0 || ny < 0 || nx >= p.w || ny >= p.h {
						continue
					}
					nidx := ny*p.w + nx
					if in[nidx] {
						if !visited[nidx] {
							visited[nidx] = true
							queue = append(queue, image.Point{X: nx, Y: ny})
						}
					} else if p.buf[nidx] > 0 {
						exits[nidx] = image.Point{X: nx, Y: ny}
					}
				}
			}
		}
		t.Logf("  unres group %d: pixels=%d exits=%d", clusterID, len(members), len(exits))
		for _, m := range members {
			t.Logf("    px (%d,%d)", m.X, m.Y)
		}
		dumpSkeletonContext(p, members[0], 20, t)
		clusterID++
	}
}

func dumpSkeletonContext(p *pixbuf, ctr image.Point, r int, t *testing.T) {
	for dy := -r; dy <= r; dy++ {
		row := make([]byte, 0, 2*r+1)
		for dx := -r; dx <= r; dx++ {
			nx, ny := ctr.X+dx, ctr.Y+dy
			if nx < 0 || ny < 0 || nx >= p.w || ny >= p.h {
				row = append(row, ' ')
				continue
			}
			if p.buf[ny*p.w+nx] > 0 {
				if dx == 0 && dy == 0 {
					row = append(row, '@')
				} else {
					row = append(row, '#')
				}
			} else {
				row = append(row, '.')
			}
		}
		t.Logf("    %s", string(row))
	}
}
