package main

import (
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
)

func TestCaptureChangeHistoryWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_CHANGE_HISTORY_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_CHANGE_HISTORY_SHOTS to capture change history")
	}
	if err := os.MkdirAll(output, 0700); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	path := filepath.Join(t.TempDir(), "history.txt")
	if err := os.WriteFile(path, []byte("one\ntwo\nthree\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := state.space.Open(path); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	state.showInActivePane(doc)
	doc.Editor.MoveTo(4, false)
	doc.Editor.Insert("X")
	state.command("Save")
	doc.Editor.MoveTo(9, false)
	doc.Editor.Insert("Y")
	doc.Editor.Insert("D")
	doc.Editor.DeleteBackward()
	state.changeHistoryView = "both"
	session, err := headless.Open(newConfig(state), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		state.menuOpen, state.changeHistoryMenuOpen = -1, false
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s editor layout: %+v", size.name, problems)
		}
		shot := filepath.Join(output, "change-history-"+size.name+".png")
		if err := session.Screenshot(shot); err != nil {
			t.Fatal(err)
		}
		file, err := os.Open(shot)
		if err != nil {
			t.Fatal(err)
		}
		image, decodeErr := png.Decode(file)
		file.Close()
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		if image.Bounds().Dx() != size.width || image.Bounds().Dy() != size.height {
			t.Fatalf("%s screenshot dimensions = %v", size.name, image.Bounds())
		}
		var content gfx.Rect
		frame := session.Snapshot()
		for index, hit := range frame.Hits {
			if hit.ID == editorID {
				if geometry, ok := frame.HitTextAt(index); ok {
					content = geometry.Bounds
				}
			}
		}
		if content.Width <= 0 {
			t.Fatal("editor did not publish text geometry")
		}
		first, firstOK := state.codeEditorForPane(0).YAtOffset(4)
		second, secondOK := state.codeEditorForPane(0).YAtOffset(9)
		if !firstOK || !secondOK || second <= first {
			t.Fatal("editor did not publish the changed row positions")
		}
		first += content.Y - state.codeEditorForPane(0).ScrollY()
		second += content.Y - state.codeEditorForPane(0).ScrollY()
		saved, modified, deletion, underline := 0, 0, 0, 0
		for y := image.Bounds().Min.Y; y < image.Bounds().Max.Y; y++ {
			for x := image.Bounds().Min.X; x < image.Bounds().Max.X; x++ {
				r, g, b, a := image.At(x, y).RGBA()
				if a != 0xffff {
					continue
				}
				if r == 0 && g == 0xa0a0 && b == 0 {
					saved++
				}
				if r == 0xffff && g == 0x8080 && b == 0 {
					modified++
					if float32(x) >= content.X && float32(x) < content.X+content.Width && float32(y) >= content.Y && float32(y) < content.Y+content.Height {
						deletion++
					}
				}
				// Saved green at alpha 50/255 over the white text background.
				if r == 0xcdcd && g == 0xecec && b == 0xcdcd && float32(x) >= content.X && float32(y) >= second-2 && float32(y) < second {
					offset, ok := state.codeEditorForPane(0).OffsetAt(gfx.Point{X: float32(x) + 0.5, Y: first + 0.5})
					if !ok || offset < 4 || offset > 5 {
						t.Fatalf("saved underline escaped its first-byte cell at %d,%d", x, y)
					}
					underline++
				}
			}
		}
		if saved == 0 || modified == 0 || deletion == 0 || underline == 0 {
			t.Fatalf("%s screenshot has missing change pixels: saved=%d modified=%d deletion=%d underline=%d", size.name, saved, modified, deletion, underline)
		}
		t.Logf("%s: %dx%d, saved pixels=%d, modified pixels=%d, in-text deletion pixels=%d, translucent underline pixels=%d", size.name, size.width, size.height, saved, modified, deletion, underline)
		clickID(t, session.Driver(), menuScope.At(2))
		clickID(t, session.Driver(), menuItemScope.At(slices.Index(menuCommands[2], "Change History")))
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s submenu layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "change-history-menu-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
}
