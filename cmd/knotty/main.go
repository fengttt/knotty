// Command knotty is the Ebiten-based GUI for exploring the KnotInfo
// database. The window is 9:16 portrait: a full-width square image of
// the knot on top, and a scrollable properties panel underneath. The
// panel hosts the search input, Search/Convert/Debug buttons, the
// image-style dropdown (Diagram, DiagramMirror, Snappy, SnappyMirror,
// Grid), the knot name, and the raw column values for the current
// Diagram.
package main

import (
	"bytes"
	"errors"
	"fmt"
	stdimage "image"
	"image/color"
	"log"
	"math"
	"strings"
	"time"

	"github.com/ebitenui/ebitenui"
	uiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/fengttt/knotty/knot"
	"github.com/hajimehoshi/ebiten/v2"
	etext "github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

const (
	// 9:16 portrait — the top pane is a full-width windowWidth × windowWidth
	// square (the knot diagram) and the rest scrolls underneath it.
	windowWidth  = 540
	windowHeight = 960
)

// styleEntry is one of the five image types, shown in the dropdown.
type styleEntry struct {
	label string
	typ   knot.ImageType
}

var styleEntries = []styleEntry{
	{"Diagram", knot.StyleDiagram},
	{"Diagram Mirror", knot.StyleDiagramMirror},
	{"Snappy", knot.StyleSnappy},
	{"Snappy Mirror", knot.StyleSnappyMirror},
	{"Grid", knot.StyleGrid},
}

// colorEntry is one of the palette colors offered by the pencil color
// dropdown. The canvas background is white so white is intentionally
// excluded from the palette (use the eraser for that).
type colorEntry struct {
	label string
	c     color.NRGBA
}

var colorEntries = []colorEntry{
	{"Black", color.NRGBA{0, 0, 0, 0xff}},
	{"Red", color.NRGBA{0xd0, 0x20, 0x20, 0xff}},
	{"Blue", color.NRGBA{0x20, 0x40, 0xc0, 0xff}},
	{"Green", color.NRGBA{0x20, 0xa0, 0x30, 0xff}},
	{"Yellow", color.NRGBA{0xe0, 0xc0, 0x20, 0xff}},
}

var (
	canvasBG = color.NRGBA{0xff, 0xff, 0xff, 0xff}
)

// game implements ebiten.Game and owns the UI.
type game struct {
	ui *ebitenui.UI

	root        *widget.Container
	input       *widget.TextInput
	imageWidget *scaledImage
	nameLabel   *widget.Text
	propsArea   *widget.TextArea

	// colorSwatch is the single mutable *ebiten.Image used as the trigger
	// button's graphic. ebitenui's Button captures GraphicImage.Idle once
	// at construction (auto-update only fires for buttons with text), so
	// to refresh the displayed color we repaint THIS image rather than
	// swap the pointer.
	colorSwatch *ebiten.Image
	colorIndex  int
	// colorBtn is the trigger button; we need its widget Rect to
	// position the popup just below it.
	colorBtn *widget.Button
	// colorPopupClose, when non-nil, removes the open color popup.
	// Set to nil when the popup is closed (manually or by CLICK_OUT).
	colorPopupClose widget.RemoveWindowFunc

	currentKnot  *knot.Diagram
	currentStyle knot.ImageType

	// undoSnap is a pixel-level snapshot of the canvas captured right
	// before doBeautify overwrites it; doUndo blits it back. Allocated
	// lazily and reused so we don't allocate a fresh Image per click.
	undoSnap *ebiten.Image

	// pendingAttach asks the next Update tick to extract a Diagram from
	// the canvas via convertImage and attach it to imageWidget. We defer
	// because ebiten.Image.ReadPixels (used by canvasToImage) cannot run
	// before RunGame starts, so the initial loadKnot at startup must not
	// try to convert the canvas synchronously.
	pendingAttach bool

	face     etext.Face
	hugeFace etext.Face
}

func main() {
	g := &game{currentStyle: knot.StyleDiagram}

	face, err := loadFont(14)
	if err != nil {
		log.Fatalf("load font: %v", err)
	}
	g.face = face
	hugeFace, err := loadFont(36)
	if err != nil {
		log.Fatalf("load font: %v", err)
	}
	g.hugeFace = hugeFace

	g.ui = g.buildUI()
	g.initCanvas()

	// Seed with the figure-eight knot so the window opens with a
	// recognizable diagram.
	g.loadKnot("4_1")

	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetWindowTitle("Knotty")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

// Layout is called by Ebiten with the current logical screen size. We
// use it as a hook to resize the top image cell to match the window
// width so the square picture area always fills the full width.
func (g *game) Layout(outW, outH int) (int, int) {
	if g.imageWidget != nil {
		w := g.imageWidget.GetWidget()
		if w.MinWidth != outW {
			w.MinWidth = outW
			w.MinHeight = outW
			if g.root != nil {
				g.root.RequestRelayout()
			}
		}
	}
	return outW, outH
}

func (g *game) Update() error {
	if g.pendingAttach {
		g.pendingAttach = false
		g.attachDiagramFromCanvas()
	}
	g.ui.Update()
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	screen.Fill(color.NRGBA{0x1a, 0x1a, 0x1a, 0xff})
	g.ui.Draw(screen)
}

// buildUI constructs the full UI tree.
//
// Layout (9:16 portrait):
//
//	root (grid 1 col, 2 rows; top fixed square, bottom stretched):
//	├─ topPane: [full-width square image]
//	└─ bottomPane: [search row + style][name][scrolling properties]
func (g *game) buildUI() *ebitenui.UI {
	root := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(uiimage.NewNineSliceColor(color.NRGBA{0x1a, 0x1a, 0x1a, 0xff})),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(1),
			// Top row sized by content (a windowWidth-side square);
			// bottom row stretches to fill remaining vertical space.
			widget.GridLayoutOpts.Stretch([]bool{true}, []bool{false, true}),
		)),
	)

	g.root = root
	root.AddChild(g.buildTopPane())
	root.AddChild(g.buildBottomPane())

	return &ebitenui.UI{Container: root}
}

func (g *game) buildTopPane() *widget.Container {
	top := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(uiimage.NewNineSliceColor(color.NRGBA{0x22, 0x22, 0x2a, 0xff})),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(1),
			// Toolbar row is content-sized; canvas row takes the full
			// square below it.
			widget.GridLayoutOpts.Stretch([]bool{true}, []bool{false, true}),
			widget.GridLayoutOpts.Spacing(0, 4),
			widget.GridLayoutOpts.Padding(&widget.Insets{Top: 4, Bottom: 0, Left: 4, Right: 4}),
		)),
	)

	top.AddChild(g.buildDrawToolbar())

	// Full-width square: the image cell's MinWidth / MinHeight are updated
	// every frame in game.Layout so the square follows the current window
	// width. The initial MinSize is just the starting window width so the
	// first frame renders correctly.
	g.imageWidget = newScaledImage(
		widget.WidgetOpts.LayoutData(widget.GridLayoutData{
			HorizontalPosition: widget.GridLayoutPositionStart,
			VerticalPosition:   widget.GridLayoutPositionStart,
		}),
		widget.WidgetOpts.MinSize(windowWidth, windowWidth),
	)
	g.imageWidget.DebugFace = g.face
	g.imageWidget.DrawEnabled = true
	g.imageWidget.Tool = ToolPencil
	g.imageWidget.BrushColor = colorEntries[0].c
	g.imageWidget.BrushSize = 3
	// When a drag mutates the diagram, re-render it onto the canvas so
	// the user sees the change. Keep this cheap — it runs every frame
	// during a drag.
	g.imageWidget.OnDiagramChanged = func() {
		if g.imageWidget == nil || g.imageWidget.Image == nil || g.imageWidget.Diagram == nil {
			return
		}
		renderDiagram(g.imageWidget.Image, g.imageWidget.Diagram, canvasBG)
	}
	g.imageWidget.OnLasso = func(closed []stdimage.Point) {
		g.doReidemeister(closed)
	}
	top.AddChild(g.imageWidget)
	return top
}

// buildDrawToolbar is the row of drawing controls — pencil and eraser
// icon buttons plus a color combo whose trigger displays the currently
// selected color as a filled circle.
func (g *game) buildDrawToolbar() *widget.Container {
	row := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(6),
		)),
	)

	row.AddChild(iconButton(beautifyIcon(), func() {
		g.doBeautify()
	}))
	row.AddChild(iconButton(undoIcon(), func() {
		g.doUndo()
	}))
	row.AddChild(iconButton(pencilIcon(), func() {
		g.imageWidget.Tool = ToolPencil
	}))
	row.AddChild(iconButton(eraserIcon(), func() {
		g.imageWidget.Tool = ToolEraser
	}))
	row.AddChild(iconButton(moveIcon(), func() {
		g.imageWidget.Tool = ToolMove
	}))
	row.AddChild(iconButton(reidemeisterIcon(), func() {
		g.imageWidget.Tool = ToolReidemeister
	}))
	row.AddChild(g.buildColorCombo())
	return row
}

// iconButton is a 36×32 toolbar button whose face is a single icon
// image, with a small inset around the glyph.
func iconButton(icon *ebiten.Image, onClick func()) *widget.Button {
	return widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}),
			widget.WidgetOpts.MinSize(36, 32),
		),
		widget.ButtonOpts.Image(buttonImage()),
		widget.ButtonOpts.Graphic(&widget.GraphicImage{Idle: icon}),
		widget.ButtonOpts.GraphicPadding(widget.Insets{Left: 4, Right: 4, Top: 4, Bottom: 4}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			onClick()
		}),
	)
}

// buildColorCombo builds the pencil-color trigger. The button face is
// a filled circle in the current color; clicking it opens a popup
// (ebitenui Window) of one circle button per palette entry. The popup
// closes when a color is selected or when the user clicks outside it.
//
// ebitenui's ListComboButton only renders text in popup entries, so we
// drop the combo entirely and manage the popup window ourselves.
func (g *game) buildColorCombo() *widget.Button {
	g.colorIndex = 0
	g.colorSwatch = ebiten.NewImage(iconSize, iconSize)
	paintColorSwatch(g.colorSwatch, colorEntries[g.colorIndex].c)
	g.colorBtn = widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}),
			widget.WidgetOpts.MinSize(36, 32),
		),
		widget.ButtonOpts.Image(buttonImage()),
		widget.ButtonOpts.Graphic(&widget.GraphicImage{Idle: g.colorSwatch}),
		widget.ButtonOpts.GraphicPadding(widget.Insets{Left: 4, Right: 4, Top: 4, Bottom: 4}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			g.toggleColorPopup()
		}),
	)
	return g.colorBtn
}

// toggleColorPopup opens (or closes) the color-picker popup window,
// positioned just below the trigger button. Selecting a color updates
// the brush, refreshes the trigger swatch, and dismisses the popup.
func (g *game) toggleColorPopup() {
	if g.colorPopupClose != nil {
		g.colorPopupClose()
		g.colorPopupClose = nil
		return
	}

	contents := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(uiimage.NewNineSliceColor(color.NRGBA{0x2a, 0x2a, 0x32, 0xff})),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionVertical),
			widget.RowLayoutOpts.Spacing(4),
			widget.RowLayoutOpts.Padding(widget.NewInsetsSimple(6)),
		)),
	)

	// closeFn is set after AddWindow returns; the entry handlers below
	// capture this variable so they can close the popup on selection.
	var closeFn widget.RemoveWindowFunc
	for i, e := range colorEntries {
		contents.AddChild(iconButton(colorSwatchIcon(e.c), func() {
			g.colorIndex = i
			g.imageWidget.BrushColor = e.c
			g.imageWidget.Tool = ToolPencil
			paintColorSwatch(g.colorSwatch, e.c)
			if closeFn != nil {
				closeFn()
				g.colorPopupClose = nil
			}
		}))
	}

	window := widget.NewWindow(
		widget.WindowOpts.Contents(contents),
		widget.WindowOpts.CloseMode(widget.CLICK_OUT),
		widget.WindowOpts.ClosedHandler(func(args *widget.WindowClosedEventArgs) {
			g.colorPopupClose = nil
		}),
	)

	const (
		entryW = 36
		entryH = 32
		gapY   = 4
		padXY  = 6
	)
	popupW := entryW + 2*padXY
	popupH := len(colorEntries)*entryH + (len(colorEntries)-1)*gapY + 2*padXY
	rect := g.colorBtn.GetWidget().Rect
	px := rect.Min.X
	py := rect.Max.Y + 2
	window.SetLocation(stdimage.Rect(px, py, px+popupW, py+popupH))

	closeFn = g.ui.AddWindow(window)
	g.colorPopupClose = closeFn
}

// buildStyleCombo builds the image-style dropdown. Lives inside the
// search row at the top of the bottom pane.
func (g *game) buildStyleCombo() *widget.ListComboButton {
	entries := make([]any, len(styleEntries))
	for i, e := range styleEntries {
		entries[i] = e
	}
	combo := widget.NewListComboButton(
		widget.ListComboButtonOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}),
		),
		widget.ListComboButtonOpts.Entries(entries),
		widget.ListComboButtonOpts.MaxContentHeight(200),
		widget.ListComboButtonOpts.ButtonParams(&widget.ButtonParams{
			Image:       buttonImage(),
			TextPadding: widget.NewInsetsSimple(6),
			TextColor: &widget.ButtonTextColor{
				Idle:     color.NRGBA{240, 240, 240, 255},
				Disabled: color.NRGBA{160, 160, 160, 255},
			},
			TextFace: &g.face,
			MinSize:  &stdimage.Point{X: 140, Y: 0},
		}),
		widget.ListComboButtonOpts.ListParams(&widget.ListParams{
			ScrollContainerImage: &widget.ScrollContainerImage{
				Idle:     uiimage.NewNineSliceColor(color.NRGBA{60, 60, 70, 255}),
				Disabled: uiimage.NewNineSliceColor(color.NRGBA{60, 60, 70, 255}),
				Mask:     uiimage.NewNineSliceColor(color.NRGBA{60, 60, 70, 255}),
			},
			Slider: &widget.SliderParams{
				TrackImage: &widget.SliderTrackImage{
					Idle:  uiimage.NewNineSliceColor(color.NRGBA{80, 80, 90, 255}),
					Hover: uiimage.NewNineSliceColor(color.NRGBA{80, 80, 90, 255}),
				},
				HandleImage: buttonImage(),
			},
			EntryFace: &g.face,
			EntryColor: &widget.ListEntryColor{
				Selected:                  color.NRGBA{255, 255, 255, 255},
				Unselected:                color.NRGBA{220, 220, 220, 255},
				SelectedBackground:        color.NRGBA{80, 80, 140, 255},
				SelectedFocusedBackground: color.NRGBA{100, 100, 160, 255},
				FocusedBackground:         color.NRGBA{90, 90, 110, 255},
				DisabledUnselected:        color.NRGBA{100, 100, 100, 255},
				DisabledSelected:          color.NRGBA{100, 100, 100, 255},
			},
			EntryTextPadding: widget.NewInsetsSimple(5),
		}),
		widget.ListComboButtonOpts.EntryLabelFunc(
			func(e any) string { return e.(styleEntry).label },
			func(e any) string { return e.(styleEntry).label },
		),
		widget.ListComboButtonOpts.EntrySelectedHandler(func(args *widget.ListComboButtonEntrySelectedEventArgs) {
			sel := args.Entry.(styleEntry)
			g.currentStyle = sel.typ
			g.refreshImage()
		}),
	)
	combo.SetSelectedEntry(entries[0])
	return combo
}

// buildSearchRow builds the top row of the right pane: knot-name input,
// Search button, and Refresh button (side by side).
func (g *game) buildSearchRow() *widget.Container {
	row := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(6),
		)),
	)

	g.input = widget.NewTextInput(
		widget.TextInputOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{
				Position: widget.RowLayoutPositionCenter,
				Stretch:  true,
			}),
			widget.WidgetOpts.MinSize(120, 0),
		),
		widget.TextInputOpts.Image(&widget.TextInputImage{
			Idle:     uiimage.NewNineSliceColor(color.NRGBA{60, 60, 70, 255}),
			Disabled: uiimage.NewNineSliceColor(color.NRGBA{40, 40, 50, 255}),
		}),
		widget.TextInputOpts.Face(&g.face),
		widget.TextInputOpts.Color(&widget.TextInputColor{
			Idle:          color.NRGBA{240, 240, 240, 255},
			Disabled:      color.NRGBA{160, 160, 160, 255},
			Caret:         color.NRGBA{240, 240, 240, 255},
			DisabledCaret: color.NRGBA{160, 160, 160, 255},
		}),
		widget.TextInputOpts.Padding(widget.NewInsetsSimple(6)),
		widget.TextInputOpts.Placeholder("Knot name. Empty = random."),
		widget.TextInputOpts.SubmitHandler(func(args *widget.TextInputChangedEventArgs) {
			g.doSearch(args.InputText)
		}),
	)
	row.AddChild(g.input)

	searchBtn := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}),
			widget.WidgetOpts.MinSize(80, 32),
		),
		widget.ButtonOpts.Image(buttonImage()),
		widget.ButtonOpts.Text("Search", &g.face, &widget.ButtonTextColor{Idle: color.NRGBA{240, 240, 240, 255}}),
		widget.ButtonOpts.TextPadding(&widget.Insets{Left: 10, Right: 10, Top: 6, Bottom: 6}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			g.doSearch(g.input.GetText())
		}),
	)
	row.AddChild(searchBtn)

	convertBtn := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}),
			widget.WidgetOpts.MinSize(80, 32),
		),
		widget.ButtonOpts.Image(buttonImage()),
		widget.ButtonOpts.Text("Convert", &g.face, &widget.ButtonTextColor{Idle: color.NRGBA{240, 240, 240, 255}}),
		widget.ButtonOpts.TextPadding(&widget.Insets{Left: 10, Right: 10, Top: 6, Bottom: 6}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			g.doConvert()
		}),
	)
	row.AddChild(convertBtn)

	debugBtn := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}),
			widget.WidgetOpts.MinSize(70, 32),
		),
		widget.ButtonOpts.Image(buttonImage()),
		widget.ButtonOpts.Text("Debug", &g.face, &widget.ButtonTextColor{Idle: color.NRGBA{240, 240, 240, 255}}),
		widget.ButtonOpts.TextPadding(&widget.Insets{Left: 10, Right: 10, Top: 6, Bottom: 6}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			g.doDebug()
		}),
	)
	row.AddChild(debugBtn)

	row.AddChild(g.buildStyleCombo())

	return row
}

func (g *game) buildBottomPane() *widget.Container {
	bottom := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(uiimage.NewNineSliceColor(color.NRGBA{0x22, 0x22, 0x2a, 0xff})),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(1),
			// 3 rows: search bar (fixed), name (fixed), properties (stretched).
			widget.GridLayoutOpts.Stretch([]bool{true}, []bool{false, false, true}),
			widget.GridLayoutOpts.Padding(widget.NewInsetsSimple(8)),
			widget.GridLayoutOpts.Spacing(0, 8),
		)),
	)

	bottom.AddChild(g.buildSearchRow())

	g.nameLabel = widget.NewText(
		widget.TextOpts.Text("", &g.hugeFace, color.NRGBA{240, 240, 240, 255}),
		widget.TextOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.GridLayoutData{
				HorizontalPosition: widget.GridLayoutPositionCenter,
			}),
		),
	)
	bottom.AddChild(g.nameLabel)

	g.propsArea = widget.NewTextArea(
		widget.TextAreaOpts.ContainerOpts(
			widget.ContainerOpts.WidgetOpts(
				widget.WidgetOpts.LayoutData(widget.GridLayoutData{
					HorizontalPosition: widget.GridLayoutPositionStart,
					VerticalPosition:   widget.GridLayoutPositionStart,
				}),
				widget.WidgetOpts.MinSize(0, 200),
			),
		),
		widget.TextAreaOpts.ControlWidgetSpacing(4),
		widget.TextAreaOpts.FontColor(color.NRGBA{230, 230, 230, 255}),
		widget.TextAreaOpts.FontFace(&g.face),
		widget.TextAreaOpts.TextPadding(widget.Insets{Top: 6, Bottom: 6, Left: 6, Right: 6}),
		widget.TextAreaOpts.ShowVerticalScrollbar(),
		widget.TextAreaOpts.ScrollContainerImage(&widget.ScrollContainerImage{
			Idle: uiimage.NewNineSliceColor(color.NRGBA{45, 45, 55, 255}),
			Mask: uiimage.NewNineSliceColor(color.NRGBA{45, 45, 55, 255}),
		}),
		widget.TextAreaOpts.SliderParams(&widget.SliderParams{
			TrackImage: &widget.SliderTrackImage{
				Idle:  uiimage.NewNineSliceColor(color.NRGBA{80, 80, 90, 255}),
				Hover: uiimage.NewNineSliceColor(color.NRGBA{80, 80, 90, 255}),
			},
			HandleImage: buttonImage(),
		}),
		widget.TextAreaOpts.Text(""),
	)
	bottom.AddChild(g.propsArea)

	return bottom
}

// doSearch handles the search button / Enter key. An empty query
// clears the image, properties, and search box, and labels the name
// as "Drawing" — a blank canvas state. Non-empty queries look up the
// KnotInfo row by name.
func (g *game) doSearch(q string) {
	q = trim(q)
	if q == "" {
		g.enterDrawingMode()
		return
	}
	g.loadKnot(q)
}

// enterDrawingMode puts the UI into a nameless "Drawing" state: blank
// white canvas, no current knot, no properties text. The debug overlay
// is cleared as well.
func (g *game) enterDrawingMode() {
	g.currentKnot = nil
	g.clearCanvas()
	g.imageWidget.Diagram = nil
	g.imageWidget.DebugCrossings = nil
	g.imageWidget.DebugArcs = nil
	g.imageWidget.DebugJunctions = nil
	g.input.SetText("")
	g.nameLabel.Label = "Drawing"
	g.propsArea.SetText("")
}

// initCanvas allocates a blank white canvas and installs it as the
// drawable image. Called once after the UI tree is built, before the
// first loadKnot.
func (g *game) initCanvas() {
	if g.imageWidget == nil {
		return
	}
	canvas := ebiten.NewImage(windowWidth, windowWidth)
	canvas.Fill(canvasBG)
	g.imageWidget.Image = canvas
}

// clearCanvas fills the canvas with the white background color,
// preserving its size and the Image reference so the scaledImage
// widget keeps pointing at the same buffer.
func (g *game) clearCanvas() {
	if g.imageWidget == nil || g.imageWidget.Image == nil {
		return
	}
	g.imageWidget.Image.Fill(canvasBG)
}

// blitKnotOnCanvas replaces the canvas contents with a white fill
// followed by img scaled uniformly to fit centered. The canvas buffer
// is reused so subsequent pencil/eraser strokes land on the same image
// that scaledImage is rendering.
func (g *game) blitKnotOnCanvas(img *ebiten.Image) {
	if g.imageWidget == nil || g.imageWidget.Image == nil || img == nil {
		return
	}
	canvas := g.imageWidget.Image
	canvas.Fill(canvasBG)
	cb := canvas.Bounds()
	cw, ch := cb.Dx(), cb.Dy()
	ib := img.Bounds()
	iw, ih := ib.Dx(), ib.Dy()
	if iw == 0 || ih == 0 {
		return
	}
	sx := float64(cw) / float64(iw)
	sy := float64(ch) / float64(ih)
	scale := sx
	if sy < sx {
		scale = sy
	}
	dw := float64(iw) * scale
	dh := float64(ih) * scale
	ox := (float64(cw) - dw) / 2
	oy := (float64(ch) - dh) / 2
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Scale(scale, scale)
	opts.GeoM.Translate(ox, oy)
	canvas.DrawImage(img, opts)
}

// loadKnot looks the name up, updates widgets.
func (g *game) loadKnot(name string) {
	k, err := knot.FindKnotByName(name)
	if err != nil {
		g.nameLabel.Label = name + " (not found)"
		g.propsArea.SetText(err.Error())
		return
	}
	g.currentKnot = k
	g.nameLabel.Label = k.GetName()
	g.input.SetText(k.GetName())
	g.refreshImage()
	g.refreshProperties()
}

// refreshImage re-loads the current knot's image for the current style
// and blits it onto the canvas, replacing any prior drawing. Keeps the
// same canvas buffer so the pencil/eraser stay wired up to the image
// being displayed.
func (g *game) refreshImage() {
	if g.currentKnot == nil {
		g.clearCanvas()
		return
	}
	data, kind, err := g.currentKnot.LoadImage(g.currentStyle)
	if err != nil {
		log.Printf("load image: %v", err)
		g.clearCanvas()
		return
	}
	img, _, err := decodeKnotImage(data, kind)
	if err != nil {
		log.Printf("decode image: %v", err)
		g.clearCanvas()
		return
	}
	g.blitKnotOnCanvas(img)
	g.imageWidget.DebugCrossings = nil
	g.imageWidget.DebugArcs = nil
	g.imageWidget.DebugJunctions = nil
	// Loading a fresh raster discards any previous draggable diagram —
	// its crossing/arc coordinates referred to the old canvas content.
	g.imageWidget.Diagram = nil
	// Defer the convertImage extraction to the next Update tick. canvasToImage
	// calls ReadPixels which is illegal before ebiten.RunGame has started, so
	// loadKnot called from main() at startup cannot attach synchronously.
	g.pendingAttach = true
}

// attachDiagramFromCanvas snapshots the canvas, runs convertImage, and
// attaches the resulting Diagram so the Move tool can grab/drag its
// crossings and arcs. Polylines from convertImage are pixel-precise
// (one point per skeleton pixel) which makes the rendered curve jagged
// and lets drag math amplify the noise; we resample each arc to a fixed
// small point count first so the curve stays smooth under interaction.
// Failures are logged and leave Diagram nil.
func (g *game) attachDiagramFromCanvas() {
	raster := g.canvasToImage()
	if raster == nil {
		g.imageWidget.Diagram = nil
		return
	}
	d, err := convertImage(raster)
	if err != nil {
		log.Printf("attach diagram: %v", err)
		g.imageWidget.Diagram = nil
		return
	}
	resampleDiagramArcs(d, attachedArcPoints)
	g.imageWidget.Diagram = d
}

// attachedArcPoints is the number of polyline points each arc is
// resampled to when a Diagram is attached for interaction. Picked so
// Catmull-Rom interpolation produces a clean smooth curve and so the
// drag math has enough handles to deform the arc without compounding
// pixel-grid jitter.
const attachedArcPoints = 13

// resampleDiagramArcs replaces every arc's Polyline with a uniform-arc-
// length resampling to nPoints points. Endpoints are preserved exactly so
// the polyline still anchors at the crossing positions.
func resampleDiagramArcs(d *Diagram, nPoints int) {
	if d == nil {
		return
	}
	for i := range d.Arcs {
		d.Arcs[i].Polyline = resamplePolylineUniform(d.Arcs[i].Polyline, nPoints)
	}
}

// canvasToImage snapshots the current canvas contents into an in-memory
// *image.RGBA so it can be fed to the pure-Go convert pipeline. ebiten
// images store premultiplied RGBA, which matches image.RGBA. Returns
// nil if the canvas is missing.
func (g *game) canvasToImage() *stdimage.RGBA {
	if g.imageWidget == nil || g.imageWidget.Image == nil {
		return nil
	}
	canvas := g.imageWidget.Image
	b := canvas.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil
	}
	pix := make([]byte, 4*w*h)
	canvas.ReadPixels(pix)
	return &stdimage.RGBA{
		Pix:    pix,
		Stride: 4 * w,
		Rect:   stdimage.Rect(0, 0, w, h),
	}
}

// doDebug toggles the diagram-overlay debug view. The first click
// snapshots the current canvas (so any user pencil/eraser strokes are
// included), runs the convert pipeline, and overlays a red circle at
// each detected crossing plus a blue × at each arc's midpoint. If
// convert fails because the skeleton has fused junctions, those
// junction pixels are overlaid as orange circles instead. A second
// click clears whichever overlay is showing.
func (g *game) doDebug() {
	if g.imageWidget == nil {
		return
	}
	if len(g.imageWidget.DebugCrossings) > 0 || len(g.imageWidget.DebugArcs) > 0 || len(g.imageWidget.DebugJunctions) > 0 {
		g.imageWidget.DebugCrossings = nil
		g.imageWidget.DebugArcs = nil
		g.imageWidget.DebugJunctions = nil
		return
	}
	raster := g.canvasToImage()
	if raster == nil {
		g.propsArea.SetText(g.propsArea.GetText() + "debug: no canvas\n")
		return
	}
	d, err := convertImage(raster)
	if err != nil {
		var fje *FusedJunctionsError
		if errors.As(err, &fje) {
			g.imageWidget.DebugJunctions = append([]stdimage.Point(nil), fje.Junctions...)
			return
		}
		g.propsArea.SetText(g.propsArea.GetText() + fmt.Sprintf("debug: convert failed: %v\n", err))
		return
	}
	g.imageWidget.DebugCrossings = append([]stdimage.Point(nil), d.Crossings...)
	marks := make([]debugArcMark, len(d.Arcs))
	for i, a := range d.Arcs {
		marks[i] = debugArcMark{
			At:   arcMidpoint(a.Polyline),
			Info: fmt.Sprintf("A%d: C%d(%s) → C%d(%s)", i, a.Start.Crossing, overLabel(a.Start.Over), a.End.Crossing, overLabel(a.End.Over)),
		}
	}
	g.imageWidget.DebugArcs = marks
}

func overLabel(over bool) string {
	if over {
		return "over"
	}
	return "under"
}

// arcMidpoint returns the point at the path-length midpoint of the
// polyline, interpolated along whichever segment contains the half-
// distance mark. For polylines of length 0 or 1, returns the first
// point (or zero).
func arcMidpoint(poly []stdimage.Point) stdimage.Point {
	if len(poly) == 0 {
		return stdimage.Point{}
	}
	if len(poly) == 1 {
		return poly[0]
	}
	segs := make([]float64, len(poly)-1)
	total := 0.0
	for i := range segs {
		dx := float64(poly[i+1].X - poly[i].X)
		dy := float64(poly[i+1].Y - poly[i].Y)
		segs[i] = math.Hypot(dx, dy)
		total += segs[i]
	}
	half := total / 2
	acc := 0.0
	for i, s := range segs {
		if acc+s >= half {
			t := 0.0
			if s > 0 {
				t = (half - acc) / s
			}
			x := float64(poly[i].X) + t*float64(poly[i+1].X-poly[i].X)
			y := float64(poly[i].Y) + t*float64(poly[i+1].Y-poly[i].Y)
			return stdimage.Point{X: int(x + 0.5), Y: int(y + 0.5)}
		}
		acc += s
	}
	return poly[len(poly)-1]
}

// doConvert runs the knotfolio-style "Convert to diagram" pipeline
// over a snapshot of the current canvas (so any user pencil/eraser
// strokes are included) and replaces the properties area with a
// summary (crossing count, arc count, per-crossing and per-arc
// details). The knot name is re-labelled "Drawing" since the on-screen
// content is now the converted diagram rather than a named KnotInfo
// row.
func (g *game) doConvert() {
	raster := g.canvasToImage()
	g.nameLabel.Label = "Drawing"
	if raster == nil {
		g.propsArea.SetText("convert: no canvas\n")
		return
	}
	d, err := convertImage(raster)
	if err != nil {
		g.propsArea.SetText(fmt.Sprintf("convert failed: %v\n", err))
		return
	}
	resampleDiagramArcs(d, attachedArcPoints)
	g.imageWidget.Diagram = d
	var b strings.Builder
	fmt.Fprintf(&b, "converted at %s\n", time.Now().Format("15:04:05"))
	fmt.Fprintf(&b, "crossings: %d\n", len(d.Crossings))
	fmt.Fprintf(&b, "arcs:      %d\n", len(d.Arcs))
	for i, c := range d.Crossings {
		if i >= 8 {
			fmt.Fprintf(&b, "  ... (%d more)\n", len(d.Crossings)-i)
			break
		}
		fmt.Fprintf(&b, "  C%d = (%d,%d)\n", i, c.X, c.Y)
	}
	for i, a := range d.Arcs {
		if i >= 8 {
			fmt.Fprintf(&b, "  ... (%d more arcs)\n", len(d.Arcs)-i)
			break
		}
		fmt.Fprintf(&b, "  A%d: C%d(%s) -> C%d(%s), %d pts\n",
			i, a.Start.Crossing, overLabel(a.Start.Over), a.End.Crossing, overLabel(a.End.Over), len(a.Polyline))
	}
	g.propsArea.SetText(b.String())
}

// doBeautify takes a snapshot of the current canvas, runs the convert
// pipeline to recover a Diagram, runs the Tutte-embedding beautifier,
// and rasterises the result back onto the canvas. The pre-beautify
// pixels are saved into undoSnap so doUndo can restore them.
func (g *game) doBeautify() {
	raster := g.canvasToImage()
	if raster == nil {
		g.propsArea.SetText("beautify: no canvas\n")
		return
	}
	d, err := convertImage(raster)
	if err != nil {
		g.propsArea.SetText(fmt.Sprintf("beautify: convert failed: %v\n", err))
		return
	}
	canvas := g.imageWidget.Image
	if canvas == nil {
		g.propsArea.SetText("beautify: no canvas image\n")
		return
	}
	g.snapshotCanvas()
	cb := canvas.Bounds()
	bd, err := d.Beautify(cb.Dx(), cb.Dy())
	if err != nil {
		g.propsArea.SetText(fmt.Sprintf("beautify: %v\n", err))
		return
	}
	renderDiagram(canvas, bd, canvasBG)
	// Hand the freshly-laid-out diagram to the canvas widget so the
	// user can grab/drag crossings and arcs. Drag mutations re-render
	// the canvas via OnDiagramChanged.
	g.imageWidget.Diagram = bd
	g.imageWidget.DebugCrossings = nil
	g.imageWidget.DebugArcs = nil
	g.imageWidget.DebugJunctions = nil
	g.nameLabel.Label = "Beautified"
}

// snapshotCanvas copies the current canvas pixels into undoSnap so a
// subsequent doUndo can restore them. Allocates undoSnap on first call
// (or when the canvas size has changed).
func (g *game) snapshotCanvas() {
	canvas := g.imageWidget.Image
	if canvas == nil {
		return
	}
	b := canvas.Bounds()
	if g.undoSnap == nil || g.undoSnap.Bounds() != b {
		g.undoSnap = ebiten.NewImage(b.Dx(), b.Dy())
	}
	g.undoSnap.Clear()
	g.undoSnap.DrawImage(canvas, nil)
}

// doUndo restores the most recent pre-beautify canvas snapshot. No-op if
// no snapshot has been taken yet in this session.
func (g *game) doUndo() {
	if g.undoSnap == nil || g.imageWidget == nil || g.imageWidget.Image == nil {
		return
	}
	canvas := g.imageWidget.Image
	canvas.Clear()
	canvas.DrawImage(g.undoSnap, nil)
}

// refreshProperties renders every Diagram property (except name, shown
// above) as "col: value" lines in the right-pane text area.
func (g *game) refreshProperties() {
	if g.currentKnot == nil {
		g.propsArea.SetText("")
		return
	}
	var b strings.Builder
	for _, c := range knot.ColumnNames() {
		if c == "name" {
			continue
		}
		v := g.currentKnot.Raw(c)
		if v == "" {
			v = "(empty)"
		}
		b.WriteString(c)
		b.WriteString(":\n")
		b.WriteString(v)
		b.WriteString("\n\n")
	}
	g.propsArea.SetText(b.String())
}

// trim is a tiny inline whitespace trimmer so we don't need another import.
func trim(s string) string {
	start, end := 0, len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func buttonImage() *widget.ButtonImage {
	return &widget.ButtonImage{
		Idle:    uiimage.NewNineSliceColor(color.NRGBA{90, 90, 110, 255}),
		Hover:   uiimage.NewNineSliceColor(color.NRGBA{110, 110, 140, 255}),
		Pressed: uiimage.NewNineSliceColor(color.NRGBA{70, 70, 90, 255}),
	}
}

func loadFont(size float64) (etext.Face, error) {
	s, err := etext.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, err
	}
	return &etext.GoTextFace{Source: s, Size: size}, nil
}
