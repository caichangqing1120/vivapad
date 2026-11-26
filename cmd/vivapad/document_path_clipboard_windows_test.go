package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/document"
	"vivapad/tools/windows"
)

func TestDocumentPathsWriteWindowsClipboard(t *testing.T) {
	if os.Getenv("VIVAPAD_WINDOWS_CLIPBOARD_TEST") != "1" {
		t.Skip("requires an isolated Windows desktop clipboard")
	}
	mainPath, subPath := windows.DocumentPathFixture(t)
	// Open the sub-view file first so raw workspace order differs from view order.
	state := newModelWithFiles([]string{subPath, mainPath})
	if len(state.space.Documents) != 2 {
		t.Fatalf("could not open both fixture documents: %s", state.notice)
	}
	sub, main := state.space.Documents[0], state.space.Documents[1]
	state.splitEnabled = true
	state.paneDocs = [2]*document.Document{main, main}
	state.paneOwner[main], state.paneOwner[sub] = 0, 1
	state.activePane = 0
	state.space.Select(1)
	wants := windows.DocumentPathClipboardValues(mainPath, subPath)
	var canvas gfx.Canvas
	var checked bool
	var checkErr error
	err := app.RunApplication(func(application *app.Application) error {
		window, err := application.OpenWindow(newNativeConfig(state), &canvas)
		if err != nil {
			return err
		}
		application.Post(func() {
			defer window.Close()
			selection := text.Range{Start: 1, End: 5}
			main.Editor.SetSelection(selection)
			mainText, subText := main.Editor.String(), sub.Editor.String()
			for i, command := range copyPathsCommands {
				sequence := windows.ClipboardSequence()
				state.command(command)
				if err := windows.CheckDocumentPathClipboard(sequence, wants[i]); err != nil {
					checkErr = fmt.Errorf("%s: %w (status: %s)", command, err, state.notice)
					return
				}
				if state.space.Current() != main || state.activePane != 0 || main.Editor.Selection() != selection || main.Editor.String() != mainText || sub.Editor.String() != subText || main.Dirty() || sub.Dirty() || main.Editor.CanUndo() || sub.Editor.CanUndo() {
					checkErr = fmt.Errorf("%s changed editor or document state", command)
					return
				}
			}
			checked = true
		})
		return nil
	})
	if err != nil || checkErr != nil || !checked {
		t.Fatalf("native application: %v; clipboard: %v; completed: %v", err, checkErr, checked)
	}
	for path, want := range map[string]string{mainPath: windows.DocumentPathMainText, subPath: windows.DocumentPathSubText} {
		if got, err := os.ReadFile(path); err != nil || string(got) != want {
			t.Fatalf("source %q after copying = %q, read error %v", path, got, err)
		}
	}
}
