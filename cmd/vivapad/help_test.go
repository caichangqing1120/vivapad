package main

import (
	"runtime/debug"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestDebugInfoUsesOnlyBuildAndPlatformFields(t *testing.T) {
	const revision = "1234567890abcdef1234567890abcdef12345678"
	info := &debug.BuildInfo{
		GoVersion: "go1.26.11",
		Path:      "/private/home/app",
		Main:      debug.Module{Path: "/private/module", Version: "v1.2.3"},
		Deps: []*debug.Module{{Path: "github.com/vivaui/viva", Version: "v0.6.5", Replace: &debug.Module{
			Path: "/private/viva", Version: "v0.6.5-0.20260101000000-123456789abc",
		}}},
		Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: revision}, {Key: "vcs.modified", Value: "true"},
			{Key: "-ldflags", Value: "should-stay-private"}, {Key: "vcs.time", Value: "2030-01-01"}},
	}
	want := "VivaPad\nBuild: v1.2.3\nRevision: " + revision + " (modified)\nPlatform: linux/arm64\nGo: go1.26.11\nViva: v0.6.5-0.20260101000000-123456789abc"
	if got := helpBuildDetails(info, "linux", "arm64").report(); got != want {
		t.Fatalf("debug report = %q, want %q", got, want)
	}
	info.Main.Version, info.GoVersion = "(devel)", "/private/toolchain"
	info.Settings[0].Value = "/private/not-a-revision"
	info.Deps[0].Replace.Version = ""
	got := helpBuildDetails(info, "darwin", "arm64").report()
	if strings.Contains(got, "private") || !strings.Contains(got, "Build: Development build") || !strings.Contains(got, "Viva: Local build") {
		t.Fatalf("unavailable metadata report = %q", got)
	}
}

func TestHelpDialogsCopyExplicitlyAndRestoreFocus(t *testing.T) {
	for _, layout := range []struct {
		label         string
		width, height int
		overflow      bool
	}{{"desktop", 1280, 800, false}, {"compact", 640, 400, true}} {
		t.Run(layout.label, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			doc.Editor.Insert("do not include this document")
			before, selection := doc.Editor.String(), doc.Editor.Selection()
			driver.SetClipboard("prior clipboard")
			if err := driver.Resize(layout.width, layout.height); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, editorID)
			openHelp := func(index int) {
				if layout.overflow {
					clickID(t, driver, menuOverflowID)
					clickID(t, driver, hiddenMenuScope.At(12))
				} else {
					clickID(t, driver, menuScope.At(12))
				}
				clickID(t, driver, menuItemScope.At(index))
			}
			openHelp(0)
			if driver.Clipboard() != "prior clipboard" {
				t.Fatal("opening About changed the clipboard")
			}
			clickID(t, driver, helpCloseID)
			requireEditorCommandFocus(t, driver, editorID)
			openHelp(1)
			if driver.Clipboard() != "prior clipboard" {
				t.Fatal("opening Debug Info changed the clipboard")
			}
			clickID(t, driver, helpCopyID)
			got := driver.Clipboard()
			if !strings.HasPrefix(got, "VivaPad\nBuild:") || strings.Contains(got, before) || !strings.Contains(got, "\nPlatform:") {
				t.Fatalf("clipboard diagnostic = %q", got)
			}
			if err := driver.Key(gfx.KeyEscape, 0); err != nil {
				t.Fatal(err)
			}
			requireEditorCommandFocus(t, driver, editorID)
			if state.space.Current() != doc || !doc.Dirty() || doc.Editor.String() != before || doc.Editor.Selection() != selection {
				t.Fatal("help dialog changed the document")
			}
		})
	}
}
