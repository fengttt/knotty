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

	root         *widget.Container
	input        *widget.TextInput
	imageWidget  *scaledImage
	nameLabel    *widget.Text
	propCombo    *widget.ListComboButton
	valueArea    *widget.TextArea
	computeInput *widget.TextInput
	resultArea   *widget.TextArea
	titleText    *widget.Text

	currentKnot  *knot.Knot
	currentStyle knot.ImageType
	currentProp  string

	face     etext.Face
	bigFace  etext.Face
	hugeFace etext.Face
}

func main() {
	g := &game{currentStyle: knot.Diagram, currentProp: "crossing_number"}

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

	// scaledImage fills the stretched cell and scales the knot image
	// uniformly to fit, preserving aspect ratio.
	g.imageWidget = newScaledImage(
		widget.WidgetOpts.LayoutData(widget.GridLayoutData{
			HorizontalPosition: widget.GridLayoutPositionStart,
			VerticalPosition:   widget.GridLayoutPositionStart,
		}),
	)
	left.AddChild(g.imageWidget)
	return left
}

func (g *game) buildRightPane() *widget.Container {
	right := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(uiimage.NewNineSliceColor(color.NRGBA{0x22, 0x22, 0x2a, 0xff})),
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(1),
			// 5 rows: name, property dropdown, property value (stretched),
			// compute section (input + 2 buttons), compute result (stretched).
			widget.GridLayoutOpts.Stretch([]bool{true}, []bool{false, false, true, false, true}),
			widget.GridLayoutOpts.Padding(widget.NewInsetsSimple(8)),
			widget.GridLayoutOpts.Spacing(0, 8),
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

	g.nameLabel = widget.NewText(
		widget.TextOpts.Text("", &g.hugeFace, color.NRGBA{240, 240, 240, 255}),
		widget.TextOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.GridLayoutData{
				HorizontalPosition: widget.GridLayoutPositionCenter,
			}),
		),
	)
	right.AddChild(g.nameLabel)

	// Dropdown to pick which knot_info property to display. Excludes
	// "name" (shown above).
	cols := knot.ColumnNames()
	propEntries := make([]any, 0, len(cols))
	var defaultEntry any
	for _, c := range cols {
		if c == "name" {
			continue
		}
		propEntries = append(propEntries, c)
		if c == g.currentProp {
			defaultEntry = c
		}
	}
	if defaultEntry == nil && len(propEntries) > 0 {
		defaultEntry = propEntries[0]
		g.currentProp = defaultEntry.(string)
	}

	g.propCombo = widget.NewListComboButton(
		widget.ListComboButtonOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.GridLayoutData{HorizontalPosition: widget.GridLayoutPositionStart}),
		),
		widget.ListComboButtonOpts.Entries(propEntries),
		widget.ListComboButtonOpts.MaxContentHeight(300),
		widget.ListComboButtonOpts.ButtonParams(&widget.ButtonParams{
			Image:       buttonImage(),
			TextPadding: widget.NewInsetsSimple(6),
			TextColor: &widget.ButtonTextColor{
				Idle:     color.NRGBA{240, 240, 240, 255},
				Disabled: color.NRGBA{160, 160, 160, 255},
			},
			TextFace: &g.face,
			MinSize:  &stdimage.Point{X: windowWidth/3 - 32, Y: 0},
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
			func(e any) string { return e.(string) },
			func(e any) string { return e.(string) },
		),
		widget.ListComboButtonOpts.EntrySelectedHandler(func(args *widget.ListComboButtonEntrySelectedEventArgs) {
			g.currentProp = args.Entry.(string)
			g.refreshValue()
		}),
	)
	if defaultEntry != nil {
		g.propCombo.SetSelectedEntry(defaultEntry)
	}
	right.AddChild(g.propCombo)

	g.valueArea = widget.NewTextArea(
		widget.TextAreaOpts.ContainerOpts(
			widget.ContainerOpts.WidgetOpts(
				widget.WidgetOpts.LayoutData(widget.GridLayoutData{
					HorizontalPosition: widget.GridLayoutPositionStart,
					VerticalPosition:   widget.GridLayoutPositionStart,
				}),
				widget.WidgetOpts.MinSize(windowWidth/3-32, 200),
			),
		),
		widget.TextAreaOpts.ControlWidgetSpacing(4),
		widget.TextAreaOpts.FontColor(color.NRGBA{230, 230, 230, 255}),
		widget.TextAreaOpts.FontFace(&g.bigFace),
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
	right.AddChild(g.valueArea)

	right.AddChild(g.buildComputeSection())

	g.resultArea = widget.NewTextArea(
		widget.TextAreaOpts.ContainerOpts(
			widget.ContainerOpts.WidgetOpts(
				widget.WidgetOpts.LayoutData(widget.GridLayoutData{
					HorizontalPosition: widget.GridLayoutPositionStart,
					VerticalPosition:   widget.GridLayoutPositionStart,
				}),
				widget.WidgetOpts.MinSize(windowWidth/3-32, 120),
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
	right.AddChild(g.resultArea)

	return right
}

// buildComputeSection is the per-knot compute UI on the right pane: a
// property-name input and two buttons that trigger compute-from-image
// and compute-from-text. Since knot algorithms are NYI, the handlers
// just record what they would have computed into resultArea.
func (g *game) buildComputeSection() *widget.Container {
	sec := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewGridLayout(
			widget.GridLayoutOpts.Columns(1),
			widget.GridLayoutOpts.Stretch([]bool{true}, []bool{false, false}),
			widget.GridLayoutOpts.Spacing(0, 6),
		)),
	)

	g.computeInput = widget.NewTextInput(
		widget.TextInputOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.GridLayoutData{HorizontalPosition: widget.GridLayoutPositionStart}),
			widget.WidgetOpts.MinSize(windowWidth/3-32, 0),
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
		widget.TextInputOpts.Placeholder("Property to compute (e.g. jones_polynomial)"),
	)
	sec.AddChild(g.computeInput)

	btnRow := widget.NewContainer(
		widget.ContainerOpts.Layout(widget.NewRowLayout(
			widget.RowLayoutOpts.Direction(widget.DirectionHorizontal),
			widget.RowLayoutOpts.Spacing(8),
		)),
	)
	fromImgBtn := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(0, 32),
		),
		widget.ButtonOpts.Image(buttonImage()),
		widget.ButtonOpts.Text("Compute from Image", &g.face, &widget.ButtonTextColor{Idle: color.NRGBA{240, 240, 240, 255}}),
		widget.ButtonOpts.TextPadding(&widget.Insets{Left: 10, Right: 10, Top: 4, Bottom: 4}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			g.computeFromImage()
		}),
	)
	fromTxtBtn := widget.NewButton(
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.MinSize(0, 32),
		),
		widget.ButtonOpts.Image(buttonImage()),
		widget.ButtonOpts.Text("Compute from Text", &g.face, &widget.ButtonTextColor{Idle: color.NRGBA{240, 240, 240, 255}}),
		widget.ButtonOpts.TextPadding(&widget.Insets{Left: 10, Right: 10, Top: 4, Bottom: 4}),
		widget.ButtonOpts.ClickedHandler(func(args *widget.ButtonClickedEventArgs) {
			g.computeFromText()
		}),
	)
	btnRow.AddChild(fromImgBtn)
	btnRow.AddChild(fromTxtBtn)
	sec.AddChild(btnRow)

	return sec
}

// doSearch handles the search button / Enter key: look up name, or pick
// a random knot if the query is empty or only whitespace.
func (g *game) doSearch(q string) {
	q = trim(q)
	if q == "" {
		name, err := knotdb.RandomKnotName()
		if err != nil {
			g.valueArea.SetText(fmt.Sprintf("random: %v", err))
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
		g.titleText.Label = name + " (not found)"
		g.valueArea.SetText(err.Error())
		return
	}
	g.currentKnot = k
	g.nameLabel.Label = k.GetName()
	g.titleText.Label = k.GetName()
	g.input.SetText(k.GetName())
	g.refreshImage()
	g.refreshValue()
	if g.resultArea != nil {
		g.resultArea.SetText("")
	}
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
		return
	}
	img, err := decodeKnotImage(data, kind)
	if err != nil {
		log.Printf("decode image: %v", err)
		g.imageWidget.Image = nil
		return
	}
	g.imageWidget.Image = img
}

// computeFromImage simulates computing the requested property from the
// current knot's left-pane image. Knot algorithms are NYI, so the
// handler just records the inputs and emits a NYI message.
func (g *game) computeFromImage() {
	prop := trim(g.computeInput.GetText())
	if prop == "" {
		g.resultArea.SetText("compute from image: no property specified")
		return
	}
	imgDesc := "(no image)"
	if g.imageWidget != nil && g.imageWidget.Image != nil {
		b := g.imageWidget.Image.Bounds()
		imgDesc = fmt.Sprintf("image %dx%d (style=%s)", b.Dx(), b.Dy(), g.currentStyle)
	}
	g.resultArea.SetText(fmt.Sprintf(
		"Compute from Image\nproperty: %s\ninput: %s\n\nnot yet implemented",
		prop, imgDesc,
	))
}

// computeFromText simulates computing the requested property from the
// text currently shown in valueArea (i.e. the dropdown-selected
// property's value). Knot algorithms are NYI.
func (g *game) computeFromText() {
	prop := trim(g.computeInput.GetText())
	if prop == "" {
		g.resultArea.SetText("compute from text: no property specified")
		return
	}
	src := g.valueArea.GetText()
	g.resultArea.SetText(fmt.Sprintf(
		"Compute from Text\nproperty: %s\ninput property: %s\ninput text:\n%s\n\nnot yet implemented",
		prop, g.currentProp, src,
	))
}

// refreshValue shows the currently-selected knot_info property's value
// in the right pane's text area.
func (g *game) refreshValue() {
	if g.currentKnot == nil || g.currentProp == "" {
		g.valueArea.SetText("")
		return
	}
	v := g.currentKnot.Raw(g.currentProp)
	if v == "" {
		v = "(empty)"
	}
	g.valueArea.SetText(v)
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
