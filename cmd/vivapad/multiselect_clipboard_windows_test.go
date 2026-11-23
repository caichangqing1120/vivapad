package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/tools/windows"
)

func TestMultiSelectionWritesWindowsClipboard(t *testing.T) {
	if os.Getenv("VIVAPAD_WINDOWS_CLIPBOARD_TEST") != "1" {
		t.Skip("requires an isolated Windows desktop clipboard")
	}
	path := filepath.Join(t.TempDir(), "selection.txt")
	if err := os.WriteFile(path, []byte(windows.SelectionClipboardSource), 0600); err != nil {
		t.Fatal(err)
	}
	state := openMultiSelectionClipboardFixture(t, path)
	doc := state.space.Current()
	var canvas gfx.Canvas
	var checkErr error
	err := app.RunApplication(func(application *app.Application) error {
		window, err := application.OpenWindow(newNativeConfig(state), &canvas)
		if err != nil {
			return err
		}
		application.Post(func() {
			defer window.Close()
			for _, command := range []string{"Copy", "Cut"} {
				sequence := windows.ClipboardSequence()
				state.command(command)
				if sequence == windows.ClipboardSequence() {
					checkErr = fmt.Errorf("%s did not write to the system clipboard (notice: %q)", command, state.notice)
					return
				}
				if err := windows.CheckSelectionClipboard(); err != nil {
					checkErr = fmt.Errorf("%s: %w", command, err)
					return
				}
			}
		})
		return nil
	})
	if err != nil || checkErr != nil {
		t.Fatalf("native application: %v; clipboard: %v", err, checkErr)
	}
	if doc.Editor.String() != "b\n" {
		t.Fatalf("Cut left %q", doc.Editor.String())
	}
}

func TestServeWindowsMultiSelectionClipboardFixture(t *testing.T) {
	path := os.Getenv("VIVAPAD_MULTI_SELECTION_FIXTURE")
	if path == "" {
		t.Skip("launched only by the isolated Windows clipboard comparison")
	}
	state := openMultiSelectionClipboardFixture(t, path)
	state.requestEditorCommandFocus()
	config := newNativeConfig(state)
	config.Title = "VivaPad clipboard fixture"
	resolve := config.TextInput
	var ready bool
	var readyErr error
	config.TextInput = func(context app.TextInputContext) *app.TextInput {
		input := resolve(context)
		if !ready && context.ID == editorID && input != nil && input.CopyOverride && input.CopyRectangular {
			ready = true
			readyErr = os.WriteFile(path+".ready", []byte("ready"), 0600)
		}
		return input
	}
	var canvas gfx.Canvas
	if err := app.RunApplication(func(application *app.Application) error {
		window, err := application.OpenWindow(config, &canvas)
		if err == nil {
			state.closeWindow = func() { config.OnCloseRequest(window.Close) }
		}
		return err
	}); err != nil || readyErr != nil || !ready {
		t.Fatalf("native fixture: %v; ready=%v, signal error=%v", err, ready, readyErr)
	}
}

func openMultiSelectionClipboardFixture(t *testing.T, path string) *model {
	t.Helper()
	state := newModel()
	if err := state.space.Open(path); err != nil {
		t.Fatal(err)
	}
	doc := state.space.Current()
	if doc.Editor.String() != "a\nb\nc\nd" {
		t.Fatalf("unexpected clipboard fixture: %q", doc.Editor.String())
	}
	ranges := []text.Range{{Start: 4, End: 7}, {Start: 2, End: 2}, {Start: 0, End: 2}}
	doc.Editor.SetSelection(ranges[2])
	state.multiSelections[0] = multiSelectionState{doc: doc, value: doc.Editor.String(), active: true, ranges: ranges}
	return state
}
