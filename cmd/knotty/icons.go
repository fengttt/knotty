package main

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// iconSize is the side length used for all toolbar glyph icons. The
// button's GraphicPadding adds visual breathing room around it.
const iconSize = 24

// pencilIcon draws a small pencil glyph: a diagonal yellow shaft
// running from top-right (sharpened tip) to bottom-left (eraser end),
// with a dark lead point at the tip and a pink stub at the back.
func pencilIcon() *ebiten.Image {
	img := ebiten.NewImage(iconSize, iconSize)
	yellow := color.NRGBA{0xe8, 0xc2, 0x40, 0xff}
	dark := color.NRGBA{0x20, 0x20, 0x20, 0xff}
	pink := color.NRGBA{0xe0, 0x80, 0x90, 0xff}
	silver := color.NRGBA{0xb8, 0xb8, 0xc0, 0xff}
	outline := color.NRGBA{0x10, 0x10, 0x10, 0xc0}

	// Yellow shaft — thick stroked line along the 45° diagonal.
	vector.StrokeLine(img, 7, 17, 17, 7, 5, yellow, true)
	// Outline of shaft so it reads against the grey button.
	vector.StrokeLine(img, 7, 17, 17, 7, 6, outline, true)
	vector.StrokeLine(img, 7, 17, 17, 7, 4, yellow, true)

	// Lead point (dark) at the upper-right.
	vector.FillCircle(img, 19, 5, 2, dark, true)
	// Wood cone shoulder transitions into the lead point.
	vector.FillCircle(img, 17, 7, 1.5, color.NRGBA{0xd8, 0xa8, 0x60, 0xff}, true)

	// Silver ferrule between shaft and eraser.
	vector.FillCircle(img, 7, 17, 2, silver, true)
	// Pink eraser stub at the lower-left.
	vector.FillCircle(img, 5, 19, 2.5, pink, true)
	vector.StrokeCircle(img, 5, 19, 2.5, 1, outline, true)
	return img
}

// eraserIcon draws a chunky rectangular eraser, rotated slightly off
// the axis so it doesn't look like just a colored block.
func eraserIcon() *ebiten.Image {
	img := ebiten.NewImage(iconSize, iconSize)
	pink := color.NRGBA{0xe8, 0x90, 0xa0, 0xff}
	dark := color.NRGBA{0xa8, 0x60, 0x70, 0xff}
	border := color.NRGBA{0x30, 0x30, 0x30, 0xff}

	// Body — split top/bottom for the classic two-tone look.
	vector.FillRect(img, 4, 8, 16, 5, pink, true)
	vector.FillRect(img, 4, 13, 16, 5, dark, true)
	vector.StrokeRect(img, 4, 8, 16, 10, 1.5, border, true)
	// A short sweep mark trailing off the right edge to suggest erasing.
	vector.StrokeLine(img, 20, 14, 23, 16, 1.5, color.NRGBA{0x60, 0x60, 0x70, 0xff}, true)
	vector.StrokeLine(img, 20, 17, 23, 19, 1.5, color.NRGBA{0x60, 0x60, 0x70, 0xff}, true)
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
	vector.StrokeCircle(img, iconSize/2, iconSize/2, 9, 1.5, color.NRGBA{0x20, 0x20, 0x20, 0xff}, true)
}
