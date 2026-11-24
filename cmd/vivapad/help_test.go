package main

import (
	"errors"
	"runtime/debug"
	"slices"
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

func TestHelpProjectPageOpensOnlyTheOfficialRepository(t *testing.T) {
	const target = "https://github.com/caichangqing1120/vivapad"
	for _, layout := range []struct {
		name          string
		width, height int
		overflow      bool
	}{{"desktop", 1280, 800, false}, {"compact", 640, 400, true}} {
		t.Run(layout.name, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			doc.Editor.Insert("private draft")
			contents, selection, canUndo := doc.Editor.String(), doc.Editor.Selection(), doc.Editor.CanUndo()
			driver.SetClipboard("private clipboard")
			if err := driver.Resize(layout.width, layout.height); err != nil {
				t.Fatal(err)
			}
			index := slices.Index(menuCommands[12], "Project Page")
			if index < 0 {
				t.Fatal("Help menu does not offer Project Page")
			}
			if layout.overflow {
				clickID(t, driver, menuOverflowID)
				clickID(t, driver, hiddenMenuScope.At(12))
			} else {
				clickID(t, driver, menuScope.At(12))
			}
			if _, opened := driver.LastOpenedURL(); opened {
				t.Fatal("opening the menu launched a URL")
			}
			clickID(t, driver, menuItemScope.At(index))
			if got, opened := driver.LastOpenedURL(); !opened || got != target {
				t.Fatalf("opened URL = %q, %t; want %q", got, opened, target)
			}
			if state.space.Current() != doc || doc.Editor.String() != contents || doc.Editor.Selection() != selection || doc.Editor.CanUndo() != canUndo || !doc.Dirty() {
				t.Fatal("opening Project Page changed the editor")
			}
			if driver.Clipboard() != "private clipboard" {
				t.Fatal("opening Project Page changed the clipboard")
			}
		})
	}
}

func TestHelpLinksReportBrowserFailure(t *testing.T) {
	for _, test := range []struct{ command, notice string }{
		{"Project Page", "Could not open the project page"},
		{"User Guide", "Could not open the user guide"},
		{"Command Line Arguments...", "Could not open command-line help"},
	} {
		t.Run(test.command, func(t *testing.T) {
			state, _ := startDriver(t)
			state.openHelpLink(test.command, func(string) error { return errors.New("no default browser") })
			if state.notice != test.notice {
				t.Fatalf("failure notice = %q, want %q", state.notice, test.notice)
			}
		})
	}
}

func TestHelpOpensPublishedInstructions(t *testing.T) {
	for _, layout := range []struct {
		name          string
		width, height int
	}{{"desktop", 1280, 800}, {"compact", 640, 400}} {
		t.Run(layout.name, func(t *testing.T) {
			state, driver := startDriver(t)
			doc := state.space.Current()
			doc.Editor.Insert("unsaved draft")
			before := doc.Editor.Selection()
			driver.SetClipboard("private clipboard")
			if err := driver.Resize(layout.width, layout.height); err != nil {
				t.Fatal(err)
			}
			previousURL, previouslyOpened := "", false
			for _, item := range []struct{ label, target string }{
				{"User Guide", "https://github.com/caichangqing1120/vivapad/blob/main/README.md#current-behavior"},
				{"Command Line Arguments...", "https://github.com/caichangqing1120/vivapad/blob/main/README.md#command-line-arguments"},
			} {
				index := slices.Index(menuCommands[12], item.label)
				if index < 0 {
					t.Fatalf("Help menu does not offer %s", item.label)
				}
				if layout.width == 640 {
					clickID(t, driver, menuOverflowID)
					clickID(t, driver, hiddenMenuScope.At(12))
				} else {
					clickID(t, driver, menuScope.At(12))
				}
				if got, opened := driver.LastOpenedURL(); got != previousURL || opened != previouslyOpened {
					t.Fatal("opening Help launched a browser")
				}
				clickID(t, driver, menuItemScope.At(index))
				if got, opened := driver.LastOpenedURL(); !opened || got != item.target {
					t.Fatalf("opened URL = %q, %t; want %q", got, opened, item.target)
				}
				previousURL, previouslyOpened = item.target, true
			}
			if state.space.Current() != doc || doc.Editor.String() != "unsaved draft" || doc.Editor.Selection() != before || !doc.Editor.CanUndo() || !doc.Dirty() {
				t.Fatal("opening Help links changed the editor")
			}
			if driver.Clipboard() != "private clipboard" {
				t.Fatal("opening Help links changed the clipboard")
			}
		})
	}
}
