package main

import (
	"image"

	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
)

// scaledImage is a minimal ebitenui widget that draws an *ebiten.Image
// stretched to fill its allocated Rect. Aspect ratio is not preserved
// — the image is scaled independently in x and y so the diagram
// occupies the full pane.
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

// PreferredSize advertises a small minimum; the real size comes from
// the enclosing stretched grid cell.
func (s *scaledImage) PreferredSize() (int, int) { return 64, 64 }

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
	opts := ebiten.DrawImageOptions{}
	opts.GeoM.Scale(sx, sy)
	opts.GeoM.Translate(float64(s.widget.Rect.Min.X), float64(s.widget.Rect.Min.Y))
	screen.DrawImage(s.Image, &opts)
}
