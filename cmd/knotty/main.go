// Command knotty is the Ebiten-based GUI for exploring the KnotInfo
// database. The window is 9:16 portrait: a full-width square image of
// the knot on top, and a scrollable properties panel underneath. The
// panel hosts the search input, Search/Refresh buttons, the image-style
// dropdown (Diagram, DiagramMirror, Snappy, SnappyMirror, Grid), the
// knot name, and the raw column values for the current Diagram.
package main

import (
	"bytes"
	"fmt"
	stdimage "image"
	"image/color"
	"log"
	"strings"
	"time"

	"github.com/ebitenui/ebitenui"
	uiimage "github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/fengttt/knotty/knot"
	"github.com/fengttt/knotty/knotdb"
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

// game implements ebiten.Game and owns the UI.
type game struct {
	ui *ebitenui.UI

	root        *widget.Container
	input       *widget.TextInput
	imageWidget *scaledImage
	nameLabel   *widget.Text
	propsArea   *widget.TextArea

	currentKnot   *knot.Diagram
	currentStyle  knot.ImageType
	currentRaster stdimage.Image

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
			widget.GridLayoutOpts.Stretch([]bool{true}, []bool{true}),
		)),
	)

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
	top.AddChild(g.imageWidget)
	return top
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

	refreshBtn := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}),
			widget.WidgetOpts.MinSize(80, 32),
		),
		widget.ButtonOpts.Image(buttonImage()),
		widget.ButtonOpts.Text("Refresh", &g.face, &widget.ButtonTextColor{Idle: color.NRGBA{240, 240, 240, 255}}),
		widget.ButtonOpts.TextPadding(&widget.Insets{Left: 10, Right: 10, Top: 6, Bottom: 6}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			g.doRefresh()
		}),
	)
	row.AddChild(refreshBtn)

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

// doSearch handles the search button / Enter key: look up name, or pick
// a random knot if the query is empty or only whitespace.
func (g *game) doSearch(q string) {
	q = trim(q)
	if q == "" {
		name, err := knotdb.RandomKnotName()
		if err != nil {
			g.propsArea.SetText(fmt.Sprintf("random: %v", err))
			return
		}
		q = name
	}
	g.loadKnot(q)
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

// doRefresh re-renders the properties area and appends a "refreshed at"
// timestamp line at the bottom. The underlying Diagram is not reloaded —
// this is a no-op besides the timestamp.
func (g *game) doRefresh() {
	g.refreshProperties()
	if g.currentKnot == nil {
		return
	}
	cur := g.propsArea.GetText()
	cur += "refreshed at " + time.Now().Format("2006-01-02 15:04:05") + "\n"
	g.propsArea.SetText(cur)
}

// refreshImage re-loads the current knot's image for the current style.
func (g *game) refreshImage() {
	if g.currentKnot == nil {
		g.imageWidget.Image = nil
		return
	}
	data, kind, err := g.currentKnot.LoadImage(g.currentStyle)
	if err != nil {
		log.Printf("load image: %v", err)
		g.imageWidget.Image = nil
		g.currentRaster = nil
		return
	}
	img, raster, err := decodeKnotImage(data, kind)
	if err != nil {
		log.Printf("decode image: %v", err)
		g.imageWidget.Image = nil
		g.currentRaster = nil
		return
	}
	g.imageWidget.Image = img
	g.currentRaster = raster
}

// doConvert runs the knotfolio-style "Convert to diagram" pipeline over
// the currently displayed raster and appends a summary (crossing count,
// arc count, first few crossing positions) to the properties area.
func (g *game) doConvert() {
	if g.currentRaster == nil {
		g.propsArea.SetText(g.propsArea.GetText() + "convert: no image loaded\n")
		return
	}
	d, err := convertImage(g.currentRaster)
	cur := g.propsArea.GetText()
	if err != nil {
		g.propsArea.SetText(cur + fmt.Sprintf("convert failed: %v\n", err))
		return
	}
	var b strings.Builder
	b.WriteString(cur)
	b.WriteString(fmt.Sprintf("--- converted at %s ---\n", time.Now().Format("15:04:05")))
	b.WriteString(fmt.Sprintf("crossings: %d\n", len(d.Crossings)))
	b.WriteString(fmt.Sprintf("arcs:      %d\n", len(d.Arcs)))
	for i, c := range d.Crossings {
		if i >= 8 {
			b.WriteString(fmt.Sprintf("  ... (%d more)\n", len(d.Crossings)-i))
			break
		}
		b.WriteString(fmt.Sprintf("  C%d = (%d,%d)\n", i, c.X, c.Y))
	}
	for i, a := range d.Arcs {
		if i >= 8 {
			b.WriteString(fmt.Sprintf("  ... (%d more arcs)\n", len(d.Arcs)-i))
			break
		}
		startStr := "over"
		if !a.Start.Over {
			startStr = "under"
		}
		endStr := "over"
		if !a.End.Over {
			endStr = "under"
		}
		b.WriteString(fmt.Sprintf("  A%d: C%d(%s) -> C%d(%s), %d pts\n",
			i, a.Start.Crossing, startStr, a.End.Crossing, endStr, len(a.Polyline)))
	}
	g.propsArea.SetText(b.String())
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
