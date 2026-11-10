package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"vivapad/internal/compare"
)

func writeFixturePNG(t *testing.T, path string, picture image.Image) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, picture); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCLIReportsDiffAndFailsConfiguredGate(t *testing.T) {
	root := t.TempDir()
	referencePath, actualPath, diffPath := filepath.Join(root, "reference.png"), filepath.Join(root, "actual.png"), filepath.Join(root, "diff.png")
	reference := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	actual := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for row := 0; row < 2; row++ {
		for column := 0; column < 2; column++ {
			reference.SetNRGBA(column, row, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			actual.SetNRGBA(column, row, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	actual.SetNRGBA(0, 0, color.NRGBA{A: 255})
	writeFixturePNG(t, referencePath, reference)
	writeFixturePNG(t, actualPath, actual)
	var output bytes.Buffer
	err := run([]string{"-reference", referencePath, "-actual", actualPath, "-diff", diffPath, "-max-changed-percent", "0"}, &output)
	if err == nil {
		t.Fatal("one changed pixel passed a zero-percent gate")
	}
	var report compare.Report
	if err := json.Unmarshal(output.Bytes(), &report); err != nil || report.ChangedPercent != 25 {
		t.Fatalf("report=%+v decode=%v", report, err)
	}
	if _, err := os.Stat(diffPath); err != nil {
		t.Fatalf("difference image missing: %v", err)
	}
	output.Reset()
	if err := run([]string{"-reference", referencePath, "-actual", actualPath, "-diff", diffPath, "-tolerance", "255", "-max-changed-percent", "0"}, &output); err != nil {
		t.Fatalf("tolerant gate failed: %v", err)
	}
}
