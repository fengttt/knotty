package main

import (
	"image"
	"image/color"
	"strconv"

	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	etext "github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// debugArcMark is an overlay marker placed at an arc's midpoint. Info
// is the tooltip text displayed when the cursor hovers over the mark.
type debugArcMark struct {
	At   image.Point
	Info string
}

// Drawing tools for scaledImage's pencil/eraser mode.
const (
	ToolPencil = 0
	ToolEraser = 1
)

// scaledImage is a minimal ebitenui widget that draws an *ebiten.Image
// uniformly scaled to fit inside its allocated Rect, preserving aspect
// ratio and centered. When the Rect is wider than it is tall (or vice
// versa), the image renders as a centered square-ish region with empty
// space on the long axis.
//
// When DrawEnabled is true, left-mouse drags over the image paint onto
// Image directly (pencil writes BrushColor, eraser writes white). Host
// code owns Image; scaledImage never allocates it.
type scaledImage struct {
	Image *ebiten.Image

	// Drawing mode. When DrawEnabled is true, mouse-drag paints on
	// Image using Tool (ToolPencil / ToolEraser), BrushColor, and
	// BrushSize. BrushSize is in source-image pixels; the eraser uses
	// a wider brush than the pencil.
	DrawEnabled bool
	Tool        int
	BrushColor  color.Color
	BrushSize   float32

	// DebugCrossings are points in the source image's pixel coordinates
	// to overlay as circles (used by the Debug button). Coordinates are
	// transformed through the same scale/offset used to draw Image.
	// Each point is labelled with its index in this slice.
	DebugCrossings []image.Point

	// DebugArcs are arc midpoints (in source image's pixel coordinates),
	// drawn as small × marks. Each carries a tooltip string shown on
	// hover (typically the arc endpoints and over/under flags).
	DebugArcs []debugArcMark

	// DebugFace is the font face used to render crossing index labels.
	// When nil, labels are skipped.
	DebugFace etext.Face

	// Drawing state: last cursor position (in source-image pixel coords)
	// and whether we're mid-stroke.
	lastDrawPoint image.Point
	drawing       bool

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
	s.handleDrawing()
}

// handleDrawing translates the mouse state into strokes on Image. The
// widget's allocated Rect typically over-covers the drawn image (due
// to aspect-ratio letterboxing), so we convert the cursor's screen
// coordinates to source-image pixel coordinates and only paint when
// those land inside Image.Bounds().
func (s *scaledImage) handleDrawing() {
	if !s.DrawEnabled || s.Image == nil {
		s.drawing = false
		return
	}
	ib := s.Image.Bounds()
	iw, ih := ib.Dx(), ib.Dy()
	rw, rh := s.widget.Rect.Dx(), s.widget.Rect.Dy()
	if iw <= 0 || ih <= 0 || rw <= 0 || rh <= 0 {
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

	pressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	mx, my := ebiten.CursorPosition()
	px := (float64(mx) - ox) / scale
	py := (float64(my) - oy) / scale
	inBounds := px >= 0 && py >= 0 && px < float64(iw) && py < float64(ih)

	if pressed && inBounds {
		cur := image.Point{X: int(px), Y: int(py)}
		if s.drawing {
			s.strokeLine(s.lastDrawPoint, cur)
		} else {
			s.strokeDot(cur)
		}
		s.lastDrawPoint = cur
		s.drawing = true
	} else {
		s.drawing = false
	}
}

func (s *scaledImage) strokeColor() color.Color {
	if s.Tool == ToolEraser {
		return color.NRGBA{0xff, 0xff, 0xff, 0xff}
	}
	if s.BrushColor == nil {
		return color.NRGBA{0, 0, 0, 0xff}
	}
	return s.BrushColor
}

func (s *scaledImage) brushWidth() float32 {
	w := s.BrushSize
	if w <= 0 {
		w = 3
	}
	if s.Tool == ToolEraser {
		w *= 4
	}
	return w
}

// strokeLine paints a stroked segment from a to b plus a disk at each
// endpoint, so consecutive segments join smoothly without visible gaps
// and the caps look round rather than butted.
func (s *scaledImage) strokeLine(a, b image.Point) {
	w := s.brushWidth()
	c := s.strokeColor()
	vector.StrokeLine(s.Image, float32(a.X), float32(a.Y), float32(b.X), float32(b.Y), w, c, true)
	r := w / 2
	vector.FillCircle(s.Image, float32(a.X), float32(a.Y), r, c, true)
	vector.FillCircle(s.Image, float32(b.X), float32(b.Y), r, c, true)
}

func (s *scaledImage) strokeDot(p image.Point) {
	vector.FillCircle(s.Image, float32(p.X), float32(p.Y), s.brushWidth()/2, s.strokeColor(), true)
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

	if len(s.DebugCrossings) == 0 && len(s.DebugArcs) == 0 {
		return
	}
	r := float32(8)
	if rr := float32(iw) * 0.02 * float32(scale); rr > r {
		r = rr
	}
	clrCross := color.NRGBA{0xff, 0x40, 0x40, 0xff}
	clrArc := color.NRGBA{0x40, 0xc0, 0xff, 0xff}
	mx, my := ebiten.CursorPosition()

	// Arc × mark is half the circle radius; hover hit-box matches.
	armLen := r * 0.55
	hoverText := ""
	hoverD2 := r * r

	for i, p := range s.DebugCrossings {
		cx := float32(ox) + float32(p.X)*float32(scale)
		cy := float32(oy) + float32(p.Y)*float32(scale)
		vector.StrokeCircle(screen, cx, cy, r, 2, clrCross, true)
		dx, dy := float32(mx)-cx, float32(my)-cy
		if d2 := dx*dx + dy*dy; d2 < hoverD2 {
			hoverText = strconv.Itoa(i)
			hoverD2 = d2
		}
	}
	for _, m := range s.DebugArcs {
		cx := float32(ox) + float32(m.At.X)*float32(scale)
		cy := float32(oy) + float32(m.At.Y)*float32(scale)
		vector.StrokeLine(screen, cx-armLen, cy-armLen, cx+armLen, cy+armLen, 2, clrArc, true)
		vector.StrokeLine(screen, cx-armLen, cy+armLen, cx+armLen, cy-armLen, 2, clrArc, true)
		dx, dy := float32(mx)-cx, float32(my)-cy
		if d2 := dx*dx + dy*dy; d2 < hoverD2 {
			hoverText = m.Info
			hoverD2 = d2
		}
	}
	if hoverText != "" && s.DebugFace != nil {
		s.drawTooltip(screen, hoverText, mx, my)
	}
}

// drawTooltip renders a small label near the cursor. Positioned
// slightly up-and-right from the cursor so it doesn't occlude the
// marker being pointed at; clamped to stay on-screen.
func (s *scaledImage) drawTooltip(screen *ebiten.Image, label string, mx, my int) {
	padX, padY := float32(6), float32(3)
	w, h := etext.Measure(label, s.DebugFace, 0)
	bw := float32(w) + 2*padX
	bh := float32(h) + 2*padY
	bx := float32(mx) + 12
	by := float32(my) - bh - 4
	sb := screen.Bounds()
	if bx+bw > float32(sb.Max.X) {
		bx = float32(sb.Max.X) - bw
	}
	if by < float32(sb.Min.Y) {
		by = float32(my) + 12
	}
	vector.FillRect(screen, bx, by, bw, bh, color.NRGBA{0x20, 0x20, 0x28, 0xe8}, true)
	vector.StrokeRect(screen, bx, by, bw, bh, 1, color.NRGBA{0xff, 0xff, 0xff, 0x80}, true)
	opts := &etext.DrawOptions{}
	opts.GeoM.Translate(float64(bx+padX), float64(by+padY))
	opts.ColorScale.ScaleWithColor(color.NRGBA{240, 240, 240, 255})
	etext.Draw(screen, label, s.DebugFace, opts)
}
