package main

import (
	"errors"
	"os"
	"time"

	"vivapad/internal/document"
)

func (state *model) toggleMonitoring() {
	doc := state.paneDocument(state.activePane)
	if state.monitors[doc] != nil {
		state.stopMonitor(doc)
		state.notice = "Monitoring stopped for " + doc.Name
		return
	}
	if doc.Path == "" {
		state.notice = "Save the document before monitoring it"
		return
	}
	if doc.Dirty() {
		state.notice = "Save changes before monitoring " + doc.Name
		return
	}
	if len(state.monitors) >= 64 {
		state.notice = "Too many monitored documents"
		return
	}
	if err := doc.CheckDiskPath(); err != nil {
		state.notice = "Monitoring unavailable: " + err.Error()
		return
	}
	watcher, err := newFileMonitor(doc.Path, state.invalidator.Invalidate, 250*time.Millisecond)
	if err != nil {
		state.notice = "Monitoring unavailable: " + err.Error()
		return
	}
	if state.monitors == nil {
		state.monitors = make(map[*document.Document]*fileMonitor)
	}
	state.monitors[doc] = watcher
	state.notice = "Monitoring " + doc.Name
	state.invalidator.Invalidate()
}

func (state *model) stopMonitor(doc *document.Document) {
	if watcher := state.monitors[doc]; watcher != nil {
		delete(state.monitors, doc)
		_ = watcher.Close()
	}
}

func (state *model) stopAllMonitors() {
	for doc := range state.monitors {
		state.stopMonitor(doc)
	}
}

// monitoringUpdatedNotice composes the file-monitor success notice from the
// reload observation. historyErr is the error the reload already reported for
// the replacement's change-history source; it is carried verbatim so a caller
// cannot overwrite it with a bare success message. It deliberately does not
// query the document again: the consumer is stateful and a second ChangeHistory
// call would be a new sync, not the same lifecycle observation.
func monitoringUpdatedNotice(name string, historyErr error) string {
	notice := "Monitoring updated " + name
	if historyErr != nil {
		notice += "; " + historyErr.Error()
	}
	return notice
}

func (state *model) consumeMonitorChanges() {
	for doc, watcher := range state.monitors {
		changed, missing, err := watcher.Consume()
		if err != nil || missing {
			state.stopMonitor(doc)
			if err != nil {
				state.notice = "Monitoring stopped: " + err.Error()
			} else {
				state.notice = "Monitoring stopped: file missing or renamed"
			}
			state.invalidator.Invalidate()
			continue
		}
		if !changed {
			continue
		}
		if doc.Dirty() {
			state.stopMonitor(doc)
			state.notice = "Monitoring stopped: unsaved changes"
			state.invalidator.Invalidate()
			continue
		}
		if err := doc.CheckDiskPath(); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				state.stopMonitor(doc)
				state.notice = "Monitoring stopped: " + err.Error()
				state.invalidator.Invalidate()
			}
			continue
		}
		unchanged, err := doc.DiskMatchesSaved()
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			state.notice = "Monitoring read failed: " + err.Error()
			state.invalidator.Invalidate()
			continue
		}
		if unchanged {
			continue
		}
		index := state.documentIndex(doc)
		if index < 0 {
			state.stopMonitor(doc)
			continue
		}
		updated, historyErr := state.reloadDocumentWithHistory(index, false)
		if updated == nil {
			state.invalidator.Invalidate()
			continue
		}
		updated.Editor.MoveTo(updated.Editor.RuneCount(), false)
		for pane := range state.paneDocs {
			if state.paneDocs[pane] == updated && (pane == 0 || state.splitEnabled) {
				state.codeEditorForPane(pane).Reveal(updated.Editor.ByteOffset(updated.Editor.Caret()))
			}
		}
		// Carry the reload's change-history observation into this notice; do not
		// call ChangeHistory again (the consumer is stateful and a second sync is
		// not the same observation).
		state.notice = monitoringUpdatedNotice(updated.Name, historyErr)
		state.invalidator.Invalidate()
	}
}
