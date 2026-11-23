package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"vivapad/internal/document"
	"vivapad/internal/session"
)

func waitMonitorEvent(t *testing.T, monitor *fileMonitor, wake <-chan struct{}, missing bool) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-wake:
			changed, deleted, err := monitor.Consume()
			if err != nil {
				t.Fatal(err)
			}
			if changed && deleted == missing {
				return
			}
		case <-deadline:
			t.Fatalf("monitor did not report missing=%v", missing)
		}
	}
}

func TestFileMonitorSignalsAppendReplaceAndPersistentDeletion(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "current.log")
	if err := os.WriteFile(path, []byte("start\n"), 0600); err != nil {
		t.Fatal(err)
	}
	wake := make(chan struct{}, 8)
	monitor, err := newFileMonitor(path, func() {
		select {
		case wake <- struct{}{}:
		default:
		}
	}, 25*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = monitor.Close() })
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("next\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	waitMonitorEvent(t, monitor, wake, false)
	replacement := filepath.Join(root, "replacement.tmp")
	if err := os.WriteFile(replacement, []byte("different\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	waitMonitorEvent(t, monitor, wake, false)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	waitMonitorEvent(t, monitor, wake, true)
	if err := monitor.Close(); err != nil {
		t.Fatal(err)
	}
	for len(wake) > 0 {
		<-wake
	}
	select {
	case <-wake:
		t.Fatal("closed monitor retained a live wake")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestMonitoringKeepsUnchangedBindingAndReloadsSameLengthEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "current.log")
	before := bytes.Repeat([]byte("start\r\n"), 12_000)
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	state.toggleMonitoring()
	t.Cleanup(state.stopAllMonitors)
	watcher := state.monitors[doc]
	if watcher == nil {
		t.Fatalf("monitoring did not start: %q", state.notice)
	}
	doc.Editor.MoveTo(2, false)
	watcher.changed.Store(true)
	state.consumeMonitorChanges()
	if state.space.Current() != doc || doc.Editor.Caret() != 2 {
		t.Fatal("unchanged bytes replaced the document or moved the caret")
	}
	before[len(before)-3] = '!'
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	watcher.changed.Store(true)
	state.consumeMonitorChanges()
	updated := state.space.Current()
	if updated == doc || !strings.HasSuffix(updated.Editor.String(), "star!\n") ||
		updated.Editor.Caret() != updated.Editor.RuneCount() || state.monitors[updated] != watcher {
		t.Fatal("monitor failed to reload the same-length edit and follow its end")
	}
}

func TestMonitoringStopsWhenItsCanonicalPathIsRedirected(t *testing.T) {
	root := t.TempDir()
	path, target := filepath.Join(root, "current.log"), filepath.Join(root, "other.log")
	if err := os.WriteFile(path, []byte("original\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("other\n"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc, originalPath := state.space.Current(), state.space.Current().Path
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	state.toggleMonitoring()
	watcher := state.monitors[doc]
	if watcher == nil {
		t.Fatalf("monitoring did not start: %q", state.notice)
	}
	if err := os.Rename(path, filepath.Join(root, "parked.log")); err != nil {
		t.Fatal(err)
	}
	workspaceFixtureLink(t, target, path)
	watcher.changed.Store(true)
	state.consumeMonitorChanges()
	if len(state.monitors) != 0 || !strings.Contains(state.notice, "Monitoring stopped") {
		t.Errorf("redirect left the monitor active: %q", state.notice)
	}
	if state.space.Current() != doc || doc.Path != originalPath || doc.Editor.String() != "original\n" {
		t.Fatal("monitor rebound the tab to a different file")
	}
}

func BenchmarkMonitoringUnchangedNotification(b *testing.B) {
	for _, tc := range []struct {
		name string
		size int
	}{{"1MiB", 1 << 20}, {"16MiB", 16 << 20}} {
		b.Run(tc.name, func(b *testing.B) {
			path := filepath.Join(b.TempDir(), "current.log")
			line := append(bytes.Repeat([]byte("x"), 127), '\n')
			if err := os.WriteFile(path, bytes.Repeat(line, tc.size/len(line)), 0600); err != nil {
				b.Fatal(err)
			}
			state := newModelWithFiles([]string{path})
			doc := state.space.Current()
			if doc.Path == "" {
				b.Fatalf("fixture was not opened: %q", state.notice)
			}
			watcher := &fileMonitor{}
			state.monitors = map[*document.Document]*fileMonitor{doc: watcher}
			b.ReportAllocs()
			for b.Loop() {
				watcher.changed.Store(true)
				state.consumeMonitorChanges()
				if state.space.Current() != doc {
					b.Fatal("unchanged bytes replaced the document")
				}
			}
		})
	}
}

func waitMonitorState(t *testing.T, driver *app.Driver, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		if ready() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("monitoring state did not advance")
}

func TestMonitoringGuardsDirtyUntitledAndPreservesManualReadOnly(t *testing.T) {
	state, driver := startDriver(t)
	state.command("Monitoring (tail -f)")
	if len(state.monitors) != 0 || state.space.Current().Path != "" {
		t.Fatal("untitled document started monitoring")
	}
	path := filepath.Join(t.TempDir(), "saved.log")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := state.space.Open(path); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	state.showInActivePane(doc)
	doc.Editor.Insert("dirty")
	state.command("Monitoring (tail -f)")
	if len(state.monitors) != 0 || !doc.Dirty() {
		t.Fatal("dirty document started monitoring or lost edits")
	}
	if !doc.Editor.Undo() || doc.Dirty() {
		t.Fatal("test document did not return to saved state")
	}
	state.readOnly[doc] = true
	state.command("Monitoring (tail -f)")
	if state.monitors[doc] == nil || !state.isReadOnly(doc) {
		t.Fatal("saved clean document did not start monitoring")
	}
	state.command("Monitoring (tail -f)")
	if state.monitors[doc] != nil || !state.isReadOnly(doc) {
		t.Fatal("stopping monitoring lost manual read-only mode")
	}
	delete(state.readOnly, doc)
	state.command("Monitoring (tail -f)")
	if state.monitors[doc] == nil || !state.isReadOnly(doc) {
		t.Fatal("monitoring did not make an editable document read-only")
	}
	state.invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, editorID)
	if err := driver.Type("blocked"); err == nil || doc.Editor.String() != "original" {
		t.Fatalf("monitoring allowed text input: %q, %v", doc.Editor.String(), err)
	}
	state.closeTab(state.documentIndex(doc))
	if len(state.monitors) != 0 {
		t.Fatal("closing a monitored tab retained its watcher")
	}
}

func TestMonitoringReloadsCloneAndStopsAfterDeletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tail.log")
	initial := strings.Repeat("old line\n", 80)
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	state.splitEnabled, state.activePane = true, 1
	state.paneDocs = [2]*document.Document{doc, doc}
	doc.Editor.MoveTo(0, false)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	state.command("Monitoring (tail -f)")
	if state.monitors[doc] == nil {
		t.Fatal("monitor did not start")
	}
	updated := initial + "appended\n"
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("appended\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	waitMonitorState(t, driver, func() bool { return state.space.Current().Editor.String() == updated })
	reloaded := state.space.Current()
	if reloaded == doc || state.paneDocs[0] != reloaded || state.paneDocs[1] != reloaded || state.monitors[reloaded] == nil || state.monitors[doc] != nil || !state.isReadOnly(reloaded) || reloaded.Dirty() || reloaded.Editor.Caret() != reloaded.Editor.RuneCount() || reloaded.Editor.CanUndo() {
		t.Fatal("monitor reload lost the clone, EOF, watcher or clean state")
	}
	replacement := filepath.Join(filepath.Dir(path), "replacement.tmp")
	if err := os.WriteFile(replacement, []byte("replacement\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	waitMonitorState(t, driver, func() bool { return state.space.Current().Editor.String() == "replacement\n" })
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	waitMonitorState(t, driver, func() bool { return len(state.monitors) == 0 })
	if state.space.Current().Editor.String() != "replacement\n" || state.isReadOnly(state.space.Current()) || state.space.Current().Dirty() {
		t.Fatal("deleted file lost its last clean contents or stayed read-only")
	}
}

func TestMonitoringBackgroundLegacyReloadAndDecodeFailure(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "legacy.txt")
	otherPath := filepath.Join(root, "other.txt")
	if err := os.WriteFile(path, []byte{0x80}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherPath, []byte("untouched"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModel()
	if err := state.space.OpenWithEncoding(path, document.Windows1252); err != nil {
		t.Fatal(err)
	}
	legacy := state.space.Current()
	state.showInActivePane(legacy)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	state.command("Monitoring (tail -f)")
	if err := state.space.Open(otherPath); err != nil {
		t.Fatal(err)
	}
	other := state.space.Current()
	state.showInActivePane(other)
	if err := os.WriteFile(path, []byte{0x80, 0xa3}, 0600); err != nil {
		t.Fatal(err)
	}
	waitMonitorState(t, driver, func() bool { return state.space.Documents[0].Editor.String() == "€£" })
	if state.space.Current() != other || other.Editor.String() != "untouched" || state.space.Documents[0].Encoding() != document.Windows1252 {
		t.Fatal("background monitoring changed the active tab or encoding")
	}
	previous := state.space.Documents[0]
	if err := os.WriteFile(path, []byte{0x81}, 0600); err != nil {
		t.Fatal(err)
	}
	waitMonitorState(t, driver, func() bool { return strings.Contains(state.notice, "Reload failed") })
	if state.space.Documents[0] != previous || previous.Editor.String() != "€£" || !state.isReadOnly(previous) || previous.Dirty() {
		t.Fatal("invalid external bytes replaced the monitored buffer")
	}
	state.command("Monitoring (tail -f)")
	if !state.isReadOnly(previous) || state.monitors[previous] == nil {
		t.Fatal("monitoring another tab affected the legacy watcher")
	}
	state.stopAllMonitors()
	if len(state.monitors) != 0 || state.isReadOnly(previous) {
		t.Fatal("closing the window retained monitoring state")
	}
	if err := driver.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
}

func TestMonitoringStopsBeforeDiscardingUnexpectedDirtyBuffer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "external.log")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	state.command("Monitoring (tail -f)")
	doc := state.space.Current()
	doc.Editor.Insert("local")
	if err := os.WriteFile(path, []byte("remote"), 0600); err != nil {
		t.Fatal(err)
	}
	waitMonitorState(t, driver, func() bool { return len(state.monitors) == 0 })
	if !doc.Dirty() || !strings.Contains(doc.Editor.String(), "local") || strings.Contains(doc.Editor.String(), "remote") || state.isReadOnly(doc) {
		t.Fatal("external change discarded dirty local text")
	}
	state.command("Monitoring (tail -f)")
	if len(state.monitors) != 0 {
		t.Fatal("dirty buffer restarted monitoring")
	}
}

func TestMonitoringCompactMenuCheckedStateAndClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "monitor.log")
	if err := os.WriteFile(path, []byte("clean"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(3))
	revealViewMenuItem(t, driver, viewCommandIndex("Monitoring (tail -f)"))
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == menuItemScope.At(viewCommandIndex("Monitoring (tail -f)")) && (hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight || hit.Checked) {
			t.Fatalf("Monitoring menu row clipped or prematurely checked: %+v", hit)
		}
	}
	clickID(t, driver, menuItemScope.At(viewCommandIndex("Monitoring (tail -f)")))
	if state.monitors[state.space.Current()] == nil || !state.isReadOnly(state.space.Current()) {
		t.Fatal("View menu did not start monitoring")
	}
	clickID(t, driver, menuScope.At(3))
	revealViewMenuItem(t, driver, viewCommandIndex("Monitoring (tail -f)"))
	checked := false
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == menuItemScope.At(viewCommandIndex("Monitoring (tail -f)")) {
			checked = hit.Checked
		}
	}
	if !checked {
		t.Fatal("Monitoring menu did not show its checked state")
	}
	clickID(t, driver, menuItemScope.At(viewCommandIndex("Monitoring (tail -f)")))
	if len(state.monitors) != 0 || state.isReadOnly(state.space.Current()) {
		t.Fatal("View menu did not stop monitoring cleanly")
	}
}

func TestMonitoringMenuStopReturnsInputToReloadedRightPane(t *testing.T) {
	root := t.TempDir()
	leftPath, rightPath := filepath.Join(root, "peer.txt"), filepath.Join(root, "tail.log")
	for path, value := range map[string]string{leftPath: "peer", rightPath: "old\r\n"} {
		if err := os.WriteFile(path, []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	state := newModelWithFiles([]string{leftPath, rightPath})
	left, right := state.space.Documents[0], state.space.Documents[1]
	state.splitEnabled, state.activePane = true, 1
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.space.Select(1)
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, rightEditorID)
	toggle := func() {
		clickID(t, driver, menuScope.At(3))
		revealViewMenuItem(t, driver, viewCommandIndex("Monitoring (tail -f)"))
		clickID(t, driver, menuItemScope.At(viewCommandIndex("Monitoring (tail -f)")))
	}
	toggle()
	const saved = "new\r\nlast"
	if err := os.WriteFile(rightPath, []byte(saved), 0600); err != nil {
		t.Fatal(err)
	}
	waitMonitorState(t, driver, func() bool { return state.space.Current().Editor.String() == "new\nlast" })
	current := state.space.Current()
	if current == right || state.paneDocs != ([2]*document.Document{left, current}) || !state.isReadOnly(current) || current.Editor.CanUndo() {
		t.Fatal("reload did not bind a fresh, monitored history source to the right pane")
	}
	toggle()
	if state.isReadOnly(current) || state.monitors[current] != nil || current.Editor.Caret() != current.Editor.RuneCount() {
		t.Fatal("stopping monitoring changed the restored editor policy or caret")
	}
	requireEditorCommandFocus(t, driver, rightEditorID)
	if err := driver.Type("x"); err != nil || current.Editor.String() != "new\nlastx" || left.Editor.String() != "peer" {
		t.Fatalf("first edit after stopping monitoring: left=%q right=%q, %v", left.Editor.String(), current.Editor.String(), err)
	}
	if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || current.Editor.String() != "new\nlast" || current.Dirty() {
		t.Fatalf("Undo after stopping monitoring: %q, %v", current.Editor.String(), err)
	}
	if bytes, err := os.ReadFile(rightPath); err != nil || string(bytes) != saved {
		t.Fatalf("monitoring modified the file: %q, %v", bytes, err)
	}
}

func TestCaptureMonitoringWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_MONITOR_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_MONITOR_SHOTS to capture monitored editor")
	}
	path := filepath.Join(t.TempDir(), "service.log")
	if err := os.WriteFile(path, []byte(strings.Repeat("INFO service running\n", 32)), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	state.command("Monitoring (tail -f)")
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	for _, size := range []struct {
		name          string
		width, height int
	}{
		{"desktop", 1280, 800},
		{"compact", 640, 400},
	} {
		if err := session.Resize(size.width, size.height); err != nil {
			t.Fatal(err)
		}
		if size.name == "compact" {
			clickID(t, session.Driver(), menuScope.At(3))
		}
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s layout: %+v", size.name, problems)
		}
		if err := session.Screenshot(filepath.Join(output, "monitoring-"+size.name+".png")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMonitoringDoesNotPersistTransientReadOnly(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "watched.log")
	sessionPath := filepath.Join(root, "settings", "session.json")
	if err := os.WriteFile(path, []byte("clean"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newStartupModel([]string{path}, sessionPath)
	canonical := state.space.Current().Path
	state.command("Monitoring (tail -f)")
	t.Cleanup(state.stopAllMonitors)
	if !state.isReadOnly(state.space.Current()) {
		t.Fatal("monitoring did not enable temporary read-only mode")
	}
	newConfig(state).OnCloseRequest(func() {})
	stored, err := session.Load(sessionPath)
	if err != nil || stored.ReadOnly[canonical] {
		t.Fatalf("session persisted monitoring as manual read-only: %+v, %v", stored.ReadOnly, err)
	}
	restored := newStartupModel(nil, sessionPath)
	if restored.isReadOnly(restored.space.Current()) || len(restored.monitors) != 0 {
		t.Fatal("restart restored a stale monitoring read-only flag")
	}
}

func TestMonitoringNPPExportOmitsRuntimeMonitoringAndReadOnly(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "watched.log")
	if err := os.WriteFile(path, []byte("clean"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	state.readOnly[doc] = true
	state.command("Monitoring (tail -f)")
	t.Cleanup(state.stopAllMonitors)
	stored, err := state.nppSessionState()
	if err != nil || len(stored.Main) != 1 || stored.Main[0].ReadOnly {
		t.Fatalf("monitored NPP export = %+v, %v", stored, err)
	}
	xmlPath := filepath.Join(root, "saved.xml")
	if err := session.SaveNPP(xmlPath, stored); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(xmlPath)
	if err != nil || strings.Contains(string(data), `userReadOnly="yes"`) || strings.Contains(string(data), `monitoring=`) {
		t.Fatalf("NPP XML invented monitoring state: %s, %v", data, err)
	}
	state.command("Monitoring (tail -f)")
	stored, err = state.nppSessionState()
	if err != nil || !stored.Main[0].ReadOnly {
		t.Fatalf("manual read-only flag did not survive stopping monitoring: %+v, %v", stored, err)
	}
}
