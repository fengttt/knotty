package main

import (
	"bytes"
	"image"
	stddraw "image/draw"
	_ "image/gif"
	"os"
	"path/filepath"
	"sort"
	"testing"

	xdraw "golang.org/x/image/draw"
)

// TestLinkGIFsConvert mirrors the UI canvas blit (white 540×540 canvas,
// scaled+centered) and runs convertImage on a sampling of link GIFs.
// The test passes if convert returns a Diagram or a recognised
// FusedJunctionsError / BadTopologyError; it fails only on infinite
// loops or unrecognised errors. Coverage is the head/tail of the
// dataset plus a stride-sample to spot systematic problems.
func TestLinkGIFsConvert(t *testing.T) {
	matches, err := filepath.Glob("../../dataset/links/*.gif")
	if err != nil {
		t.Skip(err)
	}
	if len(matches) == 0 {
		t.Skip("no link GIFs")
	}
	sort.Strings(matches)
	// Sample: first 20, last 20, plus every 100th in between.
	picked := map[string]bool{}
	for i := 0; i < 20 && i < len(matches); i++ {
		picked[matches[i]] = true
	}
	for i := len(matches) - 20; i < len(matches) && i >= 0; i++ {
		picked[matches[i]] = true
	}
	for i := 0; i < len(matches); i += 100 {
		picked[matches[i]] = true
	}
	var keys []string
	for k := range picked {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ok, fused, badtop, otherErr := 0, 0, 0, 0
	for _, p := range keys {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: read: %v", p, err)
		}
		src, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			t.Errorf("%s: decode: %v", p, err)
			continue
		}
		const cw, ch = 540, 540
		canvas := image.NewRGBA(image.Rect(0, 0, cw, ch))
		stddraw.Draw(canvas, canvas.Bounds(), image.NewUniform(image.White),
			image.Point{}, stddraw.Src)
		ib := src.Bounds()
		iw, ih := ib.Dx(), ib.Dy()
		scale := float64(cw) / float64(iw)
		if s := float64(ch) / float64(ih); s < scale {
			scale = s
		}
		dw := int(float64(iw) * scale)
		dh := int(float64(ih) * scale)
		dst := image.Rect((cw-dw)/2, (ch-dh)/2, (cw+dw)/2, (ch+dh)/2)
		xdraw.BiLinear.Scale(canvas, dst, src, ib, xdraw.Over, nil)

		d, err := convertImage(canvas)
		switch {
		case err == nil:
			ok++
			if testing.Verbose() {
				t.Logf("%s ok: %d crossings %d arcs", filepath.Base(p), len(d.Crossings), len(d.Arcs))
			}
		case isFusedErr(err):
			fused++
		case isBadTopErr(err):
			badtop++
		default:
			otherErr++
			t.Errorf("%s: unexpected error: %v", filepath.Base(p), err)
		}
	}
	t.Logf("sampled %d link GIFs: ok=%d fused=%d badtopology=%d other=%d",
		len(keys), ok, fused, badtop, otherErr)
}

func isFusedErr(err error) bool {
	type fjeKind interface{ unwrapFje() }
	_ = fjeKind(nil)
	for ; err != nil; err = unwrap(err) {
		if _, ok := err.(*FusedJunctionsError); ok {
			return true
		}
	}
	return false
}

func isBadTopErr(err error) bool {
	for ; err != nil; err = unwrap(err) {
		if _, ok := err.(*BadTopologyError); ok {
			return true
		}
	}
	return false
}

func unwrap(err error) error {
	type unwrapper interface{ Unwrap() error }
	if u, ok := err.(unwrapper); ok {
		return u.Unwrap()
	}
	return nil
}
