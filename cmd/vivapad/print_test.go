package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
	"vivapad/internal/printing"
)

func TestPrintCommandsSnapshotActiveUnsavedSplitViewWithoutEditing(t *testing.T) {
	state := newModel()
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	right.Editor.Insert("a猫b")
	right.Editor.SetSelection(text.Range{Start: 1, End: 2})
	state.splitEnabled, state.activePane = true, 1
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	var received []printing.Job
	state.printJob = func(job printing.Job) (int, error) {
		received = append(received, job)
		return 1, nil
	}
	before, selection := right.Editor.String(), right.Editor.Selection()
	state.command("Print...")
	state.command("Print Now")
	if len(received) != 2 || received[0].Text != before || received[0].Title != right.Name ||
		!received[0].Interactive || received[0].Scope != printing.ScopeSelection ||
		received[0].Selection != (text.Span{Start: 1, End: 4}) ||
		received[1].Text != before || received[1].Interactive || received[1].Scope != printing.ScopeAll ||
		received[1].Selection != (text.Span{}) {
		t.Fatalf("print jobs = %+v", received)
	}
	if right.Editor.String() != before || right.Editor.Selection() != selection || left.Editor.String() != "left" || !strings.Contains(state.notice, "Submitted") {
		t.Fatalf("print command mutated documents: right=%q left=%q notice=%q", right.Editor.String(), left.Editor.String(), state.notice)
	}
}

func TestPrintCommandsUsePersistedSettingsWithoutEditing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	state := newStartupModel(nil, path)
	state.printSettings.LineNumbers = false
	state.printSettings.FormFeedPageBreak = true
	state.printSettings.MarginMM.Top = 12
	state.printSettings.Header.Left = "$(FILE_NAME)"
	if err := state.savePreferences(); err != nil {
		t.Fatal(err)
	}
	restarted := newStartupModel(nil, path)
	doc := restarted.space.Current()
	doc.Editor.Insert("unsaved")
	var received printing.Job
	restarted.printJob = func(job printing.Job) (int, error) { received = job; return 1, nil }
	restarted.command("Print Now")
	if received.Text != "unsaved" || received.Interactive || received.Settings != restarted.printSettings ||
		received.Settings.LineNumbers || !received.Settings.FormFeedPageBreak || received.Settings.MarginMM.Top != 12 ||
		received.Settings.Header.Left != "$(FILE_NAME)" || !doc.Dirty() {
		t.Fatalf("configured print snapshot = %+v, stored = %+v", received, restarted.printSettings)
	}
}

func TestPrintCancelAndUnavailableBackendDoNotClaimSubmission(t *testing.T) {
	state := newModel()
	state.notice = "previous notice"
	state.command("Print...")
	if !strings.Contains(state.notice, "unavailable") {
		t.Fatalf("missing backend notice = %q", state.notice)
	}
	state.notice = "previous notice"
	state.printJob = func(printing.Job) (int, error) { return 0, printing.ErrPrintCanceled }
	state.command("Print...")
	if state.notice != "previous notice" || state.space.Current().Dirty() {
		t.Fatalf("canceled print changed state: %q", state.notice)
	}
	state.printJob = func(printing.Job) (int, error) { return 0, printing.ErrNoPrinter }
	state.command("Print Now")
	if !strings.Contains(state.notice, "no default printer") {
		t.Fatalf("Print Now without default notice = %q", state.notice)
	}
}

func TestCompactFileMenuScrollReachesPrintAndExit(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	state.printJob = func(printing.Job) (int, error) { return 1, nil }
	clickID(t, driver, menuScope.At(0))
	if state.fileMenuVisible >= fileMenuCount() || state.fileMenuVisible < 10 {
		t.Fatalf("File menu visible rows = %d", state.fileMenuVisible)
	}
	first := syncScrollHit(t, driver, menuItemScope.At(1))
	if err := driver.Scroll(ui.ScrollEvent{X: first.X + 20, Y: first.Y + 8, DeltaY: 120}); err != nil || state.fileMenuOffset == 0 {
		t.Fatalf("File menu ignored the wheel: offset=%d, %v", state.fileMenuOffset, err)
	}
	for _, index := range []int{21, 22, 23} {
		row := revealFileMenuItem(t, driver, index)
		if row.Y < menuHeight || row.Y+row.Height > 400-statusHeight || row.Height < 18 {
			t.Fatalf("File row %d clipped: %+v", index, row)
		}
	}
	if first := revealFileMenuItem(t, driver, 0); first.Y < menuHeight {
		t.Fatalf("File menu cannot scroll back to New: %+v", first)
	}
	revealFileMenuItem(t, driver, 22)
	clickID(t, driver, menuItemScope.At(22))
	if !strings.Contains(state.notice, "Submitted") {
		t.Fatalf("Print Now File row did not dispatch: %q", state.notice)
	}
}

func TestCapturePrintFileMenuWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_PRINT_MENU_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_PRINT_MENU_SHOTS to capture File print rows")
	}
	state := newModel()
	web, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer web.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		if err := web.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		clickID(t, web.Driver(), menuScope.At(0))
		revealFileMenuItem(t, web.Driver(), 22)
		if problems := headless.CheckLayout(web.Snapshot()); len(problems) > 0 {
			t.Fatalf("%s File print layout: %+v", size.name, problems)
		}
		if err := web.Screenshot(filepath.Join(output, "print-file-menu-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
		clickID(t, web.Driver(), menuScope.At(0))
	}
}

func fileMenuControls(driver *app.Driver) bool {
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == fileMenuNextID {
			return true
		}
	}
	return false
}

func revealFileMenuItem(t *testing.T, driver *app.Driver, index int) ui.Rect {
	t.Helper()
	for step := 0; step <= len(menuCommands[0]); step++ {
		first := len(menuCommands[0])
		for _, hit := range driver.Snapshot().Hits {
			if hit.ID == menuItemScope.At(index) {
				return hit.Bounds
			}
			for visible := range menuCommands[0] {
				if hit.ID == menuItemScope.At(visible) {
					first = min(first, visible)
				}
			}
		}
		if index < first {
			clickID(t, driver, fileMenuPreviousID)
		} else {
			clickID(t, driver, fileMenuNextID)
		}
	}
	t.Fatalf("File row %d never became visible", index)
	return ui.Rect{}
}
