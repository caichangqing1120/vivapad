package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/document"
	"vivapad/internal/preferences"
)

func TestStatusActionsDriver(t *testing.T) {
	for _, theme := range []preferences.Theme{preferences.ClassicLight, preferences.ModernDark, preferences.Monokai} {
		t.Run(string(theme), func(t *testing.T) {
			state, driver := startDriver(t)
			state.themeChoice = theme
			if err := driver.Resize(640, 400); err != nil {
				t.Fatal(err)
			}
			doc := state.space.Current()
			doc.Editor.Insert("first\nsecond\nthird")
			clickID(t, driver, statusGoToID)
			requireEditorCommandFocus(t, driver, goToFieldID)
			if err := driver.Type("2"); err != nil {
				t.Fatal(err)
			}
			if err := driver.Key(gfx.KeyEnter, 0); err != nil {
				t.Fatal(err)
			}
			if line, column := doc.Position(); line != 2 || column != 1 {
				t.Fatalf("position = %d, %d", line, column)
			}
			clickID(t, driver, statusEOLID)
			clickStatusOption(t, driver, "Windows (CR LF)")
			if doc.LineEnding() != document.CRLF {
				t.Fatal("status did not change EOL")
			}
			clickID(t, driver, statusEncodingID)
			clickStatusOption(t, driver, "GB18030")
			if doc.Encoding() != document.GB18030 {
				t.Fatal("status did not change encoding")
			}
			clickID(t, driver, statusLanguageID)
			clickStatusOption(t, driver, "JSON")
			if got := state.languageLabel(doc); got != "JSON" {
				t.Fatalf("language = %q", got)
			}
			requireEditorCommandFocus(t, driver, editorID)
		})
	}
}

func TestStatusFormatActionsRefuseReadOnly(t *testing.T) {
	for _, large := range []bool{false, true} {
		state, driver := startDriver(t)
		if large {
			oldThreshold := document.LargeFileThreshold
			document.LargeFileThreshold = 10
			path := filepath.Join(t.TempDir(), "status.txt")
			if err := os.WriteFile(path, []byte("a large test\nsecond line\n"), 0600); err != nil {
				document.LargeFileThreshold = oldThreshold
				t.Fatal(err)
			}
			opened := state.openDocumentPaths([]string{path})
			document.LargeFileThreshold = oldThreshold
			if !opened || state.space.Current().LargeFile() == nil {
				t.Fatal("large test file did not open")
			}
		} else {
			state.command("Read-Only on Current Document")
		}
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
		doc := state.space.Current()
		encoding, ending := doc.Encoding(), doc.LineEnding()
		for _, target := range []gfx.ID{statusEOLID, statusEncodingID} {
			clickID(t, driver, target)
			if !state.statusPopup.kind.IsZero() || !strings.Contains(strings.ToLower(state.notice), "read-only") || doc.Encoding() != encoding || doc.LineEnding() != ending || doc.Dirty() {
				t.Fatalf("readonly format action mutated state: %q", state.notice)
			}
		}
		clickID(t, driver, statusGoToID)
		requireEditorCommandFocus(t, driver, goToFieldID)
	}
}

func clickStatusOption(t *testing.T, driver *app.Driver, label string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID.Scope() == statusChoiceScope && hit.Label == label {
			clickID(t, driver, hit.ID)
			return
		}
	}
	t.Fatalf("missing status choice %q", label)
}
