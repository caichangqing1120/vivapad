package compare

import (
	"fmt"
	"image"
	"image/color"
)

type Report struct {
	Width             int     `json:"width"`
	Height            int     `json:"height"`
	Tolerance         uint8   `json:"tolerance"`
	ChangedPixels     int     `json:"changed_pixels"`
	ChangedPercent    float64 `json:"changed_percent"`
	MeanAbsoluteError float64 `json:"mean_absolute_error"`
	MaxChannelDelta   uint8   `json:"max_channel_delta"`
}

func Compare(reference, actual image.Image, tolerance uint8) (Report, *image.NRGBA, error) {
	referenceBounds, actualBounds := reference.Bounds(), actual.Bounds()
	width, height := referenceBounds.Dx(), referenceBounds.Dy()
	if width <= 0 || height <= 0 || width != actualBounds.Dx() || height != actualBounds.Dy() {
		return Report{}, nil, fmt.Errorf("image dimensions differ or are empty: reference %dx%d, actual %dx%d", width, height, actualBounds.Dx(), actualBounds.Dy())
	}
	report := Report{Width: width, Height: height, Tolerance: tolerance}
	difference := image.NewNRGBA(image.Rect(0, 0, width, height))
	var errorSum uint64
	for row := 0; row < height; row++ {
		for column := 0; column < width; column++ {
			referenceColor := onWhite(reference.At(referenceBounds.Min.X+column, referenceBounds.Min.Y+row))
			actualColor := onWhite(actual.At(actualBounds.Min.X+column, actualBounds.Min.Y+row))
			red := delta(referenceColor.R, actualColor.R)
			green := delta(referenceColor.G, actualColor.G)
			blue := delta(referenceColor.B, actualColor.B)
			errorSum += uint64(red) + uint64(green) + uint64(blue)
			maximum := max(red, green, blue)
			if maximum > report.MaxChannelDelta {
				report.MaxChannelDelta = maximum
			}
			if maximum > tolerance {
				report.ChangedPixels++
				difference.SetNRGBA(column, row, color.NRGBA{R: 255, B: max(120, maximum), A: 255})
			} else {
				gray := uint8((int(actualColor.R) + int(actualColor.G) + int(actualColor.B)) / 3)
				difference.SetNRGBA(column, row, color.NRGBA{R: gray, G: gray, B: gray, A: 255})
			}
		}
	}
	pixels := float64(width) * float64(height)
	report.ChangedPercent = float64(report.ChangedPixels) * 100 / pixels
	report.MeanAbsoluteError = float64(errorSum) / (pixels * 3)
	return report, difference, nil
}

func onWhite(value color.Color) color.NRGBA {
	red, green, blue, alpha := value.RGBA()
	return color.NRGBA{
		R: uint8((red + 65535 - alpha + 128) / 257),
		G: uint8((green + 65535 - alpha + 128) / 257),
		B: uint8((blue + 65535 - alpha + 128) / 257),
		A: 255,
	}
}

func delta(first, second uint8) uint8 {
	if first > second {
		return first - second
	}
	return second - first
}
