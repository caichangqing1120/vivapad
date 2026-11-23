package windows_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// IDs from the pinned Notepad++ release's menuCmdID.h.
const (
	nppSave                 = 41006
	nppUndo                 = 42003
	nppStartMacro           = 42018
	nppStopMacro            = 42019
	nppPlayMacro            = 42021
	nppAllWholeWord         = 42092
	nppNextCaseAndWholeWord = 42097
	nppUndoLatestSelection  = 42098
	nppSkipSelection        = 42099
	nppNextTab              = 44095
)

func TestNotepadPlusPlusMultiSelectionMacros(t *testing.T) {
	reference := os.Getenv("VIVAPAD_RTL_REFERENCE_EXE")
	if reference == "" {
		t.Skip("requires pinned portable Notepad++ on an isolated Windows desktop")
	}
	type selectionStep struct {
		command uintptr
		ranges  [][2]uintptr
	}
	for _, tc := range []struct {
		name, source, recorded, target, replayed string
		steps                                    []selectionStep
		key                                      uintptr
	}{
		{
			name: "AllAndType", source: "cat CAT cats cat", recorded: "x x cats x",
			// Notepad++ records a typed character as SCI_REPLACESEL. Playback
			// clears the selected ranges and inserts once at the main caret.
			// VivaPad's Driver instead expects "x x dogs x".
			target: "dog DOG dogs dog", replayed: "  dogs x",
			steps: []selectionStep{{nppAllWholeWord, [][2]uintptr{{0, 3}, {4, 7}, {13, 16}}}}, key: 'X',
		},
		{
			name: "NextSkipUndoAndBackspace", source: "cat cat cat cat!", recorded: " cat cat cat!",
			target: "dog dog dog dog!", replayed: " dog dog dog!",
			steps: []selectionStep{
				{nppNextCaseAndWholeWord, [][2]uintptr{{0, 3}}},
				{nppNextCaseAndWholeWord, [][2]uintptr{{0, 3}, {4, 7}}},
				{nppSkipSelection, [][2]uintptr{{0, 3}, {8, 11}}},
				{nppUndoLatestSelection, [][2]uintptr{{0, 3}}},
			},
			key: 0x08, // VK_BACK
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			work := t.TempDir()
			source, target := filepath.Join(work, "record.txt"), filepath.Join(work, "replay.txt")
			if err := os.WriteFile(source, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte(tc.target), 0600); err != nil {
				t.Fatal(err)
			}
			editor := openDesktopEditor(t, reference, "Notepad++", nil, "-multiInst", "-nosession", "-noPlugin", source, target)
			// The window appears before Notepad++ opens the two files and
			// activates the last one supplied on the command line.
			for deadline := time.Now().Add(8 * time.Second); !notepadShowsFile(editor, target); time.Sleep(40 * time.Millisecond) {
				if time.Now().After(deadline) {
					t.Fatal("Notepad++ did not finish opening the macro fixtures")
				}
			}
			send := selectMacroFixture(t, editor, source, tc.source)
			invokeNotepadCommand(t, editor, nppStartMacro)
			for _, step := range tc.steps {
				invokeNotepadCommand(t, editor, step.command)
				var ranges [][2]uintptr
				for n := range send(2570, 0, 0) { // SCI_GETSELECTIONS
					ranges = append(ranges, [2]uintptr{send(2585, n, 0), send(2587, n, 0)})
				}
				if !slices.Equal(ranges, step.ranges) {
					t.Fatalf("menu command %d selected %v, want %v", step.command, ranges, step.ranges)
				}
			}
			editor.keys(t, tc.key)
			for deadline := time.Now().Add(4 * time.Second); send(2159, 0, 0) == 0; time.Sleep(40 * time.Millisecond) { // SCI_GETMODIFY
				if time.Now().After(deadline) {
					t.Fatal("recorded native key did not edit the source")
				}
			}
			invokeNotepadCommand(t, editor, nppStopMacro)
			assertNotepadSaved(t, editor, source, tc.recorded)

			send = selectMacroFixture(t, editor, target, tc.target)
			if send(2174, 0, 0) != 0 { // SCI_CANUNDO
				t.Fatal("the replay target already has undo history")
			}
			invokeNotepadCommand(t, editor, nppPlayMacro)
			assertNotepadSaved(t, editor, target, tc.replayed)
			invokeNotepadCommand(t, editor, nppUndo)
			assertNotepadSaved(t, editor, target, tc.target)
			if send(2174, 0, 0) != 0 { // SCI_CANUNDO
				t.Fatal("macro playback left more than one undo unit")
			}
			editor.close(t)
		})
	}
}

func selectMacroFixture(t *testing.T, editor *desktopEditor, path, text string) func(uintptr, uintptr, uintptr) uintptr {
	t.Helper()
	if !notepadShowsFile(editor, path) {
		invokeNotepadCommand(t, editor, nppNextTab)
	}
	if !notepadShowsFile(editor, path) {
		t.Fatalf("Notepad++ did not switch to %s", filepath.Base(path))
	}
	editor.focusText(t)
	send := focusedScintilla(t, editor)
	if got := send(2006, 0, 0); got != uintptr(len(text)) { // SCI_GETLENGTH
		t.Fatalf("document length = %d, want %d", got, len(text))
	}
	send(2556, 1, 0) // SCI_SETEMPTYSELECTION: inside the initial word.
	return send
}

func notepadShowsFile(editor *desktopEditor, path string) bool {
	var title [2048]uint16
	getWindowText.Call(editor.hwnd, uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
	return strings.Contains(syscall.UTF16ToString(title[:]), filepath.Base(path)+" - Notepad++")
}

func invokeNotepadCommand(t *testing.T, editor *desktopEditor, command uintptr) {
	t.Helper()
	var result uintptr
	if ok, _, err := sendMessageTimeout.Call(editor.hwnd, 0x0111, command, 0, 2, 3000, uintptr(unsafe.Pointer(&result))); ok == 0 { // WM_COMMAND
		t.Fatalf("Notepad++ menu command %d: %v", command, err)
	}
}

func assertNotepadSaved(t *testing.T, editor *desktopEditor, path, want string) {
	t.Helper()
	invokeNotepadCommand(t, editor, nppSave)
	if got, err := os.ReadFile(path); err != nil || string(got) != want {
		t.Fatalf("saved %s = %q, want %q (read: %v)", filepath.Base(path), got, want, err)
	}
}
