//go:build darwin || windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/preferences"
)

func TestThemeNativeDesktop(t *testing.T) {
	if os.Getenv("VIVAPAD_TEST_NATIVE_THEME") != "1" {
		t.Skip("requires a native desktop session")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0])
	child.Env = append(os.Environ(), "VIVAPAD_THEME_CHILD=1", "VIVA_SOFTWARE_RENDERING=1")
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("native themes: %v\n%s", err, output)
	}
}

func runNativeTheme() error {
	state := newModel()
	state.space.Current().Editor.Insert("package demo\n\n// Native desktop theme check\n")
	state.command("Clone to Other View")
	state.command("Mark...")
	config := newNativeConfig(state)
	config.Title, config.Width, config.Height = "VivaPad native theme test", 960, 720
	choices := [...]preferences.Theme{preferences.ClassicLight, preferences.ModernDark, preferences.Monokai, preferences.FollowSystem}
	backgrounds := [...]uint32{0xffffff, 0x1e1f22, 0x272822, 0xffffff}
	var result error
	completed := false
	if err := app.RunApplication(func(application *app.Application) error {
		step, nextPosted := 0, false
		paint := config.Paint
		config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
			paint(canvas, size)
			if nextPosted {
				return
			}
			nextPosted = true
			want := backgrounds[step]
			if choices[step] == preferences.FollowSystem && app.SystemAppearance() == app.AppearanceDark {
				want = 0x1e1f22
			}
			result = checkNativeThemeFrame(canvas, want)
			application.Post(func() {
				if result != nil || step+1 == len(choices) {
					completed = result == nil
					application.Quit()
					return
				}
				step++
				nextPosted = false
				state.themeChoice = choices[step]
				state.invalidator.Invalidate()
			})
		}
		_, err := application.OpenWindow(config, new(gfx.Canvas))
		return err
	}); err != nil {
		return err
	}
	if result != nil {
		return result
	}
	if !completed {
		return fmt.Errorf("native window closed before all themes painted")
	}
	return nil
}

func checkNativeThemeFrame(canvas *gfx.Canvas, background uint32) error {
	var panes, backgrounds, dialog, input int
	for _, hit := range canvas.Hits() {
		switch hit.ID {
		case editorID, rightEditorID:
			panes++
			for _, rectangle := range canvas.Rectangles() {
				if rectangle.Color == gfx.RGB24(background) && rectangle.Bounds.Width >= hit.Bounds.Width-6 && rectangle.Bounds.Height >= hit.Bounds.Height-6 && rectangle.Bounds.X >= hit.Bounds.X && rectangle.Bounds.Y >= hit.Bounds.Y {
					backgrounds++
					break
				}
			}
		case markPanelID:
			dialog++
		case queryID:
			input++
		}
	}
	if panes != 2 || backgrounds != 2 || dialog != 1 || input != 1 {
		return fmt.Errorf("native frame %06x: %d panes, %d themed backgrounds, %d dialogs, %d inputs", background, panes, backgrounds, dialog, input)
	}
	if background != 0xffffff {
		for _, rectangle := range canvas.Rectangles() {
			color := rectangle.Color
			if rectangle.Bounds.Width > 20 && rectangle.Bounds.Height > 12 && color.Alpha == 1 && min(color.Red, color.Green, color.Blue) > 0.9 {
				return fmt.Errorf("native dark frame has a light surface at %v: %v", rectangle.Bounds, color)
			}
		}
	}
	return nil
}
