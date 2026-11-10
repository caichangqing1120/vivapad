package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
)

func main() {
	markFile, err := os.Open("assets/viva-mark.png")
	if err != nil {
		log.Fatal(err)
	}
	mark, err := png.Decode(markFile)
	markFile.Close()
	if err != nil {
		log.Fatal(err)
	}
	icon := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	rounded(icon, image.Rect(64, 64, 960, 960), 184, color.NRGBA{R: 228, G: 228, B: 231, A: 255})
	rounded(icon, image.Rect(70, 70, 954, 954), 178, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
	draw.Draw(icon, image.Rect(252, 230, 252+mark.Bounds().Dx(), 230+mark.Bounds().Dy()), mark, mark.Bounds().Min, draw.Over)
	for _, line := range []struct {
		bounds image.Rectangle
		color  color.NRGBA
	}{
		{image.Rect(295, 691, 694, 713), color.NRGBA{R: 63, G: 63, B: 70, A: 255}},
		{image.Rect(295, 752, 716, 772), color.NRGBA{R: 161, G: 161, B: 170, A: 255}},
		{image.Rect(295, 810, 578, 830), color.NRGBA{R: 161, G: 161, B: 170, A: 255}},
	} {
		rounded(icon, line.bounds, 10, line.color)
	}
	rounded(icon, image.Rect(714, 677, 736, 727), 10, color.NRGBA{R: 215, G: 38, B: 61, A: 255})
	output, err := os.Create("assets/vivapad-icon.png")
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(output, icon); err != nil {
		output.Close()
		log.Fatal(err)
	}
	if err := output.Close(); err != nil {
		log.Fatal(err)
	}
}

func rounded(canvas *image.NRGBA, bounds image.Rectangle, radius int, fill color.NRGBA) {
	for row := bounds.Min.Y; row < bounds.Max.Y; row++ {
		for column := bounds.Min.X; column < bounds.Max.X; column++ {
			centerX := min(max(column, bounds.Min.X+radius), bounds.Max.X-radius-1)
			centerY := min(max(row, bounds.Min.Y+radius), bounds.Max.Y-radius-1)
			deltaX, deltaY := column-centerX, row-centerY
			if deltaX*deltaX+deltaY*deltaY <= radius*radius {
				canvas.SetNRGBA(column, row, fill)
			}
		}
	}
}
