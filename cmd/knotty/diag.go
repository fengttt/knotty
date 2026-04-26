package main

import (
	"fmt"
	"image/color"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	etext "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// inputDiag is an always-on debugging overlay that visualizes the raw
// pointer input Ebiten is receiving — primarily a tool for diagnosing
// whether iOS Safari is delivering touch events to the wasm runtime.
//
// Each active touch shows as a translucent green ring at its current
// position with a "Xs" label counting how long the touch has been
// held; when the mouse left button is held, a smaller magenta ring
// is drawn at the cursor with a similar label. If a finger is held
// on screen and no ring appears, Ebiten is not seeing the touch at
// all (most likely the OS or browser hijacked the gesture); if the
// ring appears but the label resets to 0 each frame, the touch ID
// isn't persisting between frames.
type inputDiag struct {
	touchStarts map[ebiten.TouchID]time.Time
	mouseDownAt time.Time
	mouseHeld   bool
}

// update should run once per frame before draw, so the start times
// for newly-pressed inputs are recorded and stale ones are cleared.
func (d *inputDiag) update() {
	if d.touchStarts == nil {
		d.touchStarts = make(map[ebiten.TouchID]time.Time)
	}
	now := time.Now()

	active := ebiten.AppendTouchIDs(nil)
	seen := make(map[ebiten.TouchID]bool, len(active))
	for _, id := range active {
		seen[id] = true
		if _, ok := d.touchStarts[id]; !ok {
			d.touchStarts[id] = now
		}
	}
	for id := range d.touchStarts {
		if !seen[id] {
			delete(d.touchStarts, id)
		}
	}

	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		if !d.mouseHeld {
			d.mouseHeld = true
			d.mouseDownAt = now
		}
	} else {
		d.mouseHeld = false
	}
}

// draw overlays the diagnostic on screen. Should be called late in the
// render pipeline so the rings sit above the UI.
func (d *inputDiag) draw(screen *ebiten.Image, face etext.Face) {
	now := time.Now()

	// Touch rings — translucent green so they don't fully occlude
	// what's underneath but are clearly visible.
	ringColor := color.NRGBA{0x40, 0xff, 0x60, 0xc0}
	for id, t0 := range d.touchStarts {
		tx, ty := ebiten.TouchPosition(id)
		vector.StrokeCircle(screen, float32(tx), float32(ty), 26, 3, ringColor, true)
		// Smaller filled center dot makes the precise touch point
		// readable even when the ring is large.
		vector.FillCircle(screen, float32(tx), float32(ty), 4, ringColor, true)
		if face != nil {
			label := fmt.Sprintf("touch %d  %.1fs", int(id), now.Sub(t0).Seconds())
			drawDiagLabel(screen, face, label, float64(tx)+32, float64(ty)-8)
		}
	}

	// Mouse ring shown only while the left button is held — desktop
	// users don't want a ring tracking their cursor full-time.
	if d.mouseHeld {
		c := color.NRGBA{0xff, 0x60, 0xff, 0xc0}
		mx, my := ebiten.CursorPosition()
		vector.StrokeCircle(screen, float32(mx), float32(my), 14, 2, c, true)
		if face != nil {
			label := fmt.Sprintf("mouse  %.1fs", now.Sub(d.mouseDownAt).Seconds())
			drawDiagLabel(screen, face, label, float64(mx)+18, float64(my)-6)
		}
	}

	// Top-left status: number of active touches plus mouse-pressed
	// flag. Always rendered so absence of touches is also informative
	// ("touches: 0" when the user IS pressing means events aren't
	// reaching Ebiten).
	if face != nil {
		drawDiagLabel(screen, face,
			fmt.Sprintf("touches: %d   mouse: %v", len(d.touchStarts), d.mouseHeld),
			6, 6)
	}
}

// drawDiagLabel paints text with a dark backing rectangle so it stays
// readable against any canvas content beneath.
func drawDiagLabel(screen *ebiten.Image, face etext.Face, label string, x, y float64) {
	const padX, padY = 4.0, 2.0
	w, h := etext.Measure(label, face, 0)
	bx := float32(x)
	by := float32(y)
	bw := float32(w) + 2*float32(padX)
	bh := float32(h) + 2*float32(padY)
	vector.FillRect(screen, bx, by, bw, bh, color.NRGBA{0x10, 0x10, 0x10, 0xc8}, true)
	vector.StrokeRect(screen, bx, by, bw, bh, 1, color.NRGBA{0xff, 0xff, 0xff, 0x80}, true)
	opts := &etext.DrawOptions{}
	opts.GeoM.Translate(x+padX, y+padY)
	opts.ColorScale.ScaleWithColor(color.NRGBA{0xff, 0xff, 0xff, 0xff})
	etext.Draw(screen, label, face, opts)
}
