package main

import (
	"image"

	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
)

// scaledImage is a minimal ebitenui widget that draws an *ebiten.Image
// uniformly scaled to fit inside its allocated Rect, preserving aspect
// ratio and centered. When the Rect is wider than it is tall (or vice
// versa), the image renders as a centered square-ish region with empty
// space on the long axis.
type scaledImage struct {
	Image *ebiten.Image

	widgetOpts []widget.WidgetOpt
	init       *widget.MultiOnce
	widget     *widget.Widget
}

func newScaledImage(opts ...widget.WidgetOpt) *scaledImage {
	s := &scaledImage{
		init:       &widget.MultiOnce{},
		widgetOpts: opts,
	}
	s.init.Append(s.createWidget)
	return s
}

func (s *scaledImage) createWidget() {
	s.widget = widget.NewWidget(s.widgetOpts...)
	s.widgetOpts = nil
}

func (s *scaledImage) GetWidget() *widget.Widget {
	s.init.Do()
	return s.widget
}

func (s *scaledImage) SetLocation(rect image.Rectangle) {
	s.init.Do()
	s.widget.Rect = rect
}

// PreferredSize is what the enclosing layout uses to size the cell.
// We honor MinWidth / MinHeight on the underlying Widget so callers can
// drive the cell size by setting those fields (including dynamically —
// game.Layout updates MinWidth/MinHeight every frame to keep the
// picture area a full-width square). A small intrinsic floor keeps the
// widget visible before the first Layout pass.
func (s *scaledImage) PreferredSize() (int, int) {
	s.init.Do()
	w, h := 64, 64
	if s.widget != nil {
		if s.widget.MinWidth > w {
			w = s.widget.MinWidth
		}
		if s.widget.MinHeight > h {
			h = s.widget.MinHeight
		}
	}
	return w, h
}

func (s *scaledImage) Validate() {}

func (s *scaledImage) Update(updObj *widget.UpdateObject) {
	s.init.Do()
	s.widget.Update(updObj)
}

func (s *scaledImage) Render(screen *ebiten.Image) {
	s.init.Do()
	s.widget.Render(screen)
	if s.Image == nil {
		return
	}
	ib := s.Image.Bounds()
	iw, ih := ib.Dx(), ib.Dy()
	if iw <= 0 || ih <= 0 {
		return
	}
	rw, rh := s.widget.Rect.Dx(), s.widget.Rect.Dy()
	if rw <= 0 || rh <= 0 {
		return
	}
	sx := float64(rw) / float64(iw)
	sy := float64(rh) / float64(ih)
	scale := sx
	if sy < sx {
		scale = sy
	}
	dw := float64(iw) * scale
	dh := float64(ih) * scale
	ox := float64(s.widget.Rect.Min.X) + (float64(rw)-dw)/2
	oy := float64(s.widget.Rect.Min.Y) + (float64(rh)-dh)/2
	opts := ebiten.DrawImageOptions{}
	opts.GeoM.Scale(scale, scale)
	opts.GeoM.Translate(ox, oy)
	screen.DrawImage(s.Image, &opts)
}
