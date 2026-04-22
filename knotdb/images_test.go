package knotdb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImagePathShapes(t *testing.T) {
	cases := []struct {
		style string
		want  string
	}{
		{StyleDiagram, filepath.Join("d", "diagrams", "3_1.png")},
		{StyleDiagramMirror, filepath.Join("d", "diagrams", "3_1mirror.png")},
		{StyleSnappy, filepath.Join("d", "diagrams_snappy", "snappyKnot3_1.png")},
		{StyleSnappyMirror, filepath.Join("d", "diagrams_snappy", "snappyMirrorKnot3_1.png")},
		{StyleGrid, filepath.Join("d", "GridDiagramSVG_D", "grid3_1.svg")},
	}
	for _, c := range cases {
		got, err := imagePath("d", "3_1", c.style)
		if err != nil {
			t.Errorf("imagePath(%s): %v", c.style, err)
			continue
		}
		if got != c.want {
			t.Errorf("imagePath(%s) = %q, want %q", c.style, got, c.want)
		}
	}
	if _, err := imagePath("d", "3_1", "bogus"); err == nil {
		t.Error("expected error for unknown style")
	}
}

func TestLoadImageBlob(t *testing.T) {
	datasetDir, _ := filepath.Abs(testDatasetRel)
	useTestDir(t)

	if _, err := os.Stat(filepath.Join(datasetDir, "diagrams")); err != nil {
		t.Skipf("diagrams dir missing: %v", err)
	}

	// 0_1 (unknot) has no images.
	data, err := LoadImageBlob("0_1", StyleDiagram)
	if err != nil {
		t.Fatalf("LoadImageBlob(0_1, diagram): %v", err)
	}
	if len(data) != 0 {
		t.Errorf("0_1 diagram: expected empty, got %d bytes", len(data))
	}

	// 3_1 should have a PNG diagram.
	data, err = LoadImageBlob("3_1", StyleDiagram)
	if err != nil {
		t.Fatalf("LoadImageBlob(3_1, diagram): %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("3_1 diagram: got 0 bytes")
	}
	want := []byte{0x89, 'P', 'N', 'G'}
	for i, b := range want {
		if i >= len(data) || data[i] != b {
			t.Errorf("3_1 diagram byte %d = %#x, want %#x", i, data[i], b)
			break
		}
	}

	// Grid SVG.
	data, err = LoadImageBlob("3_1", StyleGrid)
	if err != nil {
		t.Fatalf("LoadImageBlob(3_1, grid): %v", err)
	}
	if len(data) == 0 {
		t.Errorf("3_1 grid: got 0 bytes")
	}
}
