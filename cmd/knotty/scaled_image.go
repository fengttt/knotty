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
	// ToolMove disables pencil/eraser drawing and routes the press
	// into the drag state machine instead. A press over a crossing
	// or arc grabs it immediately and continuous press-motion drags
	// it. A press over empty space is a no-op.
	ToolMove = 2
	// ToolReidemeister collects a free-form pointer-drag into a
	// closed polygon (the "lasso"). On release, OnLasso is invoked
	// with the closed point list so the host can detect-and-rewrite a
	// Reidemeister move on the underlying Diagram. While the user is
	// dragging, the lasso path is overlayed on the canvas.
	ToolReidemeister = 3
)

// scaledImage is a minimal ebitenui widget that draws an *ebiten.Image
// uniformly scaled to fit inside its allocated Rect, preserving aspect
// ratio and centered. When the Rect is wider than it is tall (or vice
// versa), the image renders as a centered square-ish region with empty
// space on the long axis.
//
// When DrawEnabled is true and Tool is ToolPencil or ToolEraser,
// left-mouse / touch drags over the image paint onto Image directly
// (pencil writes BrushColor, eraser writes white). Host code owns
// Image; scaledImage never allocates it.
//
// When Tool is ToolMove and Diagram is non-nil, the widget instead
// routes presses into the drag state machine (see drag.go): press
// over a crossing or arc grabs it, continuous press-motion drags it,
// release ends the drag. After every diagram-mutating frame
// OnDiagramChanged is invoked so the host can re-render the canvas.
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

	// Diagram, when non-nil, enables interactive drag of crossings and
	// arcs. The host owns the diagram value; scaledImage mutates the
	// underlying Crossings / Arc.Polyline slices in place. Set to nil
	// to disable drag.
	Diagram *Diagram

	// OnDiagramChanged is called after a drag mutation so the host can
	// re-render the canvas from the (mutated) diagram. Called per frame
	// while dragging — keep it cheap.
	OnDiagramChanged func()

	drag dragState

	// lassoPath is the cursor path collected during a ToolReidemeister
	// drag. Coordinates are in source-image (canvas) pixels. Empty when
	// no lasso is in progress; the host clears it (via OnLasso) only
	// after consuming a completed lasso, otherwise the overlay keeps
	// painting it. Reset on every press to start a fresh path.
	lassoPath    []image.Point
	lassoPressed bool

	// OnLasso is invoked with the auto-closed lasso polygon (last
	// point == first point, in source-image pixel coordinates) once
	// the pointer is released after a ToolReidemeister drag. nil
	// disables the tool's effect (it still draws the overlay).
	OnLasso func(closed []image.Point)

	// DebugCrossings are points in the source image's pixel coordinates
	// to overlay as circles (used by the Debug button). Coordinates are
	// transformed through the same scale/offset used to draw Image.
	// Each point is labelled with its index in this slice.
	DebugCrossings []image.Point

	// DebugArcs are arc midpoints (in source image's pixel coordinates),
	// drawn as small × marks. Each carries a tooltip string shown on
	// hover (typically the arc endpoints and over/under flags).
	DebugArcs []debugArcMark

	// DebugJunctions are pixels in the thinned skeleton with more than
	// two same-color neighbors — places where the convert pipeline
	// couldn't separate over from under. Rendered as orange circles so
	// they're distinguishable from resolved crossings.
	DebugJunctions []image.Point

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

// primaryPointer returns the position and pressed state of the active
// pointing device — touch wins over mouse so the same finger never
// double-fires through synthetic mouse events. The first active touch
// (lowest-index in AppendTouchIDs) is the only one consulted; multi-
// touch gestures aren't part of the drawing/drag UI. On desktop with
// no touch screen, AppendTouchIDs is always empty and this falls
// back to mouse. fromTouch is true iff a touch was the source.
func primaryPointer() (x, y float64, pressed, fromTouch bool) {
	touches := ebiten.AppendTouchIDs(nil)
	if len(touches) > 0 {
		tx, ty := ebiten.TouchPosition(touches[0])
		return float64(tx), float64(ty), true, true
	}
	mx, my := ebiten.CursorPosition()
	return float64(mx), float64(my), ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), false
}

func (s *scaledImage) Update(updObj *widget.UpdateObject) {
	s.init.Do()
	s.widget.Update(updObj)
	// Tool selects which input handler runs. ToolMove routes the
	// press into the drag state machine; Pencil/Eraser route into
	// handleDrawing. The two are mutually exclusive — switching tools
	// resets stale state on whichever side just lost the input.
	switch s.Tool {
	case ToolMove:
		s.drawing = false
		s.lassoReset()
		s.handleDragging()
	case ToolReidemeister:
		s.drawing = false
		s.drag.reset()
		s.handleLasso()
	default:
		s.drag.reset()
		s.lassoReset()
		s.handleDrawing()
	}
}

// lassoReset clears any in-progress lasso state. Called when the user
// switches away from ToolReidemeister so a partially-collected lasso
// doesn't linger as a visual overlay.
func (s *scaledImage) lassoReset() {
	s.lassoPath = nil
	s.lassoPressed = false
}

// handleLasso collects pointer drags into s.lassoPath while the
// pointer is pressed. On release (after at least 3 points have been
// collected so we have an actual polygon), the path is auto-closed by
// appending the first point and OnLasso is fired. Points are recorded
// in source-image pixel coordinates; consecutive duplicates and very-
// close-together points are filtered out so the path stays short
// enough for cheap point-in-polygon tests.
func (s *scaledImage) handleLasso() {
	if s.Image == nil {
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

	mx, my, pressed, _ := primaryPointer()
	px := (mx - ox) / scale
	py := (my - oy) / scale
	inBounds := px >= 0 && py >= 0 && px < float64(iw) && py < float64(ih)

	if pressed {
		if !s.lassoPressed {
			// New lasso. Reset path and seed with first point.
			s.lassoPath = s.lassoPath[:0]
			if inBounds {
				s.lassoPath = append(s.lassoPath, image.Point{X: int(px), Y: int(py)})
			}
			s.lassoPressed = true
			return
		}
		if !inBounds {
			return
		}
		cur := image.Point{X: int(px), Y: int(py)}
		if n := len(s.lassoPath); n == 0 {
			s.lassoPath = append(s.lassoPath, cur)
		} else if last := s.lassoPath[n-1]; last != cur {
			dx, dy := cur.X-last.X, cur.Y-last.Y
			if dx*dx+dy*dy >= 4 { // ≥ 2 px from previous sample
				s.lassoPath = append(s.lassoPath, cur)
			}
		}
		return
	}
	// Pointer released this frame.
	if !s.lassoPressed {
		return
	}
	s.lassoPressed = false
	if len(s.lassoPath) >= 3 {
		// Auto-close: append the first point so the polygon is closed
		// and the host's point-in-polygon test sees the loop edge.
		closed := make([]image.Point, len(s.lassoPath)+1)
		copy(closed, s.lassoPath)
		closed[len(s.lassoPath)] = s.lassoPath[0]
		if s.OnLasso != nil {
			s.OnLasso(closed)
		}
	}
	s.lassoPath = nil
}

// handleDragging routes pointer input into the drag state machine.
// Only called when Tool == ToolMove, so it doesn't need to coexist
// with the drawing path. A nil Diagram is benign — refreshHover is
// a no-op and presses over empty space don't grab anything.
func (s *scaledImage) handleDragging() {
	if s.Diagram == nil {
		s.drag.reset()
		return
	}
	if s.Image == nil {
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

	mx, my, pressed, fromTouch := primaryPointer()
	cursor := imagePointF{
		X: (mx - ox) / scale,
		Y: (my - oy) / scale,
	}
	inBounds := cursor.X >= 0 && cursor.Y >= 0 && cursor.X < float64(iw) && cursor.Y < float64(ih)

	mutated := s.drag.update(s.Diagram, cursor, inBounds, pressed, fromTouch)
	if mutated && s.OnDiagramChanged != nil {
		s.OnDiagramChanged()
	}
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

	mx, my, pressed, _ := primaryPointer()
	px := (mx - ox) / scale
	py := (my - oy) / scale
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

	// Drag hover/grab overlay sits above the canvas and below the debug
	// markers so the debug overlay (when enabled) is never occluded.
	drawDragOverlay(screen, ox, oy, scale, s.Diagram, &s.drag)

	// In-progress lasso path. Drawn on every frame as long as
	// lassoPath is non-empty (set during pointer drag in
	// handleLasso). Translucent magenta so it reads as transient.
	if len(s.lassoPath) >= 2 {
		lc := color.NRGBA{0xc8, 0x40, 0xff, 0xb0}
		for i := 0; i+1 < len(s.lassoPath); i++ {
			p := s.lassoPath[i]
			q := s.lassoPath[i+1]
			vector.StrokeLine(screen,
				float32(ox)+float32(p.X)*float32(scale),
				float32(oy)+float32(p.Y)*float32(scale),
				float32(ox)+float32(q.X)*float32(scale),
				float32(oy)+float32(q.Y)*float32(scale),
				2, lc, true)
		}
		// Soft preview of the closing segment so the user sees what
		// shape the auto-close will make.
		if s.lassoPressed && len(s.lassoPath) >= 3 {
			p := s.lassoPath[len(s.lassoPath)-1]
			q := s.lassoPath[0]
			vector.StrokeLine(screen,
				float32(ox)+float32(p.X)*float32(scale),
				float32(oy)+float32(p.Y)*float32(scale),
				float32(ox)+float32(q.X)*float32(scale),
				float32(oy)+float32(q.Y)*float32(scale),
				1, color.NRGBA{0xc8, 0x40, 0xff, 0x60}, true)
		}
	}

	if len(s.DebugCrossings) == 0 && len(s.DebugArcs) == 0 && len(s.DebugJunctions) == 0 {
		return
	}
	r := float32(8)
	if rr := float32(iw) * 0.02 * float32(scale); rr > r {
		r = rr
	}
	clrCross := color.NRGBA{0xff, 0x40, 0x40, 0xff}
	clrArc := color.NRGBA{0x40, 0xc0, 0xff, 0xff}
	clrJunc := color.NRGBA{0xff, 0xa0, 0x20, 0xff}
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
	// Junctions are skeleton pixels the convert pipeline couldn't
	// resolve. Drawn slightly smaller than crossings so they read as
	// "trouble spot" rather than "valid crossing".
	jr := r * 0.7
	for _, p := range s.DebugJunctions {
		cx := float32(ox) + float32(p.X)*float32(scale)
		cy := float32(oy) + float32(p.Y)*float32(scale)
		vector.StrokeCircle(screen, cx, cy, jr, 2, clrJunc, true)
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
