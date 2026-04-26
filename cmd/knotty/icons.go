package main

import (
	"bytes"
	_ "embed"
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	etext "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// iconSize is the side length used for all toolbar glyph icons. The
// button's GraphicPadding adds visual breathing room around it.
const iconSize = 24

// notoSymbolsTTF supplies U+1F589 (LOWER LEFT PENCIL) and other
// pictographic code points that the Go font (goregular) does not cover.
// The bundled font is Noto Sans Symbols 2 (SIL OFL).
//
//go:embed NotoSansSymbols2-Regular.ttf
var notoSymbolsTTF []byte

// materialSymbolsTTF is a 3-glyph subset of Material Symbols Outlined
// (Apache 2.0). It supplies U+E166 (undo), U+E65F (auto_awesome, used
// for Beautify), and U+E6D0 (ink_eraser).
//
//go:embed MaterialSymbols-subset.ttf
var materialSymbolsTTF []byte

var (
	symbolFace   etext.Face
	materialFace etext.Face
)

func init() {
	src, err := etext.NewGoTextFaceSource(bytes.NewReader(notoSymbolsTTF))
	if err != nil {
		log.Fatalf("load symbols font: %v", err)
	}
	symbolFace = &etext.GoTextFace{Source: src, Size: 20}

	msrc, err := etext.NewGoTextFaceSource(bytes.NewReader(materialSymbolsTTF))
	if err != nil {
		log.Fatalf("load material symbols font: %v", err)
	}
	materialFace = &etext.GoTextFace{Source: msrc, Size: 20}
}

// glyphIconFace renders a single rune from the given font face, centered
// in a 24×24 image and tinted to the given color.
func glyphIconFace(face etext.Face, glyph string, c color.Color) *ebiten.Image {
	img := ebiten.NewImage(iconSize, iconSize)
	w, h := etext.Measure(glyph, face, 0)
	opts := &etext.DrawOptions{}
	opts.GeoM.Translate((float64(iconSize)-w)/2, (float64(iconSize)-h)/2)
	opts.ColorScale.ScaleWithColor(c)
	etext.Draw(img, glyph, face, opts)
	return img
}

// glyphIcon renders a Noto-symbols glyph; kept for the pencil icon.
func glyphIcon(glyph string, c color.Color) *ebiten.Image {
	return glyphIconFace(symbolFace, glyph, c)
}

// pencilIcon renders U+1F589 (LOWER LEFT PENCIL) in bright yellow.
func pencilIcon() *ebiten.Image {
	return glyphIcon("\U0001F589", color.NRGBA{0xff, 0xee, 0x80, 0xff})
}

// eraserIcon renders U+E6D0 (Material Symbols "ink_eraser") in bright pink.
func eraserIcon() *ebiten.Image {
	return glyphIconFace(materialFace, "", color.NRGBA{0xff, 0x90, 0xb0, 0xff})
}

// beautifyIcon renders U+E65F (Material Symbols "auto_awesome") in
// soft cyan as the Beautify trigger.
func beautifyIcon() *ebiten.Image {
	return glyphIconFace(materialFace, "", color.NRGBA{0x80, 0xe0, 0xff, 0xff})
}

// undoIcon renders U+E166 (Material Symbols "undo") in soft amber.
func undoIcon() *ebiten.Image {
	return glyphIconFace(materialFace, "", color.NRGBA{0xff, 0xc8, 0x70, 0xff})
}

// moveIcon draws a four-headed arrow (the standard "move" cursor
// glyph) inside a 24×24 image. Drawn programmatically because it
// isn't covered by either bundled symbol font subset and we don't
// want to bring in another TTF for one glyph.
func moveIcon() *ebiten.Image {
	img := ebiten.NewImage(iconSize, iconSize)
	c := color.NRGBA{0xb0, 0xe0, 0xff, 0xff}
	const (
		cx, cy   = float32(iconSize) / 2, float32(iconSize) / 2
		armR     = float32(9)
		headSize = float32(3)
		stroke   = float32(1.5)
	)
	// Vertical and horizontal shafts intersecting at center.
	vector.StrokeLine(img, cx, cy-armR, cx, cy+armR, stroke, c, true)
	vector.StrokeLine(img, cx-armR, cy, cx+armR, cy, stroke, c, true)

	// Arrowhead at each tip: two short legs aimed back toward the
	// center, forming a small "V" that opens away from the tip.
	heads := []struct {
		tipX, tipY                 float32
		leg1X, leg1Y, leg2X, leg2Y float32
	}{
		// Up tip — legs splay down-left and down-right.
		{cx, cy - armR, cx - headSize, cy - armR + headSize, cx + headSize, cy - armR + headSize},
		// Down tip — legs splay up-left and up-right.
		{cx, cy + armR, cx - headSize, cy + armR - headSize, cx + headSize, cy + armR - headSize},
		// Left tip — legs splay right-up and right-down.
		{cx - armR, cy, cx - armR + headSize, cy - headSize, cx - armR + headSize, cy + headSize},
		// Right tip — legs splay left-up and left-down.
		{cx + armR, cy, cx + armR - headSize, cy - headSize, cx + armR - headSize, cy + headSize},
	}
	for _, h := range heads {
		vector.StrokeLine(img, h.tipX, h.tipY, h.leg1X, h.leg1Y, stroke, c, true)
		vector.StrokeLine(img, h.tipX, h.tipY, h.leg2X, h.leg2Y, stroke, c, true)
	}
	return img
}

// colorSwatchIcon draws a single filled circle in the given color with
// a thin dark border so light colors (yellow) stay visible against the
// button background.
func colorSwatchIcon(c color.Color) *ebiten.Image {
	img := ebiten.NewImage(iconSize, iconSize)
	paintColorSwatch(img, c)
	return img
}

// paintColorSwatch (re)draws the swatch onto an existing image. Used to
// update the trigger button's displayed color in place — ebitenui's
// Button captures the GraphicImage.Idle pointer at construction and
// doesn't re-read it on render unless auto-update is on, so the only
// way to refresh the visible swatch is to repaint the same image.
func paintColorSwatch(img *ebiten.Image, c color.Color) {
	img.Clear()
	vector.FillCircle(img, iconSize/2, iconSize/2, 9, c, true)
	vector.StrokeCircle(img, iconSize/2, iconSize/2, 9, 1.5, color.NRGBA{0xff, 0xff, 0xff, 0xff}, true)
}
