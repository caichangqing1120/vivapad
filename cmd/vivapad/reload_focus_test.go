package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"vivapad/internal/document"
)

func TestCompactReloadShowsDecodeFailureBeforePath(t *testing.T) {
	root := filepath.Join(t.TempDir(), strings.Repeat("legacy-directory-", 8))
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "source.txt")
	if err := os.WriteFile(path, []byte{0xaa, 0xa1}, 0600); err != nil {
		t.Fatal(err)
	}
	state, driver := startDriver(t)
	if err := state.space.OpenWithEncoding(path, document.GB18030); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	state.showInActivePane(doc)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte{0x81}, 0600); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	reload := slices.Index(menuCommands[0], "Reload from Disk")
	revealFileMenuItem(t, driver, reload)
	clickID(t, driver, menuItemScope.At(reload))
	if state.space.Current() != doc || doc.Editor.String() != "\ue000" || doc.Dirty() || doc.Editor.CanUndo() {
		t.Fatal("failed reload changed the original GB18030 document")
	}
	if data, err := os.ReadFile(path); err != nil || !slices.Equal(data, []byte{0x81}) {
		t.Fatalf("failed reload wrote to disk: %x, %v", data, err)
	}
	const cause = "Reload failed: invalid GB18030 sequence"
	for _, label := range driver.Snapshot().Texts {
		if strings.Contains(label.Value, cause) {
			return
		}
	}
	t.Fatalf("compact status hid the decode error: %q", state.notice)
}

func TestReloadReturnsKeyboardToActiveSource(t *testing.T) {
	for _, size := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		for _, layout := range []string{"right", "clone-right"} {
			for _, outcome := range []string{"clean", "confirm", "missing"} {
				t.Run(size.name+"/"+layout+"/"+outcome, func(t *testing.T) {
					root := t.TempDir()
					path, peer := filepath.Join(root, "reload.txt"), filepath.Join(root, "peer.txt")
					for name, value := range map[string]string{path: "stored", peer: "peer"} {
						if err := os.WriteFile(name, []byte(value), 0600); err != nil {
							t.Fatal(err)
						}
					}
					state := newStartupModel([]string{path, peer}, filepath.Join(root, "session.json"))
					source, other := state.space.Documents[0], state.space.Documents[1]
					left := other
					if layout == "clone-right" {
						left = source
					}
					state.splitEnabled = true
					state.paneDocs = [2]*document.Document{left, source}
					state.activatePane(1)
					driver, err := app.NewDriver(newConfig(state), nil)
					if err != nil {
						t.Fatal(err)
					}
					defer driver.Close()
					if err := driver.Resize(size.width, size.height); err != nil {
						t.Fatal(err)
					}
					clickID(t, driver, rightEditorID)
					source.Editor.MoveTo(source.Editor.RuneCount(), false)
					if outcome != "clean" {
						if err := driver.Type("local"); err != nil {
							t.Fatal(err)
						}
						source.Editor.BreakUndoGroup()
					}
					history, err := source.ChangeHistory()
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte("disk"), 0600); err != nil {
						t.Fatal(err)
					}
					clickID(t, driver, menuScope.At(0))
					revealFileMenuItem(t, driver, 9)
					clickID(t, driver, menuItemScope.At(9))
					if outcome != "clean" {
						if state.pendingReload != state.documentIndex(source) {
							t.Fatal("dirty reload bypassed confirmation")
						}
						if outcome == "missing" {
							if err := os.Remove(path); err != nil {
								t.Fatal(err)
							}
						}
						clickID(t, driver, reloadConfirmID)
					}
					current := state.space.Current()
					gotHistory, err := current.ChangeHistory()
					if err != nil {
						t.Fatal(err)
					}
					want, wantDirty := "disk", false
					if outcome == "missing" {
						want, wantDirty = "storedlocal", true
						if current != source || gotHistory != history || !strings.Contains(state.notice, "Reload failed") {
							t.Fatal("failed reload replaced the source, history, or notice")
						}
					} else if current == source || gotHistory == history || current.Editor.CanUndo() || current.Editor.CanRedo() {
						t.Fatal("successful reload did not establish fresh history")
					}
					if layout == "clone-right" {
						left = current
					}
					if state.activePane != 1 || state.paneDocs != [2]*document.Document{left, current} || current.Editor.String() != want || current.Dirty() != wantDirty {
						t.Fatalf("unexpected reload state: text=%q dirty=%v activePane=%d", current.Editor.String(), current.Dirty(), state.activePane)
					}

					requireEditorCommandFocus(t, driver, rightEditorID)
					if err := driver.Type("x"); err != nil || current.Editor.String() != want+"x" || other.Editor.String() != "peer" {
						t.Fatalf("first input after reload: source=%q peer=%q err=%v", current.Editor.String(), other.Editor.String(), err)
					}
					if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || current.Editor.String() != want || current.Dirty() != wantDirty {
						t.Fatalf("first Undo after reload: text=%q dirty=%v handled=%v err=%v", current.Editor.String(), current.Dirty(), handled, err)
					}
					if outcome == "missing" {
						if handled, err := driver.ShortcutChord(primary(), 'z'); err != nil || !handled || current.Editor.String() != "stored" || current.Dirty() {
							t.Fatalf("prior Undo after failed reload: text=%q dirty=%v handled=%v err=%v", current.Editor.String(), current.Dirty(), handled, err)
						}
					}
					bytes, err := os.ReadFile(path)
					if outcome == "missing" && !errors.Is(err, os.ErrNotExist) || outcome != "missing" && (err != nil || string(bytes) != "disk") {
						t.Fatalf("reload or typing wrote to disk: %q, %v", bytes, err)
					}
				})
			}
		}
	}
}
