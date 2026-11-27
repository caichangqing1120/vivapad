package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
)

const externalBarHeight = 31

var (
	externalReloadID  = tree.ID(page, "external-reload")
	externalCompareID = tree.ID(page, "external-compare")
	externalIgnoreID  = tree.ID(page, "external-ignore")
)

type externalBinding struct {
	path, baseline, ignored string
}

type externalWatcher struct {
	source *fsnotify.Watcher
	done   chan struct{}
	dirs   map[string]int
	mu     sync.Mutex
	paths  map[string]int
	ready  map[string]bool
	err    error
}

func watchKey(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		path = strings.ToLower(path)
	}
	return path
}

func newExternalWatcher(wake func()) (*externalWatcher, error) {
	source, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	watcher := &externalWatcher{source: source, done: make(chan struct{}), dirs: make(map[string]int), paths: make(map[string]int), ready: make(map[string]bool)}
	go watcher.run(wake)
	return watcher, nil
}

func (watcher *externalWatcher) run(wake func()) {
	defer close(watcher.done)
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	defer timer.Stop()
	var due <-chan time.Time
	pending := make(map[string]bool)
	for {
		select {
		case event, ok := <-watcher.source.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename|fsnotify.Chmod) == 0 {
				continue
			}
			path := watchKey(event.Name)
			watcher.mu.Lock()
			if watcher.paths[path] > 0 {
				pending[path] = true
			} else if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				for child := range watcher.paths {
					if watchKey(filepath.Dir(child)) == path {
						pending[child] = true
					}
				}
			}
			watcher.mu.Unlock()
			if len(pending) > 0 && due == nil {
				timer.Reset(90 * time.Millisecond)
				due = timer.C
			}
		case <-due:
			due = nil
			watcher.mu.Lock()
			for path := range pending {
				if watcher.paths[path] > 0 {
					watcher.ready[path] = true
				}
			}
			watcher.mu.Unlock()
			clear(pending)
			wake()
		case err, ok := <-watcher.source.Errors:
			if !ok {
				return
			}
			watcher.mu.Lock()
			watcher.err = err
			for path := range watcher.paths {
				watcher.ready[path] = true
			}
			watcher.mu.Unlock()
			wake()
		}
	}
}

func (watcher *externalWatcher) add(path string) error {
	dir := filepath.Dir(path)
	key := watchKey(dir)
	watcher.mu.Lock()
	watcher.paths[watchKey(path)]++
	watcher.mu.Unlock()
	if watcher.dirs[key] == 0 {
		if err := watcher.source.Add(dir); err != nil {
			watcher.mu.Lock()
			delete(watcher.paths, watchKey(path))
			watcher.mu.Unlock()
			return err
		}
	}
	watcher.dirs[key]++
	return nil
}

func (watcher *externalWatcher) remove(path string) {
	dir := filepath.Dir(path)
	dirKey, pathKey := watchKey(dir), watchKey(path)
	watcher.mu.Lock()
	if watcher.paths[pathKey] <= 1 {
		delete(watcher.paths, pathKey)
		delete(watcher.ready, pathKey)
	} else {
		watcher.paths[pathKey]--
	}
	watcher.mu.Unlock()
	if watcher.dirs[dirKey] <= 1 {
		delete(watcher.dirs, dirKey)
		_ = watcher.source.Remove(dir)
	} else {
		watcher.dirs[dirKey]--
	}
}

func (watcher *externalWatcher) close() {
	_ = watcher.source.Close()
	<-watcher.done
}

func (state *model) releaseExternalWatch(doc *document.Document) {
	if binding, ok := state.externalBindings[doc]; ok {
		delete(state.externalBindings, doc)
		if watcher := state.externalWatcher; watcher != nil {
			watcher.remove(binding.path)
			if len(watcher.dirs) == 0 {
				watcher.close()
				state.externalWatcher = nil
			}
		}
	}
	delete(state.externalChanges, doc)
}

func (state *model) syncExternalWatches() {
	for _, doc := range state.space.Documents {
		binding, watching := state.externalBindings[doc]
		if watching && (binding.path != doc.Path || doc.LargeFile() != nil || state.monitors[doc] != nil) {
			state.releaseExternalWatch(doc)
			watching = false
		}
		if doc.Path == "" || doc.DiskDigest() == "" || doc.LargeFile() != nil || state.monitors[doc] != nil {
			continue
		}
		if watching {
			if binding.baseline != doc.DiskDigest() {
				binding.baseline, binding.ignored = doc.DiskDigest(), ""
				state.externalBindings[doc] = binding
				delete(state.externalChanges, doc)
			}
			continue
		}
		if state.externalWatchFailed {
			continue
		}
		if state.externalWatcher == nil {
			watcher, err := newExternalWatcher(state.invalidator.Invalidate)
			if err != nil {
				state.notice, state.externalWatchFailed = "File watching unavailable: "+err.Error(), true
				return
			}
			state.externalWatcher = watcher
		}
		if err := state.externalWatcher.add(doc.Path); err != nil {
			state.notice, state.externalWatchFailed = "File watching unavailable: "+err.Error(), true
			if len(state.externalWatcher.dirs) == 0 {
				state.externalWatcher.close()
				state.externalWatcher = nil
			}
			return
		}
		state.externalBindings[doc] = externalBinding{path: doc.Path, baseline: doc.DiskDigest()}
	}
}

func (state *model) consumeExternalChanges() {
	watcher := state.externalWatcher
	if watcher == nil {
		return
	}
	watcher.mu.Lock()
	changes, watchErr := watcher.ready, watcher.err
	watcher.ready, watcher.err = make(map[string]bool), nil
	watcher.mu.Unlock()
	if watchErr != nil {
		state.notice = "File watch: " + watchErr.Error()
	}
	for doc, binding := range state.externalBindings {
		if doc.Path != binding.path || !changes[watchKey(binding.path)] {
			continue
		}
		revision, err := document.ReadDiskRevision(binding.path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			revision = "missing"
		case err != nil:
			state.notice = "File watch: " + err.Error()
			continue
		}
		if revision != binding.ignored && binding.ignored != "" {
			binding.ignored = ""
			state.externalBindings[doc] = binding
		}
		if revision == doc.DiskDigest() || revision == binding.ignored {
			delete(state.externalChanges, doc)
		} else {
			state.externalChanges[doc] = revision
		}
	}
}

func (state *model) nextExternalChange() *document.Document {
	if current := state.space.Current(); state.externalChanges[current] != "" {
		return current
	}
	if len(state.externalChanges) != 0 {
		for _, doc := range state.space.Documents {
			if state.externalChanges[doc] != "" {
				return doc
			}
		}
	}
	return nil
}

func (state *model) paintExternalBar(canvas *gfx.Canvas, bounds gfx.Rect, doc *document.Document) {
	revision := state.externalChanges[doc]
	palette := state.palette()
	canvas.Border(bounds, 1, palette.border, palette.selectedRow)
	message := doc.Name + " changed on disk"
	missing := revision == "missing"
	if missing {
		message = doc.Name + " is missing on disk"
	}
	if doc.Dirty() {
		message += " (unsaved edits)"
	}
	buttons := [...]struct {
		id       gfx.ID
		label    string
		width    float32
		disabled bool
	}{
		{externalReloadID, "Reload", 58, missing},
		{externalCompareID, compareSavedCommand, 119, missing || !canCompareDocument(doc)},
		{externalIgnoreID, "Ignore", 54, false},
	}
	x := bounds.X + bounds.Width - 5
	for index := len(buttons) - 1; index >= 0; index-- {
		button := buttons[index]
		x -= button.width
		if palette.m5Button(canvas, button.id, gfx.Rect{X: x, Y: bounds.Y + 3, Width: button.width, Height: bounds.Height - 6}, button.label, button.disabled) {
			if button.id != externalIgnoreID && state.space.Current() != doc {
				state.space.Select(state.documentIndex(doc))
				state.activateOpenedDocument(doc)
			}
			switch button.id {
			case externalReloadID:
				state.requestReload()
			case externalCompareID:
				state.command(compareSavedCommand)
			case externalIgnoreID:
				binding := state.externalBindings[doc]
				binding.ignored = revision
				state.externalBindings[doc] = binding
				delete(state.externalChanges, doc)
				state.requestEditorCommandFocus()
			}
			state.invalidator.Invalidate()
		}
		x -= 5
	}
	label := gfx.Rect{X: bounds.X + 8, Y: bounds.Y, Width: max(0, x-bounds.X-8), Height: bounds.Height}
	smallLabel(canvas, canvas.FitLine(message, 11, gfx.WeightRegular, label.Width), label, palette.text, 11)
}
