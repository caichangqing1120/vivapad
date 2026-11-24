//go:build darwin || windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

const postItNativeTitle = "VivaPad native Post-It test"

type postItNativeWindow struct {
	id                          uintptr
	decorated, topmost, focused bool
}

func TestMain(m *testing.M) {
	// Cocoa must run the window loop on the process's original main thread.
	if scenario := os.Getenv("VIVAPAD_POST_IT_CHILD"); scenario != "" {
		if err := runNativePostIt(scenario); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestNativePostIt(t *testing.T) {
	if os.Getenv("VIVAPAD_TEST_NATIVE_POST_IT") != "1" {
		t.Skip("requires a native desktop session")
	}
	for _, scenario := range []string{"windowed", "fullscreen-topmost"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
			defer cancel()
			child := exec.CommandContext(ctx, os.Args[0])
			child.Env = append(os.Environ(), "VIVAPAD_POST_IT_CHILD="+scenario, "VIVA_SOFTWARE_RENDERING=1")
			output, err := child.CombinedOutput()
			if err != nil {
				t.Fatalf("native Post-It: %v\n%s", err, output)
			}
			t.Logf("%s", output)
		})
	}
}

func runNativePostIt(scenario string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	cleanup, err := prepareNativePostItDesktop()
	if err != nil {
		return err
	}
	defer cleanup()

	state := newModel()
	left := state.space.Current()
	left.Editor.Insert("left draft")
	right := state.space.NewDocument()
	right.Editor.Insert("right draft")
	selection := text.Range{Start: 6, End: 11}
	right.Editor.SetSelection(selection)
	state.toggleSplit()
	state.requestNativeChooserFocus()
	config := newNativeConfig(state)
	config.Title, config.Width, config.Height = postItNativeTitle, 640, 400
	var canvas gfx.Canvas
	var result error
	completed := false
	loopErr := app.RunApplication(func(application *app.Application) error {
		window, err := application.OpenWindow(config, &canvas)
		if err != nil {
			return err
		}
		state.bindWindowModeControls(window)
		focusNativePostItWindow()
		fullscreen := scenario == "fullscreen-topmost"
		phase := "ready"
		deadline := time.Now().Add(25 * time.Second)
		var original postItNativeWindow
		var frame gfx.Rect
		var advance func()
		finish := func(err error) {
			completed, result = true, err
			window.Close()
		}
		advance = func() {
			if time.Now().After(deadline) {
				finish(fmt.Errorf("timed out at %s: active=%t pending=%t fullscreen=%t focus=%v notice=%q", phase,
					state.postIt.active, state.postIt.pending, state.fullscreen, canvas.Focused(), state.notice))
				return
			}
			now, available := window.Frame()
			native := inspectNativePostItWindow()
			switch phase {
			case "ready":
				if !available || native.id == 0 || !native.focused || canvas.Focused() != rightEditorID {
					break
				}
				if fullscreen {
					state.command("Always on Top")
					if err := window.SetFullscreen(true); err != nil {
						finish(err)
						return
					}
				}
				phase = "enter"
			case "enter":
				if !available || !native.focused || state.fullscreen != fullscreen || native.topmost != fullscreen ||
					fullscreen && now.Width == 640 && now.Height == 400 {
					break
				}
				original, frame = native, now
				if !fullscreen && (!native.decorated || now.Width != 640 || now.Height != 400) {
					finish(fmt.Errorf("invalid initial native window: %+v %v", native, now))
					return
				}
				sendNativePostItKey(native.id, true)
				phase = "post-it"
			case "post-it":
				if !state.postIt.active || state.postIt.pending || !postItNativeHits(&canvas, postItRestoreID, editorID, rightEditorID) {
					break
				}
				if !available || now != frame || native.id != original.id || !native.focused || !native.topmost ||
					state.fullscreen != fullscreen || !fullscreen && native.decorated {
					finish(fmt.Errorf("active Post-It changed native window: %+v %v, before %+v %v", native, now, original, frame))
					return
				}
				sendNativePostItKey(native.id, true)
				phase = "restore"
			case "restore":
				if state.postIt.active || state.postIt.pending || canvas.Focused() != rightEditorID {
					break
				}
				if !available || now != frame || native != original || state.fullscreen != fullscreen ||
					!state.splitEnabled || state.activePane != 1 || state.paneDocument(0) != left || state.paneDocument(1) != right || right.Editor.Selection() != selection {
					finish(fmt.Errorf("restore changed window or editor state: %+v %v, before %+v %v, selection=%v", native, now, original, frame, right.Editor.Selection()))
					return
				}
				sendNativePostItKey(native.id, false)
				phase = "first edit"
			case "first edit":
				if right.Editor.String() == "right draft" && left.Editor.String() == "left draft" {
					break
				}
				if right.Editor.String() != "right x" || left.Editor.String() != "left draft" || !right.Editor.Undo() ||
					right.Editor.String() != "right draft" {
					finish(fmt.Errorf("first edit or Undo reached the wrong pane: %q / %q", left.Editor.String(), right.Editor.String()))
					return
				}
				fmt.Printf("PASS %s: same native window %#x and client %v; focus, level, first key and Undo preserved\n", scenario, native.id, frame)
				finish(nil)
				return
			}
			application.PostAfter(60*time.Millisecond, advance)
		}
		application.PostAfter(100*time.Millisecond, advance)
		return nil
	})
	if loopErr != nil || !completed {
		return fmt.Errorf("native window loop: %v (completed=%t, result=%v)", loopErr, completed, result)
	}
	return result
}

func postItNativeHits(canvas *gfx.Canvas, ids ...gfx.ID) bool {
	for _, id := range ids {
		found := false
		for _, hit := range canvas.Hits() {
			if hit.ID == id && !hit.Disabled {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
