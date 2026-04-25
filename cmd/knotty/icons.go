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

var symbolFace etext.Face

func init() {
	src, err := etext.NewGoTextFaceSource(bytes.NewReader(notoSymbolsTTF))
	if err != nil {
		log.Fatalf("load symbols font: %v", err)
	}
	symbolFace = &etext.GoTextFace{Source: src, Size: 20}
}

// glyphIcon renders a single rune from the embedded symbols font,
// centered in a 24×24 image and tinted to the given color.
func glyphIcon(glyph string, c color.Color) *ebiten.Image {
	img := ebiten.NewImage(iconSize, iconSize)
	w, h := etext.Measure(glyph, symbolFace, 0)
	opts := &etext.DrawOptions{}
	opts.GeoM.Translate((float64(iconSize)-w)/2, (float64(iconSize)-h)/2)
	opts.ColorScale.ScaleWithColor(c)
	etext.Draw(img, glyph, symbolFace, opts)
	return img
}

// pencilIcon renders U+1F589 (LOWER LEFT PENCIL) in bright yellow.
func pencilIcon() *ebiten.Image {
	return glyphIcon("\U0001F589", color.NRGBA{0xff, 0xee, 0x80, 0xff})
}

// eraserIcon renders U+232B (ERASE TO THE LEFT) in bright pink.
func eraserIcon() *ebiten.Image {
	return glyphIcon("⌫", color.NRGBA{0xff, 0x90, 0xb0, 0xff})
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
