package main

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"golang.org/x/sys/windows"
)

func TestClassicLightTabColorsMatchWindowsHLS(t *testing.T) {
	convert := windows.NewLazySystemDLL("shlwapi.dll").NewProc("ColorHLSToRGB")
	if err := convert.Find(); err != nil {
		t.Fatal(err)
	}
	// Parameters.h and getIndividualTabColor in Notepad++ v8.9.8.1.
	for i, hue := range [...]uintptr{37, 70, 144, 255, 195} {
		for _, shade := range []struct {
			name                  string
			lightness, saturation uintptr
			got                   gfx.Color
		}{
			{"inactive fill", 210, 150, tabColorPastels[i]},
			{"focused bar", 140, 180, tabColorAccents[i]},
		} {
			ref, _, _ := convert.Call(hue, shade.lightness, shade.saturation)
			// COLORREF stores red in the low byte, then green, then blue.
			want := gfx.RGB(uint8(ref), uint8(ref>>8), uint8(ref>>16))
			if shade.got != want {
				t.Errorf("color %d %s = %+v; Windows returned RGB(%d, %d, %d)", i+1, shade.name, shade.got, uint8(ref), uint8(ref>>8), uint8(ref>>16))
			}
		}
	}
}
