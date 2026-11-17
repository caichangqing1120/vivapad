package main

import (
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/headless"
)

func TestChangeHistoryViewUsesRealEditionsAndDisplayModes(t *testing.T) {
	state, _ := openChangeHistoryFixture(t, "one\ntwo\n")
	doc := state.space.Current()
	if state.changeHistoryView != "margin" {
		t.Fatalf("default view = %q, want margin", state.changeHistoryView)
	}
	doc.Editor.MoveTo(4, false)
	doc.Editor.Insert("X")
	check := func(wantMargin, wantIndicators bool) {
		t.Helper()
		snapshot := components.CodeEditorSnapshot{}
		state.applyChangeDecorations(&snapshot, doc)
		decorations, ok := snapshot.Decorations.(*components.CodeEditorDecorations)
		if !ok {
			if wantMargin || wantIndicators {
				t.Fatal("visible changes have no decorations")
			}
			return
		}
		if (len(decorations.GutterBars) != 0) != wantMargin || (len(decorations.IndicatorLayers) != 0) != wantIndicators {
			t.Fatalf("mode %q decorations = %+v", state.changeHistoryView, decorations)
		}
	}
	check(true, false)
	state.command("Change History: Indicators")
	check(false, true)
	state.command("Change History: Both")
	check(true, true)
	state.command("Change History: Off")
	check(false, false)
	state.command("Change History: Margin")
	check(true, false)
	if err := doc.Save(filepath.Join(t.TempDir(), "saved.txt")); err != nil {
		t.Fatal(err)
	}
	if bars := state.changeMarginLines(doc); len(bars) == 0 || bars[0].Fill != palette.changeSaved {
		t.Fatalf("saved edition is not green: %+v", bars)
	}
}

func TestChangeHistoryMarginRendersSavedAndModifiedAtDesktopAndCompactSize(t *testing.T) {
	state, _ := openChangeHistoryFixture(t, "one\ntwo\nthree\n")
	doc := state.space.Current()
	state.showInActivePane(doc)
	doc.Editor.MoveTo(4, false)
	doc.Editor.Insert("X")
	if err := doc.Save(""); err != nil {
		t.Fatal(err)
	}
	doc.Editor.MoveTo(9, false)
	doc.Editor.Insert("Y")
	session, err := headless.Open(newConfig(state), nil, headless.Options{Scale: 1})
	if errors.Is(err, headless.ErrNoRenderer) {
		t.Skip("no offscreen renderer on this platform")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	rendered := 0
	for _, size := range []struct{ width, height int }{{1280, 800}, {640, 400}} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%dx%d layout problems: %+v", size.width, size.height, problems)
		}
		path := filepath.Join(t.TempDir(), "history.png")
		if err := session.Screenshot(path); err != nil {
			if errors.Is(err, headless.ErrNoRenderer) {
				continue
			}
			t.Fatal(err)
		}
		rendered++
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		image, decodeErr := png.Decode(file)
		file.Close()
		if decodeErr != nil {
			t.Fatal(decodeErr)
		}
		saved, modified := 0, 0
		for y := image.Bounds().Min.Y; y < image.Bounds().Max.Y; y++ {
			for x := image.Bounds().Min.X; x < image.Bounds().Max.X; x++ {
				r, g, b, a := image.At(x, y).RGBA()
				if a == 0xffff && r == 0 && g == 0xa0a0 && b == 0 {
					saved++
				}
				if a == 0xffff && r == 0xffff && g == 0x8080 && b == 0 {
					modified++
				}
			}
		}
		if saved == 0 || modified == 0 {
			t.Fatalf("%dx%d margin pixels: saved=%d modified=%d", size.width, size.height, saved, modified)
		}
	}
	if rendered == 0 {
		t.Skip("desktop and compact layouts checked; no offscreen pixels on this platform")
	}
}
