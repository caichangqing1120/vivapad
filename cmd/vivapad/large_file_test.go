package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

func TestLargeFileDriverWorkflow(t *testing.T) {
	oldLight, oldLarge := document.LightweightThreshold, document.LargeFileThreshold
	document.LightweightThreshold, document.LargeFileThreshold = 32, 256
	t.Cleanup(func() { document.LightweightThreshold, document.LargeFileThreshold = oldLight, oldLarge })
	root := t.TempDir()
	state, driver := startDriver(t)
	for index, fixture := range []struct {
		value        string
		light, large bool
	}{{"short", false, false}, {strings.Repeat("abc\n", 64), true, false}, {strings.Repeat("info 0123456789\r\n", 100) + "ERROR last\nmore error\n", false, true}} {
		path := filepath.Join(root, fmt.Sprintf("test-%d.go", index))
		if err := os.WriteFile(path, []byte(fixture.value), 0600); err != nil {
			t.Fatal(err)
		}
		if !state.openDocumentPaths([]string{path}) {
			t.Fatal(state.notice)
		}
		doc := state.space.Current()
		if doc.Lightweight() != fixture.light || (doc.LargeFile() != nil) != fixture.large {
			t.Fatalf("size %d: wrong mode", len(fixture.value))
		}
		if fixture.light {
			doc.Editor.Insert("changed ")
			if _, ok := state.recoverySnapshot(doc); ok {
				t.Fatal("lightweight edit was snapshotted")
			}
			if _, err := doc.ChangeHistory(); err == nil || !state.save(state.space.Active, false) {
				t.Fatalf("history = %v; save = %s", err, state.notice)
			}
		}
	}
	doc := state.space.Current()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	assertEditorHitReadOnly(t, driver, editorID, true)
	clickID(t, driver, editorID)
	if _, err := driver.ShortcutChord(primary(), 'g'); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("101"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if view := state.largeView(doc, 0); view.top < 60 || !strings.Contains(view.value, "ERROR last") {
		t.Fatalf("Go to did not move to line 101: top %d", view.top)
	}
	if _, err := driver.ShortcutChord(primary(), 'f'); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("error"); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, findAllCurrentID)
	waitMonitorState(t, driver, func() bool { return len(state.fileResults.Matches) == 2 })
	clickID(t, driver, state.resultRowID(1))
	waitMonitorState(t, driver, func() bool { return state.largeView(doc, 0).selectedText() == "error" })
	if _, err := driver.ShortcutChord(primary(), 'c'); err != nil || driver.Clipboard() != "error" {
		t.Fatalf("copy = %q, %v", driver.Clipboard(), err)
	}
	if _, err := driver.ShortcutChord(primary(), 'v'); err != nil || !strings.Contains(state.notice, "Read-Only") {
		t.Fatalf("paste guard = %q, %v", state.notice, err)
	}
	if err := doc.Save(""); !errors.Is(err, document.ErrLargeFileReadOnly) {
		t.Fatalf("save guard: %v", err)
	}
	state.command("Find Previous")
	waitMonitorState(t, driver, func() bool { return state.largeView(doc, 0).selectedText() == "ERROR" })
	state.command("Find Next")
	waitMonitorState(t, driver, func() bool { return state.largeView(doc, 0).selectedText() == "error" })
	view := state.largeView(doc, 0)
	lastTop := view.bottom(doc.LineCount())
	barX := view.bar.X + view.bar.Width/2
	if err := driver.Press(barX, view.thumb.Y+view.thumb.Height/2, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(barX, view.bar.Y+1); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(barX, view.bar.Y+1, gfx.PointerButtonPrimary); err != nil || view.top != 1 {
		t.Fatalf("scrollbar drag to start: top %d, %v", view.top, err)
	}
	if err := driver.Scroll(gfx.ScrollEvent{X: barX, Y: view.bar.Y + 8, DeltaY: view.rowHeight / 2, Precise: true}); err != nil || view.fraction <= 0 {
		t.Fatalf("precise scroll: fraction %f, %v", view.fraction, err)
	}
	if err := driver.Click(barX, view.bar.Y+view.bar.Height-1); err != nil || view.top != lastTop {
		t.Fatalf("scrollbar click to end: top %d, want %d, %v", view.top, lastTop, err)
	}
	state.command("Monitoring (tail -f)")
	writer, err := os.OpenFile(doc.Path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.WriteString("TAIL ADDED\r\n"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	waitMonitorState(t, driver, func() bool {
		return strings.Contains(state.largeView(doc, 0).value, "TAIL ADDED") && doc.LargeFile().Progress().Lines == 104
	})
	engine := doc.LargeFile()
	state.closeTab(state.space.Active)
	if state.monitors[doc] != nil {
		t.Fatal("closed tab still monitored")
	}
	if _, err := engine.Window(1, 1, 0, 0); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed tab engine: %v", err)
	}
	if !state.openDocumentPaths([]string{doc.Path}) {
		t.Fatal(state.notice)
	}
	reopened := state.space.Current()
	state.command("Monitoring (tail -f)")
	driver.Close()
	if len(state.monitors) != 0 {
		t.Fatal("window shutdown left watchers")
	}
	if _, err := reopened.LargeFile().Window(1, 1, 0, 0); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("window shutdown left engine open: %v", err)
	}
}

func BenchmarkLargeFileDriverFirstPaint200MiB(bench *testing.B) {
	path := filepath.Join(bench.TempDir(), "large.log")
	file, err := os.Create(path)
	if err != nil {
		bench.Fatal(err)
	}
	block := strings.Repeat(strings.Repeat("a", 126)+"\r\n", 8192)
	for range 200 {
		if _, err := file.WriteString(block); err != nil {
			_ = file.Close()
			bench.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		bench.Fatal(err)
	}
	state := newModel()
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		bench.Fatal(err)
	}
	defer driver.Close()
	for bench.Loop() {
		if !state.openDocumentPaths([]string{path}) {
			bench.Fatal(state.notice)
		}
		if err := driver.Pump(); err != nil {
			bench.Fatal(err)
		}
		bench.StopTimer()
		if view := state.largeView(state.space.Current(), 0); !strings.HasPrefix(view.value, "aaaa") || view.loadedLines < view.rows {
			bench.Fatalf("first frame: only %d/%d lines", view.loadedLines, view.rows)
		}
		state.closeTab(state.space.Active)
		bench.StartTimer()
	}
}
