package main

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"

	"github.com/fengttt/knotty/knot"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// rasterSize is the pixel width/height we rasterize SVG to. PNGs are
// used at their native size.
const rasterSize = 600

// decodeKnotImage decodes the raw bytes of a knot image into an
// *ebiten.Image. PNG uses the standard library; SVG is rasterized via
// oksvg + rasterx at rasterSize x rasterSize.
func decodeKnotImage(data []byte, kind knot.ImageKind) (*ebiten.Image, error) {
	if len(data) == 0 {
		return placeholderImage("no image"), nil
	}
	switch kind {
	case knot.PNG:
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decode png: %w", err)
		}
		return ebiten.NewImageFromImage(img), nil
	case knot.SVG:
		icon, err := oksvg.ReadIconStream(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("read svg: %w", err)
		}
		w, h := rasterSize, rasterSize
		icon.SetTarget(0, 0, float64(w), float64(h))
		rgba := image.NewRGBA(image.Rect(0, 0, w, h))
		scanner := rasterx.NewScannerGV(w, h, rgba, rgba.Bounds())
		dasher := rasterx.NewDasher(w, h, scanner)
		icon.Draw(dasher, 1.0)
		return ebiten.NewImageFromImage(rgba), nil
	default:
		return nil, fmt.Errorf("unknown image kind %q", kind)
	}
}

// placeholderImage returns a small blank *ebiten.Image used when a knot
// has no stored image (e.g. the unknot).
func placeholderImage(_ string) *ebiten.Image {
	return ebiten.NewImage(1, 1)
}
