package compare

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func filled(width, height int, value color.NRGBA) *image.NRGBA {
	result := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			result.SetNRGBA(x, y, value)
		}
	}
	return result
}

func TestCompareExactAndSinglePixelChange(t *testing.T) {
	reference := filled(2, 2, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	actual := filled(2, 2, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	report, _, err := Compare(reference, actual, 0)
	if err != nil || report.ChangedPixels != 0 || report.MeanAbsoluteError != 0 {
		t.Fatalf("exact match = %+v, %v", report, err)
	}
	actual.SetNRGBA(1, 0, color.NRGBA{A: 255})
	report, diff, err := Compare(reference, actual, 0)
	if err != nil || report.Width != 2 || report.Height != 2 || report.ChangedPixels != 1 ||
		report.ChangedPercent != 25 || math.Abs(report.MeanAbsoluteError-63.75) > 0.001 || report.MaxChannelDelta != 255 {
		t.Fatalf("single pixel = %+v, %v", report, err)
	}
	if got := diff.NRGBAAt(1, 0); got.R < 200 || got.B < 100 || got.A != 255 {
		t.Fatalf("difference pixel = %+v", got)
	}
}

func TestCompareToleranceAndDimensions(t *testing.T) {
	reference := filled(1, 1, color.NRGBA{R: 100, G: 100, B: 100, A: 255})
	actual := filled(1, 1, color.NRGBA{R: 103, G: 100, B: 100, A: 255})
	report, _, err := Compare(reference, actual, 3)
	if err != nil || report.ChangedPixels != 0 || report.MaxChannelDelta != 3 {
		t.Fatalf("tolerant comparison = %+v, %v", report, err)
	}
	if _, _, err := Compare(reference, filled(2, 1, color.NRGBA{A: 255}), 0); err == nil {
		t.Fatal("accepted screenshots with different dimensions")
	}
}
