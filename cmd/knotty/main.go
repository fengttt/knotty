// Command knotty is the Ebiten-based GUI for exploring the KnotInfo
// database: search a knot by name (empty = random), display one of its
// five images (Diagram, DiagramMirror, Snappy, SnappyMirror, Grid) on
// the left, and its full knot_info row on the right as a scrollable pane.
package main

import (
	"bytes"
	"fmt"
	"image/color"
	"log"
	stdimage "image"

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
	windowWidth  = 1280
	windowHeight = 800
)

// styleEntry is one of the five image types, shown in the dropdown.
type styleEntry struct {
	label string
	typ   knot.ImageType
}

var styleEntries = []styleEntry{
	{"Diagram", knot.Diagram},
	{"Diagram Mirror", knot.DiagramMirror},
	{"Snappy", knot.Snappy},
	{"Snappy Mirror", knot.SnappyMirror},
	{"Grid", knot.Grid},
}

// game implements ebiten.Game and owns the UI.
type game struct {
	ui *ebitenui.UI

	root        *widget.Container
	input       *widget.TextInput
	imageWidget *widget.Graphic
	infoArea    *widget.TextArea
	titleText   *widget.Text

	currentKnot  *knot.Knot
	currentStyle knot.ImageType

	face    etext.Face
	bigFace etext.Face
}

func main() {
	g := &game{currentStyle: knot.Diagram}

	face, err := loadFont(14)
	if err != nil {
		log.Fatalf("load font: %v", err)
	}
	g.face = face
	bigFace, err := loadFont(18)
	if err != nil {
		log.Fatalf("load font: %v", err)
	}
	g.bigFace = bigFace

	g.ui = g.buildUI()

	// Seed with a random knot so the window has something to show.
	if name, err := knotdb.RandomKnotName(); err == nil {
		g.loadKnot(name)
	} else {
		log.Printf("initial random knot failed: %v", err)
	}

	ebiten.SetWindowSize(windowWidth, windowHeight)
	ebiten.SetWindowTitle("Knotty")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

func (g *game) Layout(outW, outH int) (int, int) { return outW, outH }

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
// Layout:
//
//	root (grid 1 col, rows: 60px top bar, stretched main)
//	├─ topBar: [Search input][Search button][Knot title]
//	└─ main (grid 2 cols, 2/3 | 1/3):
//	    ├─ leftPane: [style dropdown][image]
//	    └─ rightPane: scrollable text area
func (g *game) buildUI() *ebitenui.UI {
	root := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(uiimage.NewNineSliceColor(color.NRGBA{0x1a, 0x1a, 0x1a, 0xff})),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(1),
			widget.GridLayoutOpts.Stretch([]bool{true}, []bool{false, true}),
			widget.GridLayoutOpts.Padding(widget.NewInsetsSimple(8)),
			widget.GridLayoutOpts.Spacing(0, 8),
		)),
	)

	g.root = root
	root.AddChild(g.buildTopBar())
	root.AddChild(g.buildMain())

	return &ebitenui.UI{Container: root}
}

func (g *game) buildTopBar() *widget.Container {
	bar := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(uiimage.NewNineSliceColor(color.NRGBA{0x22, 0x22, 0x2a, 0xff})),
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(8),
			widget.RowLayoutOpts.Padding(widget.NewInsetsSimple(8)),
		)),
	)

	g.input = widget.NewTextInput(
		widget.TextInputOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}),
			widget.WidgetOpts.MinSize(300, 0),
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
		widget.TextInputOpts.Placeholder("Knot name (e.g. 3_1, 4_1). Empty = random."),
		widget.TextInputOpts.SubmitHandler(func(args *widget.TextInputChangedEventArgs) {
			g.doSearch(args.InputText)
		}),
	)
	bar.AddChild(g.input)

	searchBtn := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}),
			widget.WidgetOpts.MinSize(100, 32),
		),
		widget.ButtonOpts.Image(buttonImage()),
		widget.ButtonOpts.Text("Search", &g.face, &widget.ButtonTextColor{Idle: color.NRGBA{240, 240, 240, 255}}),
		widget.ButtonOpts.TextPadding(&widget.Insets{Left: 16, Right: 16, Top: 6, Bottom: 6}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			g.doSearch(g.input.GetText())
		}),
	)
	bar.AddChild(searchBtn)

	g.titleText = widget.NewText(
		widget.TextOpts.Text("", &g.bigFace, color.NRGBA{240, 240, 240, 255}),
		widget.TextOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.RowLayoutData{Position: widget.RowLayoutPositionCenter}),
		),
	)
	bar.AddChild(g.titleText)
	return bar
}

func (g *game) buildMain() *widget.Container {
	main := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(2),
			widget.GridLayoutOpts.Stretch([]bool{true, false}, []bool{true}),
			widget.GridLayoutOpts.Spacing(8, 0),
		)),
	)

	main.AddChild(g.buildLeftPane())
	main.AddChild(g.buildRightPane())
	return main
}

func (g *game) buildLeftPane() *widget.Container {
	left := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(uiimage.NewNineSliceColor(color.NRGBA{0x22, 0x22, 0x2a, 0xff})),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(1),
			widget.GridLayoutOpts.Stretch([]bool{true}, []bool{false, true}),
			widget.GridLayoutOpts.Padding(widget.NewInsetsSimple(8)),
			widget.GridLayoutOpts.Spacing(0, 8),
		)),
	)

	entries := make([]any, len(styleEntries))
	for i, e := range styleEntries {
		entries[i] = e
	}
	combo := widget.NewListComboButton(
		widget.ListComboButtonOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.GridLayoutData{HorizontalPosition: widget.GridLayoutPositionStart}),
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
			MinSize:  &stdimage.Point{X: 200, Y: 0},
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
	left.AddChild(combo)

	// Graphic requires an image at construction; supply a placeholder.
	g.imageWidget = widget.NewGraphic(
		widget.GraphicOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.GridLayoutData{
				HorizontalPosition: widget.GridLayoutPositionCenter,
				VerticalPosition:   widget.GridLayoutPositionCenter,
			}),
		),
		widget.GraphicOpts.Image(placeholderImage("")),
	)
	left.AddChild(g.imageWidget)
	return left
}

func (g *game) buildRightPane() *widget.Container {
	right := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(uiimage.NewNineSliceColor(color.NRGBA{0x22, 0x22, 0x2a, 0xff})),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(1),
			widget.GridLayoutOpts.Stretch([]bool{true}, []bool{true}),
			widget.GridLayoutOpts.Padding(widget.NewInsetsSimple(8)),
		)),
		widget.ContainerOpts.WidgetOpts(
			// Pin right pane to ~1/3 of the window. MaxWidth caps the
			// column width in GridLayout; MinSize prevents shrinking.
			widget.WidgetOpts.LayoutData(widget.GridLayoutData{
				MaxWidth: windowWidth / 3,
			}),
			widget.WidgetOpts.MinSize(windowWidth/3, 0),
		),
	)

	g.infoArea = widget.NewTextArea(
		widget.TextAreaOpts.ContainerOpts(
			widget.ContainerOpts.WidgetOpts(
				widget.WidgetOpts.LayoutData(widget.GridLayoutData{
					HorizontalPosition: widget.GridLayoutPositionStart,
					VerticalPosition:   widget.GridLayoutPositionStart,
				}),
				widget.WidgetOpts.MinSize(windowWidth/3-32, windowHeight-100),
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
	right.AddChild(g.infoArea)
	return right
}

// doSearch handles the search button / Enter key: look up name, or pick
// a random knot if the query is empty or only whitespace.
func (g *game) doSearch(q string) {
	q = trim(q)
	if q == "" {
		name, err := knotdb.RandomKnotName()
		if err != nil {
			g.infoArea.SetText(fmt.Sprintf("random: %v", err))
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
		g.infoArea.SetText(fmt.Sprintf("not found: %q (%v)", name, err))
		g.titleText.Label = name + " (not found)"
		return
	}
	g.currentKnot = k
	g.titleText.Label = k.GetName()
	g.input.SetText(k.GetName())
	g.refreshImage()
	g.refreshInfo()
}

// refreshImage re-loads the current knot's image for the current style.
// After swapping the image, request a relayout so the Graphic widget's
// allocated rect reflects the new preferred size.
func (g *game) refreshImage() {
	if g.currentKnot == nil {
		return
	}
	data, kind, err := g.currentKnot.LoadImage(g.currentStyle)
	if err != nil {
		log.Printf("load image: %v", err)
		g.imageWidget.Image = placeholderImage("")
	} else {
		img, err := decodeKnotImage(data, kind)
		if err != nil {
			log.Printf("decode image: %v", err)
			g.imageWidget.Image = placeholderImage("")
		} else {
			g.imageWidget.Image = img
		}
	}
	if g.root != nil {
		g.root.RequestRelayout()
	}
}

// refreshInfo rebuilds the knot_info display as "col: value" lines,
// skipping empty columns.
func (g *game) refreshInfo() {
	if g.currentKnot == nil {
		g.infoArea.SetText("")
		return
	}
	var b bytes.Buffer
	for _, col := range knot.ColumnNames() {
		v := g.currentKnot.Raw(col)
		if v == "" {
			continue
		}
		fmt.Fprintf(&b, "%s: %s\n", col, v)
	}
	g.infoArea.SetText(b.String())
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
