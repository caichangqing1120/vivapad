package assets

import "testing"

func TestWindowIconLoadsFullResolutionVivaMark(t *testing.T) {
	icon := WindowIcon()
	if icon.Size != 1024 || len(icon.RGBA) != 1024*1024*4 {
		t.Fatalf("icon dimensions = %d and %d bytes", icon.Size, len(icon.RGBA))
	}
	redPixels := 0
	for index := 0; index < len(icon.RGBA); index += 4 {
		if icon.RGBA[index] > 150 && icon.RGBA[index+1] < 100 && icon.RGBA[index+2] < 130 {
			redPixels++
		}
	}
	if redPixels < 80000 {
		t.Fatalf("Viva mark is too small or missing: %d red pixels", redPixels)
	}
	if icon.RGBA[3] != 0 {
		t.Fatalf("outer corner should be transparent, got %v", icon.RGBA[:4])
	}
	face := (240*icon.Size + 240) * 4
	if icon.RGBA[face] < 245 || icon.RGBA[face+1] < 245 || icon.RGBA[face+2] < 245 || icon.RGBA[face+3] != 255 {
		t.Fatalf("Viva light icon face missing: %v", icon.RGBA[face:face+4])
	}
	center := (600*icon.Size + 500) * 4
	if icon.RGBA[center] < 180 || icon.RGBA[center+1] > 80 || icon.RGBA[center+2] > 110 {
		t.Fatalf("Viva mark should stay legible at center: %v", icon.RGBA[center:center+4])
	}
	line := (702*icon.Size + 400) * 4
	if icon.RGBA[line] > 90 || icon.RGBA[line+1] > 90 || icon.RGBA[line+2] > 100 {
		t.Fatalf("editor motif should have a readable graphite line: %v", icon.RGBA[line:line+4])
	}
	caret := (702*icon.Size + 724) * 4
	if icon.RGBA[caret] < 180 || icon.RGBA[caret+1] > 80 || icon.RGBA[caret+2] > 110 {
		t.Fatalf("editor motif should retain a Viva red caret: %v", icon.RGBA[caret:caret+4])
	}
}
