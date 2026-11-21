package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
)

func TestCompactStyleMenusPaintCopyAndNavigateIndependentIndicators(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	doc.Editor.Insert("cat CAT dog")
	doc.Editor.ClearHistory()
	doc.Editor.MoveTo(1, false)
	openStyleMenu := func(index int) {
		t.Helper()
		clickID(t, driver, menuScope.At(2))
		state.searchMenuOffset = 15
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		clickID(t, driver, menuItemScope.At(index))
		if problems := headless.CheckLayout(driver.Snapshot()); len(problems) != 0 {
			t.Fatalf("style submenu layout: %+v", problems)
		}
	}
	openStyleMenu(searchCommandIndex(t, "Style All Occurrences of Token"))
	clickID(t, driver, styleMenuScope.At(0))
	if len(state.styleSpans(doc, 0)) != 2 {
		t.Fatalf("style-all menu = %+v", state.styleSpans(doc, 0))
	}
	colored := 0
	color := styleColors[0]
	color.Alpha = 100.0 / 255
	for _, rectangle := range driver.Snapshot().Primitives {
		if rectangle.Color == color {
			colored++
		}
	}
	if colored != 2 {
		t.Fatalf("style color was not rendered: %d fills", colored)
	}
	if err := driver.Pump(); err != nil || driver.Snapshot().Focused != editorID {
		t.Fatalf("style menu did not return focus to editor: %v focus=%+v", err, driver.Snapshot().Focused)
	}
	openStyleMenu(searchCommandIndex(t, "Copy Styled Text"))
	clickID(t, driver, styleMenuScope.At(0))
	if driver.Clipboard() != "cat\r\nCAT\r\n" {
		t.Fatalf("style copy menu = %q", driver.Clipboard())
	}
	doc.Editor.MoveTo(0, false)
	openStyleMenu(searchCommandIndex(t, "Jump Down"))
	clickID(t, driver, markJumpScope.At(0))
	if doc.Editor.Selection() != (text.Range{Start: 4, End: 7}) {
		t.Fatalf("jump-down style menu = %v", doc.Editor.Selection())
	}
	openStyleMenu(searchCommandIndex(t, "Clear Style"))
	clickID(t, driver, styleMenuScope.At(5))
	if len(state.styleSpans(doc, 0)) != 0 || doc.Editor.CanUndo() || doc.Editor.String() != "cat CAT dog" {
		t.Fatal("Clear all Styles edited the source or left indicators")
	}
}

func TestPersistentFindMarksUsePinnedSourceFillColor(t *testing.T) {
	state, driver := startDriver(t)
	state.space.Current().Editor.Insert("cat cat")
	setMarkQuery(state, "cat")
	state.command("Mark All")
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, primitive := range driver.Snapshot().Primitives {
		if primitive.Color.Red == 1 && primitive.Color.Green == 0 && primitive.Color.Blue == 0 && primitive.Color.Alpha == 100.0/255 {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("source red Find Mark fill count = %d", count)
	}
}

func TestCaptureFiveStyleIndicatorsWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_STYLE_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_STYLE_SHOTS to capture styled indicators")
	}
	state := newModel()
	doc := state.space.Current()
	value := "alpha beta gamma delta epsilon\nalpha beta gamma delta epsilon"
	doc.Editor.Insert(value)
	for style, word := range []string{"alpha", "beta", "gamma", "delta", "epsilon"} {
		start := strings.Index(value, word)
		doc.Editor.SetSelection(text.Range{Start: start, End: start + len(word)})
		state.styleToken(style, true)
	}
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"desktop", 1280, 800}, {"compact", 640, 400},
	} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s styled indicators layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "five-styles-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
	state.menuOpen, state.styleMenuMode, state.searchMenuOffset = 2, "copy", 15
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("compact copy styles menu: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "five-styles-menu-compact.png")); err != nil {
		t.Fatal(err)
	}
	setMarkQuery(state, "alpha")
	state.command("Mark All")
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("compact overlapping Find Mark layout: %+v", problems)
	}
	if err := session.Screenshot(filepath.Join(output, "five-styles-with-findmark-compact.png")); err != nil {
		t.Fatal(err)
	}
}
