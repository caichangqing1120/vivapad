package assets

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"image/color"
	"image/png"
	"testing"
)

func TestWindowIconPreservesApprovedVivaPadPNG(t *testing.T) {
	if got := fmt.Sprintf("%x", sha1.Sum(iconPNG)); got != "617b64d0645af3aeb4954e881694a8bc69f4a91c" {
		t.Fatalf("window icon does not embed the approved original PNG: %s", got)
	}
	icon := WindowIcon()
	if icon.Size != 1024 || len(icon.RGBA) != 1024*1024*4 {
		t.Fatalf("icon dimensions = %d and %d bytes", icon.Size, len(icon.RGBA))
	}
	original, err := png.Decode(bytes.NewReader(iconPNG))
	if err != nil {
		t.Fatal(err)
	}
	for row := 0; row < icon.Size; row++ {
		for column := 0; column < icon.Size; column++ {
			want := color.NRGBAModel.Convert(original.At(column, row)).(color.NRGBA)
			offset := (row*icon.Size + column) * 4
			got := icon.RGBA[offset : offset+4]
			if got[0] != want.R || got[1] != want.G || got[2] != want.B || got[3] != want.A {
				t.Fatalf("window icon pixel (%d,%d) = %v, want %+v", column, row, got, want)
			}
		}
	}
}
