package main

import (
	"crypto/sha256"
	"encoding/hex"
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
		data, err := os.ReadFile(doc.Path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			state.notice = "Monitoring read failed: " + err.Error()
			state.invalidator.Invalidate()
			continue
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) == doc.DiskDigest() {
			continue
		}
		index := state.documentIndex(doc)
		if index < 0 {
			state.stopMonitor(doc)
			continue
		}
		updated := state.reloadDocument(index, false)
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
		state.notice = "Monitoring updated " + updated.Name
		state.invalidator.Invalidate()
	}
}
