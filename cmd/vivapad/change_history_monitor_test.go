package main

// H03/P2-1 monitoring notice tests.
//
// The monitor reload path composes its own "Monitoring updated" notice after the
// reload. It must carry the change-history error the reload already observed
// instead of overwriting it with a bare success message, and it must never call
// ChangeHistory a second time to re-derive it.
//
// A fresh document.Open cannot currently produce an unavailable history source
// (TrackHistory cannot fail on a final editor before any edit), so the
// unavailable branch of this composition is proven by the pure notice units plus
// the reachable unavailable-document case in the lifecycle tests; no production
// seam is added to fake an end-to-end unavailable monitor reload.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vivapad/internal/document"
)

// TestMonitoringUpdatedNoticeCarriesReloadHistoryError is the direct test of
// the pure composition unit: the reload's already-computed observation is
// carried verbatim next to the success text.
func TestMonitoringUpdatedNoticeCarriesReloadHistoryError(t *testing.T) {
	historyErr := errors.New("change history unavailable: the editor was replaced without binding a new source")
	notice := monitoringUpdatedNotice("tail.log", historyErr)
	if !strings.Contains(notice, "Monitoring updated tail.log") {
		t.Fatalf("composed notice lost the success text: %q", notice)
	}
	if !strings.Contains(notice, historyErr.Error()) {
		t.Fatalf("composed notice dropped the reload history error: %q", notice)
	}
	if got := monitoringUpdatedNotice("tail.log", nil); got != "Monitoring updated tail.log" {
		t.Fatalf("clean notice = %q, want the plain success text", got)
	}
}

// TestReloadedNoticeCarriesHistoryError is the direct test of the reload notice
// unit: the observation returned by reloadDocumentWithHistory is the same one
// composed into its notice, never dropped before composition.
func TestReloadedNoticeCarriesHistoryError(t *testing.T) {
	historyErr := errors.New("change history unavailable: document has no change-history source")
	notice := reloadedNotice("reload.txt", historyErr)
	if !strings.Contains(notice, "Reloaded reload.txt") || !strings.Contains(notice, historyErr.Error()) {
		t.Fatalf("reload notice = %q, want success text plus the carried error", notice)
	}
	if got := reloadedNotice("reload.txt", nil); got != "Reloaded reload.txt" {
		t.Fatalf("clean reload notice = %q", got)
	}
}

// TestMonitoringCleanReloadNoticeHasNoFalseError drives the monitor loop with a
// real reload of a clean tab (the monitor watcher is a test-constructed one so
// the test is deterministic). A clean reload must report the normal monitoring
// notice and must not fabricate a history error.
func TestMonitoringCleanReloadNoticeHasNoFalseError(t *testing.T) {
	state := newModel()
	path := filepath.Join(t.TempDir(), "tail.log")
	if err := os.WriteFile(path, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := state.space.Open(path); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	state.showInActivePane(doc)
	watcher := &fileMonitor{path: doc.Path}
	state.monitors = map[*document.Document]*fileMonitor{doc: watcher}
	watcher.update.Store(monitorFileChanged)
	if err := os.WriteFile(path, []byte("after"), 0600); err != nil {
		t.Fatal(err)
	}
	state.consumeMonitorChanges()
	updated := state.space.Current()
	if updated == doc {
		t.Fatalf("monitor did not reload the changed tab: %q", state.notice)
	}
	if updated.Editor.String() != "after" {
		t.Fatalf("monitor reloaded text = %q, want %q", updated.Editor.String(), "after")
	}
	if state.notice != "Monitoring updated "+updated.Name {
		t.Fatalf("clean monitor reload notice = %q, want the plain success notice", state.notice)
	}
	if state.monitors[updated] != watcher {
		t.Fatal("monitor watcher did not move to the reloaded document")
	}
}

// TestReloadDocumentWithHistoryReportsObservation pins the returned observation
// channel: a clean reload returns a nil history error and composes the plain
// reload notice, while a failed reload returns no replacement and preserves its
// failure notice. The same returned error is what the caller composes into its
// own notice (see TestReloadedNoticeCarriesHistoryError for the non-nil form).
func TestReloadDocumentWithHistoryReportsObservation(t *testing.T) {
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
	doc.Editor.Insert(" local")
	updated, historyErr := state.reloadDocumentWithHistory(0, true)
	if updated == nil {
		t.Fatalf("reload failed: %q", state.notice)
	}
	if updated != state.space.Documents[0] {
		t.Fatal("reloadDocumentWithHistory did not return the installed replacement")
	}
	if historyErr != nil {
		t.Fatalf("clean reload returned a change-history error: %v", historyErr)
	}
	if want := reloadedNotice(updated.Name, historyErr); state.notice != want {
		t.Fatalf("reload notice = %q, want the composition of the returned observation %q", state.notice, want)
	}
	// The monitor's own composition gets the same returned observation.
	if want := monitoringUpdatedNotice(updated.Name, historyErr); want != "Monitoring updated "+updated.Name {
		t.Fatalf("monitor composition of the clean observation = %q", want)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if failed, err := state.reloadDocumentWithHistory(0, true); failed != nil || err != nil {
		t.Fatalf("failed reload = %p, %v", failed, err)
	}
	if !strings.HasPrefix(state.notice, "Reload failed: ") {
		t.Fatalf("failed reload notice = %q", state.notice)
	}
}
