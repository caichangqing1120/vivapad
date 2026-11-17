package main

// H03 application-lifecycle tests: recovery, reload, split clone, macro,
// open-document replace and Save/Save As all consume the one change-history
// consumer bound to the document. A producer gap must be reported, not hidden.
//
// Consumption-sensitive assertions retain the consumer (and a baseline mask)
// BEFORE the lifecycle operation and read LineMask afterwards WITHOUT another
// Sync. If the production path did not consume the operation's frames the
// committed metadata is stale, so LineMask either rejects the new bounds or
// returns the pre-operation mask and the test fails. Only tests where the test
// itself is the next legitimate consumer (Save does not sync by design; H05/H06
// does) use the synced helper.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"vivapad/internal/changehistory"
	"vivapad/internal/document"
	"vivapad/internal/macro"
	"vivapad/internal/recovery"
	"vivapad/internal/searchfiles"
)

// syncedLineMask returns the single-line mask after consuming the document's
// current history. Use it only where the test itself is the next consumer (Save
// publishes the frame but production does not sync) or where the assertion is
// about consumer identity, never to claim that a lifecycle path consumed.
func syncedLineMask(t *testing.T, doc *document.Document) uint8 {
	t.Helper()
	consumer, err := doc.ChangeHistory()
	if err != nil || consumer == nil {
		t.Fatalf("ChangeHistory = %p, %v", consumer, err)
	}
	return lineMaskOn(t, doc, consumer)
}

// retainLineMask consumes the document once and returns an assertion function
// that reads the SAME consumer's single-line mask after an operation without a
// second Sync. It therefore fails (LineMask rejects the stale bounds) when the
// production lifecycle path did not consume the operation's frames.
func retainLineMask(t *testing.T, doc *document.Document) func() (uint8, error) {
	t.Helper()
	consumer, err := doc.ChangeHistory()
	if err != nil || consumer == nil {
		t.Fatalf("ChangeHistory = %p, %v", consumer, err)
	}
	return func() (uint8, error) {
		value := doc.Editor.String()
		if strings.IndexByte(value, '\n') >= 0 {
			t.Fatalf("line mask helper expects a single-line document, got %q", value)
		}
		return consumer.LineMask(0, len(value), len(value))
	}
}

// lineMaskOn reads the committed single-line mask of doc without syncing.
func lineMaskOn(t *testing.T, doc *document.Document, consumer interface {
	LineMask(start, end, next int) (uint8, error)
}) uint8 {
	t.Helper()
	value := doc.Editor.String()
	if strings.IndexByte(value, '\n') >= 0 {
		t.Fatalf("line mask helper expects a single-line document, got %q", value)
	}
	mask, err := consumer.LineMask(0, len(value), len(value))
	if err != nil {
		t.Fatalf("LineMask without a production Sync: %v", err)
	}
	return mask
}

// TestChangeHistoryLifecycleRecoveryUsesRealEdits drives the untitled recovery
// branch: the restored buffer is an actual editor edit (modified editions), never
// an implicit save of the recovered text. Untitled documents are created by the
// recovery itself, so a consumer cannot be retained before the operation; the
// consumption-sensitive recovery evidence is the pre-opened disk-tab test below.
func TestChangeHistoryLifecycleRecoveryUsesRealEdits(t *testing.T) {
	state := newModel()
	state.recoveryPath = filepath.Join(t.TempDir(), "recovery.json")
	stored := recovery.State{Active: 0, Tabs: []recovery.Tab{{Name: "new 1", Content: "restored buffer"}}}
	if err := recovery.Save(state.recoveryPath, stored); err != nil {
		t.Fatal(err)
	}
	state.restoreRecovery(nil)
	doc := state.space.Current()
	if doc.Editor.String() != "restored buffer" {
		t.Fatalf("recovered text = %q", doc.Editor.String())
	}
	if !doc.Dirty() {
		t.Fatal("recovered document is not dirty")
	}
	if mask := syncedLineMask(t, doc); mask != 4 {
		t.Fatalf("recovery marked restored text as saved: mask = %d, want 4 (modified)", mask)
	}
	if strings.Contains(strings.ToLower(state.notice), "unavailable") {
		t.Fatalf("recovery reported an unavailable history: %q", state.notice)
	}
}

// TestChangeHistoryLifecycleRecoveredDiskTabUsesRealEdits covers the other
// recovery branch with consumption sensitivity: the disk tabs already exist
// (pre-opened like a command-line startup), so their consumers are retained
// before restoreRecovery, and the restored masks are read afterwards without a
// test Sync. Two tabs prove the production path consumes each one.
func TestChangeHistoryLifecycleRecoveredDiskTabUsesRealEdits(t *testing.T) {
	state := newModel()
	root := t.TempDir()
	type recoveryTab struct {
		doc     *document.Document
		read    func() (uint8, error)
		content string
	}
	var tabs []recoveryTab
	var stored recovery.State
	stored.Active = 0
	for index, content := range []string{"base one", "base two"} {
		path := filepath.Join(root, "recovered-"+string(rune('a'+index))+".txt")
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if err := state.space.Open(path); err != nil {
			t.Fatal(err)
		}
		doc := state.space.Current()
		state.showInActivePane(doc)
		edited := content + " edited"
		tabs = append(tabs, recoveryTab{doc: doc, read: retainLineMask(t, doc), content: edited})
		stored.Tabs = append(stored.Tabs, recovery.Tab{
			Path: doc.Path, Name: doc.Name, Content: edited,
			DiskDigest: doc.DiskDigest(), Encoding: document.UTF8, DiskEncoding: document.UTF8,
			LineEnding: document.LF, SelectionStart: 0, SelectionEnd: 0,
		})
	}
	state.recoveryPath = filepath.Join(root, "recovery.json")
	if err := recovery.Save(state.recoveryPath, stored); err != nil {
		t.Fatal(err)
	}
	state.restoreRecovery(nil)
	for index, tab := range tabs {
		if state.documentIndex(tab.doc) < 0 {
			t.Fatalf("recovery tab %d did not restore onto the pre-opened document", index)
		}
		if tab.doc.Editor.String() != tab.content {
			t.Fatalf("recovery tab %d text = %q, want %q", index, tab.doc.Editor.String(), tab.content)
		}
		if !tab.doc.Dirty() {
			t.Fatalf("recovery tab %d is not dirty", index)
		}
		if mask, err := tab.read(); err != nil || mask != 4 {
			t.Fatalf("recovery tab %d mask without production Sync = %d/%v, want 4 (modified)", index, mask, err)
		}
		// Repeated ChangeHistory Syncs on the restored source are idempotent: the
		// same consumer keeps answering the same mask, with no invalidation and no
		// additional metadata events.
		first, err := tab.doc.ChangeHistory()
		if err != nil || first == nil {
			t.Fatalf("recovery tab %d first repeated ChangeHistory = %p, %v", index, first, err)
		}
		firstMask := lineMaskOn(t, tab.doc, first)
		second, err := tab.doc.ChangeHistory()
		if err != nil || second != first {
			t.Fatalf("recovery tab %d second repeated ChangeHistory = %p, %v", index, second, err)
		}
		secondMask := lineMaskOn(t, tab.doc, second)
		if firstMask != 4 || secondMask != 4 {
			t.Fatalf("recovery tab %d repeated Syncs changed the restored mask: %d then %d, want 4", index, firstMask, secondMask)
		}
		if mask, err := tab.read(); err != nil || mask != 4 {
			t.Fatalf("recovery tab %d retained mask changed after repeated Syncs: %d/%v", index, mask, err)
		}
	}
}

// TestChangeHistoryLifecycleReloadBindsNewSource drives the real reload path
// and proves the retired document's source is not transferred. The replacement
// is a fresh source, so its consumption is a no-op; the returned observation is
// asserted separately by the monitor tests.
func TestChangeHistoryLifecycleReloadBindsNewSource(t *testing.T) {
	state := newModel()
	path := filepath.Join(t.TempDir(), "reload.txt")
	if err := os.WriteFile(path, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := state.space.Open(path); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	state.showInActivePane(doc)
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert(" local")
	oldConsumer, err := doc.ChangeHistory()
	if err != nil || oldConsumer == nil {
		t.Fatalf("original ChangeHistory = %p, %v", oldConsumer, err)
	}
	readOld := retainLineMask(t, doc)
	if mask, err := readOld(); err != nil || mask != 4 {
		t.Fatalf("edit before reload mask = %d/%v, want 4", mask, err)
	}
	if err := os.WriteFile(path, []byte("after"), 0600); err != nil {
		t.Fatal(err)
	}
	reloaded, historyErr := state.reloadDocumentWithHistory(0, true)
	if reloaded == nil {
		t.Fatalf("reload failed: %q", state.notice)
	}
	if historyErr != nil {
		t.Fatalf("clean reload reported a change-history error: %v", historyErr)
	}
	if reloaded != state.space.Documents[0] {
		t.Fatal("reloadDocumentWithHistory did not return the installed replacement")
	}
	if reloaded == doc {
		t.Fatal("reload did not replace the document")
	}
	freshConsumer, err := reloaded.ChangeHistory()
	if err != nil || freshConsumer == nil {
		t.Fatalf("reloaded ChangeHistory = %p, %v", freshConsumer, err)
	}
	if freshConsumer == oldConsumer {
		t.Fatal("reload transferred the retired change-history source")
	}
	if mask := lineMaskOn(t, reloaded, freshConsumer); mask != 0 {
		t.Fatalf("retired history appeared on the reloaded document: mask = %d, want 0", mask)
	}
	if mask, err := readOld(); err != nil || mask != 4 {
		t.Fatalf("reload disturbed the retired source without a test Sync: mask = %d/%v, want 4", mask, err)
	}
	if state.paneDocs[0] != reloaded {
		t.Fatal("reload did not rebind the active pane")
	}
}

// TestChangeHistoryLifecycleSplitCloneSharesSource proves Clone to Other View
// keeps one document and therefore one consumer for both panes.
func TestChangeHistoryLifecycleSplitCloneSharesSource(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	state.showInActivePane(doc)
	state.command("Clone to Other View")
	if !state.splitEnabled || state.paneDocs[0] != doc || state.paneDocs[1] != doc {
		t.Fatalf("clone state = split %v panes %p/%p doc %p", state.splitEnabled, state.paneDocs[0], state.paneDocs[1], doc)
	}
	left, err := state.paneDocument(0).ChangeHistory()
	if err != nil || left == nil {
		t.Fatalf("left pane ChangeHistory = %p, %v", left, err)
	}
	right, err := state.paneDocument(1).ChangeHistory()
	if err != nil || right == nil {
		t.Fatalf("right pane ChangeHistory = %p, %v", right, err)
	}
	if left != right {
		t.Fatal("Clone to Other View created a second change-history consumer")
	}
	doc.Editor.Insert("shared")
	if mask := syncedLineMask(t, doc); mask != 4 {
		t.Fatalf("shared clone edit mask = %d, want 4", mask)
	}
}

// TestChangeHistoryLifecycleMacroConsumesRealSource plays a macro through the
// real transaction path; the retained consumer proves the production path
// consumed the frame.
func TestChangeHistoryLifecycleMacroConsumesRealSource(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	state.showInActivePane(doc)
	read := retainLineMask(t, doc)
	steps := []macro.Step{{Kind: macro.TextStep, TextKind: app.TextInsert, Text: "macro text"}}
	if !state.playMacroSteps(steps, 1, false) {
		t.Fatalf("macro playback failed: %q", state.notice)
	}
	if doc.Editor.String() != "macro text" {
		t.Fatalf("macro text = %q", doc.Editor.String())
	}
	if mask, err := read(); err != nil || mask != 4 {
		t.Fatalf("macro edit mask without production Sync = %d/%v, want 4 (modified)", mask, err)
	}
	if strings.Contains(strings.ToLower(state.notice), "unavailable") {
		t.Fatalf("macro playback reported an unavailable history: %q", state.notice)
	}
}

// TestChangeHistoryLifecycleGapIsReported proves an unconsumed 256-frame gap is
// surfaced by a lifecycle consumption path instead of being silently dropped.
func TestChangeHistoryLifecycleGapIsReported(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	state.showInActivePane(doc)
	for range 300 {
		doc.Editor.Insert("x")
	}
	steps := []macro.Step{{Kind: macro.TextStep, TextKind: app.TextInsert, Text: "y"}}
	if !state.playMacroSteps(steps, 1, false) {
		t.Fatalf("macro playback failed: %q", state.notice)
	}
	if !strings.Contains(strings.ToLower(state.notice), "change history unavailable") {
		t.Fatalf("256-frame history gap was not surfaced: %q", state.notice)
	}
}

// TestChangeHistoryLifecycleFileReplaceConsumesEdits drives Replace All over two
// open documents and reads both retained consumers without a test Sync, proving
// the production path consumed every changed document.
func TestChangeHistoryLifecycleFileReplaceConsumesEdits(t *testing.T) {
	state := newModel()
	first := state.space.Current()
	state.showInActivePane(first)
	second := state.space.NewDocument()
	state.showInActivePane(second)
	readFirst := retainLineMask(t, first)
	readSecond := retainLineMask(t, second)
	first.Editor.Insert("cat")
	second.Editor.Insert("cat")
	state.query.Insert("cat")
	state.replacement.Insert("dog food")
	state.replaceAllOpenDocuments()
	if first.Editor.String() != "dog food" || second.Editor.String() != "dog food" {
		t.Fatalf("replace all = %q / %q, notice %q", first.Editor.String(), second.Editor.String(), state.notice)
	}
	if mask, err := readFirst(); err != nil || mask != 4 {
		t.Fatalf("first replaced document mask without production Sync = %d/%v, want 4", mask, err)
	}
	if mask, err := readSecond(); err != nil || mask != 4 {
		t.Fatalf("second replaced document mask without production Sync = %d/%v, want 4", mask, err)
	}
}

// TestChangeHistoryLifecycleReplaceAllSurfacesUnavailableHistory covers an
// existing document whose consumer is unavailable for a reachable reason: a
// forced producer gap. The open-document replacement path must consume it and
// surface the explicit error instead of reporting only success.
func TestChangeHistoryLifecycleReplaceAllSurfacesUnavailableHistory(t *testing.T) {
	state := newModel()
	doc := state.space.Current()
	state.showInActivePane(doc)
	for range 300 {
		doc.Editor.Insert("x") // exceed the 256-frame producer ring before any Sync
	}
	state.query.Insert("x")
	state.replacement.Insert("y")
	state.replaceAllOpenDocuments()
	if got := doc.Editor.String(); got != strings.Repeat("y", 300) {
		t.Fatalf("open-document replacement text = %q, want 300 replaced bytes", got)
	}
	if !strings.Contains(strings.ToLower(state.notice), "change history unavailable") {
		t.Fatalf("unavailable existing-document history was reported as success: %q", state.notice)
	}
}

// TestChangeHistoryLifecycleReplaceAllReadOnlyErrorPath pins the replacement
// error path: a read-only document in the plan refuses the whole apply before
// any document changes. No history is reported as consumed or unavailable.
func TestChangeHistoryLifecycleReplaceAllReadOnlyErrorPath(t *testing.T) {
	state := newModel()
	first := state.space.Current()
	state.showInActivePane(first)
	readFirst := retainLineMask(t, first)
	second := state.space.NewDocument()
	state.showInActivePane(second)
	first.Editor.Insert("cat")
	second.Editor.Insert("cat")
	state.readOnly[second] = true
	state.query.Insert("cat")
	state.replacement.Insert("dog")
	state.replaceAllOpenDocuments()
	if !strings.Contains(state.notice, "Read-only document in replacement plan") {
		t.Fatalf("read-only replacement notice = %q", state.notice)
	}
	if first.Editor.String() != "cat" || second.Editor.String() != "cat" {
		t.Fatalf("read-only refusal changed text: %q / %q", first.Editor.String(), second.Editor.String())
	}
	// The refusal happens before any apply, so the pending insert frame stays
	// unconsumed by this path: the retained consumer still sees stale metadata.
	if mask, err := readFirst(); err == nil {
		t.Fatalf("refused replacement consumed the first document's pending edit: mask = %d", mask)
	}
}

// directoryReplaceTab prepares one clean tab the way the Find-in-Files
// replacement tool leaves it before a directory reload: the document is at its
// save point but its consumer has unconsumed frames (insert + Save + insert +
// Undo), and a retained consumer baseline exists from before those frames. The
// replacement must consume the retiring source before releasing it; otherwise
// the retained LineMask sees stale bounds.
type directoryReplaceTab struct {
	doc       *document.Document
	consumer  *changehistory.Consumer
	read      func() (uint8, error)
	path      string
	oldText   string
	freshText string
}

func prepareDirectoryReplaceTab(t *testing.T, state *model, path, content string) directoryReplaceTab {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	if err := state.space.Open(path); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	consumer, err := doc.ChangeHistory()
	if err != nil || consumer == nil {
		t.Fatalf("%s ChangeHistory = %p, %v", path, consumer, err)
	}
	read := retainLineMask(t, doc) // baseline consumer before the pending frames
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert(" local")
	if err := doc.Save(doc.Path); err != nil {
		t.Fatal(err)
	}
	// Keep real, unconsumed history and stay clean: this must remain a reloadable
	// tab whose retiring source is the only one with pending frames.
	doc.Editor.MoveTo(doc.Editor.RuneCount(), false)
	doc.Editor.Insert("!")
	if !doc.Editor.Undo() || doc.Dirty() {
		t.Fatalf("%s did not return to a clean edited state: text %q dirty %v", path, doc.Editor.String(), doc.Dirty())
	}
	return directoryReplaceTab{
		doc: doc, consumer: consumer, read: read, path: doc.Path,
		oldText: content + " local", freshText: content + " after",
	}
}

// TestChangeHistoryLifecycleDirectoryReplaceBindsNewSource drives the directory
// replace path over two tabs. It is consumption-sensitive: the retiring tabs
// have pending frames and their consumers are retained before the replacement,
// so the post-replace LineMask fails with stale bounds if the production path
// did not consume them. It also proves the replacement gets a distinct consumer,
// starts clean, and the retired consumer never observes the new source.
func TestChangeHistoryLifecycleDirectoryReplaceBindsNewSource(t *testing.T) {
	state := newModel()
	root := t.TempDir()
	tabs := []directoryReplaceTab{
		prepareDirectoryReplaceTab(t, state, filepath.Join(root, "replaced-a.txt"), "before one"),
		prepareDirectoryReplaceTab(t, state, filepath.Join(root, "replaced-b.txt"), "before two"),
	}
	state.splitEnabled, state.activePane = true, 0
	state.paneDocs = [2]*document.Document{tabs[0].doc, tabs[1].doc}
	paths := []string{tabs[0].path, tabs[1].path}
	for _, tab := range tabs {
		if err := os.WriteFile(tab.path, []byte(tab.freshText), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.refreshReplacedTabs(searchfiles.ApplyResult{Paths: paths, FilesChanged: len(paths)}); err != nil {
		t.Fatal(err)
	}
	for index, tab := range tabs {
		if mask, err := tab.read(); err != nil || mask != 2 {
			t.Fatalf("directory replace did not consume retiring tab %d: mask = %d/%v, want 2 (saved)", index, mask, err)
		}
		fresh := state.space.Documents[index]
		if fresh == tab.doc {
			t.Fatalf("directory replace %d did not reload the tab: %q", index, state.notice)
		}
		if fresh.Editor.String() != tab.freshText {
			t.Fatalf("directory replace %d text = %q, want %q", index, fresh.Editor.String(), tab.freshText)
		}
		freshConsumer, err := fresh.ChangeHistory()
		if err != nil || freshConsumer == nil {
			t.Fatalf("replaced tab %d ChangeHistory = %p, %v", index, freshConsumer, err)
		}
		if freshConsumer == tab.consumer {
			t.Fatalf("directory replace %d shared the retired consumer with the new source", index)
		}
		if mask := lineMaskOn(t, fresh, freshConsumer); mask != 0 {
			t.Fatalf("directory replace %d inherited history: mask = %d, want 0", index, mask)
		}
		// The retired consumer keeps its own source: edits and Syncs on the
		// replacement must not change what it answers for the retired document.
		fresh.Editor.Insert(" new-source edit")
		if _, err := fresh.ChangeHistory(); err != nil {
			t.Fatalf("replacement %d ChangeHistory: %v", index, err)
		}
		if mask, err := tab.read(); err != nil || mask != 2 {
			t.Fatalf("retired tab %d consumer saw the replacement source: mask = %d/%v, want 2", index, mask, err)
		}
		if tab.doc.Editor.String() != tab.oldText {
			t.Fatalf("retired tab %d text changed: %q, want %q", index, tab.doc.Editor.String(), tab.oldText)
		}
	}
	if state.paneDocs[0] != state.space.Documents[0] || state.paneDocs[1] != state.space.Documents[1] {
		t.Fatal("directory replace did not rebind both panes")
	}
}

// TestChangeHistoryLifecycleDirectoryReplacePartialError covers the partial
// error path: one tab cannot be reloaded (its file was removed) while the other
// is replaced. The replaced tab's retiring source is consumed; the failed tab
// keeps its document and its unconsumed history for the next legitimate consumer.
func TestChangeHistoryLifecycleDirectoryReplacePartialError(t *testing.T) {
	state := newModel()
	root := t.TempDir()
	good := prepareDirectoryReplaceTab(t, state, filepath.Join(root, "good.txt"), "good before")
	bad := prepareDirectoryReplaceTab(t, state, filepath.Join(root, "bad.txt"), "bad before")
	if err := os.WriteFile(good.path, []byte(good.freshText), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(bad.path); err != nil {
		t.Fatal(err)
	}
	err := state.refreshReplacedTabs(searchfiles.ApplyResult{Paths: []string{good.path, bad.path}, FilesChanged: 2})
	if err == nil || !strings.Contains(err.Error(), bad.path) {
		t.Fatalf("partial directory replace error = %v, want the failed tab path", err)
	}
	if mask, err := good.read(); err != nil || mask != 2 {
		t.Fatalf("replaced tab's retiring source was not consumed: mask = %d/%v, want 2", mask, err)
	}
	if state.space.Documents[0] == good.doc || state.space.Documents[0].Editor.String() != good.freshText {
		t.Fatal("the readable tab was not replaced")
	}
	if state.space.Documents[1] != bad.doc || bad.doc.Editor.String() != bad.oldText {
		t.Fatal("the failed tab lost its document")
	}
	// A failed reload leaves the document live, so its pending source must stay
	// for the next legitimate consumer: the retained baseline sees stale bounds.
	if mask, err := bad.read(); err == nil {
		t.Fatalf("failed tab's pending history was consumed by the failed reload: mask = %d", mask)
	}
	freshConsumer, err := state.space.Documents[0].ChangeHistory()
	if err != nil || freshConsumer == nil {
		t.Fatalf("replacement ChangeHistory = %p, %v", freshConsumer, err)
	}
	if freshConsumer == good.consumer {
		t.Fatal("the replaced tab kept the retired consumer")
	}
}

// TestChangeHistoryLifecycleSaveCancelAndFailure pins the Save dialog boundary:
// a cancelled or failed Save/Save As emits no save point; only a successful
// Save As publishes it. Save deliberately does not sync (H05/H06 consumes), so
// these assertions use the synced helper as the next legitimate consumer.
func TestChangeHistoryLifecycleSaveCancelAndFailure(t *testing.T) {
	state, driver := startDriver(t)
	doc := state.space.Current()
	doc.Editor.Insert("draft")
	if mask := syncedLineMask(t, doc); mask != 4 {
		t.Fatalf("unsaved edit mask = %d, want 4", mask)
	}

	if err := driver.QueueSaveFile("", app.ErrCanceled); err != nil {
		t.Fatal(err)
	}
	if state.save(0, true) {
		t.Fatal("cancelled Save As reported success")
	}
	if mask := syncedLineMask(t, doc); mask != 4 {
		t.Fatalf("cancelled Save As emitted a save point: mask = %d, want 4", mask)
	}

	directory := t.TempDir()
	if err := driver.QueueSaveFile(directory, nil); err != nil {
		t.Fatal(err)
	}
	if state.save(0, true) {
		t.Fatal("Save As accepted a directory destination")
	}
	if mask := syncedLineMask(t, doc); mask != 4 {
		t.Fatalf("failed Save As emitted a save point: mask = %d, want 4", mask)
	}

	path := filepath.Join(t.TempDir(), "saved.txt")
	if err := driver.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	if !state.save(0, true) {
		t.Fatalf("Save As failed: %q", state.notice)
	}
	if mask := syncedLineMask(t, doc); mask != 2 {
		t.Fatalf("successful Save As did not publish the save point: mask = %d, want 2", mask)
	}
	if doc.Dirty() {
		t.Fatal("successful Save As left the document dirty")
	}
}
