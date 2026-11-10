package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"vivapad/internal/document"
)

func clickMenuLabel(t *testing.T, driver *app.Driver, label string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == label && hit.Role == ui.RoleMenuItem {
			clickID(t, driver, hit.ID)
			return
		}
	}
	if fileMenuControls(driver) {
		for index, command := range menuCommands[0] {
			if command == label {
				clickID(t, driver, menuItemScope.At(index))
				return
			}
		}
	}
	t.Fatalf("menu item %q missing", label)
}

func clickCharacterSet(t *testing.T, driver *app.Driver, label string) {
	t.Helper()
	for range len(document.LegacyEncodings()) {
		for _, hit := range driver.Snapshot().Hits {
			if hit.Label == label && hit.Role == ui.RoleMenuItem {
				clickID(t, driver, hit.ID)
				return
			}
		}
		for _, hit := range driver.Snapshot().Hits {
			if hit.Label == "Next character sets" {
				clickID(t, driver, hit.ID)
				break
			}
		}
	}
	t.Fatalf("character set %q unreachable", label)
}

func TestCharacterSetsMenuReachesEverySupportedCodePageAtCompactSize(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "text.txt")
	if err := os.WriteFile(path, []byte("Japanese text"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(4))
	clickMenuLabel(t, driver, "Character sets")
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == "GB18030" && hit.ID.Scope() == charsetScope {
			if err := driver.Scroll(ui.ScrollEvent{X: hit.Bounds.X + 20, Y: hit.Bounds.Y + 8, DeltaY: 120}); err != nil {
				t.Fatal(err)
			}
			if state.charsetOffset == 0 {
				t.Fatal("character sets did not respond to wheel input")
			}
			if err := driver.Scroll(ui.ScrollEvent{X: hit.Bounds.X + 20, Y: hit.Bounds.Y + 8, DeltaY: -120}); err != nil {
				t.Fatal(err)
			}
			break
		}
	}
	seen := make(map[string]bool)
	for range len(document.LegacyEncodings()) {
		for _, hit := range driver.Snapshot().Hits {
			if _, ok := document.EncodingFromLabel(hit.Label); ok {
				seen[hit.Label] = true
				if hit.Bounds.X < 0 || hit.Bounds.Y < menuHeight || hit.Bounds.X+hit.Bounds.Width > 640 || hit.Bounds.Y+hit.Bounds.Height > 400-statusHeight {
					t.Fatalf("character-set row clipped: %+v", hit.Bounds)
				}
			}
		}
		if len(seen) == len(document.LegacyEncodings()) {
			break
		}
		clickMenuLabel(t, driver, "Next character sets")
	}
	if len(seen) != len(document.LegacyEncodings()) {
		t.Fatalf("reachable character sets = %d, want %d", len(seen), len(document.LegacyEncodings()))
	}
	clickCharacterSet(t, driver, "Shift-JIS")
	doc := state.space.Current()
	if doc.Encoding() != document.ShiftJIS || !doc.Dirty() || doc.Editor.String() != "Japanese text" {
		t.Fatalf("conversion = encoding:%v dirty:%v text:%q", doc.Encoding(), doc.Dirty(), doc.Editor.String())
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "Japanese text" {
		t.Fatalf("conversion changed source = %q, %v", data, err)
	}
}

func TestOpenAsNewOEMCodePageRejectsInvalidBeforeOpening(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	dirty := state.space.Current()
	dirty.Editor.Insert("keep")
	bad := filepath.Join(t.TempDir(), "invalid-greek.txt")
	if err := os.WriteFile(bad, []byte{'A', 0x80}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueOpenFiles([]string{bad}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickMenuLabel(t, driver, "Open as Encoding...")
	clickCharacterSet(t, driver, "OEM 869")
	if len(state.space.Documents) != 1 || state.space.Current() != dirty || dirty.Editor.String() != "keep" || !dirty.Dirty() {
		t.Fatalf("invalid OEM data changed workspace: %+v", state.space.Documents)
	}
	valid := filepath.Join(t.TempDir(), "arabic.txt")
	if err := os.WriteFile(valid, []byte{'A', 0xa0}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueOpenFiles([]string{valid}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickMenuLabel(t, driver, "Open as Encoding...")
	clickCharacterSet(t, driver, "OEM 720")
	doc := state.space.Current()
	if doc == dirty || doc.Encoding() != document.DOS720 || doc.Editor.String() != "Aب" || doc.Dirty() || dirty.Editor.String() != "keep" {
		t.Fatalf("OEM 720 open = %q, encoding=%d, dirty=%v", doc.Editor.String(), doc.Encoding(), doc.Dirty())
	}
}

func TestCaptureOEMCharacterSetMenuWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_OEM_MENU_SHOT")
	if output == "" {
		t.Skip("set VIVAPAD_OEM_MENU_SHOT to capture OEM menu")
	}
	state := newModel()
	state.space.Current().Editor.Insert("Aب")
	state.space.Current().SetEncoding(document.DOS720)
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, session.Driver(), menuScope.At(4))
	clickMenuLabel(t, session.Driver(), "Character sets")
	found := false
	for range len(document.LegacyEncodings()) {
		for _, hit := range session.Snapshot().Hits {
			if hit.Label == "OEM 720" && hit.ID.Scope() == charsetScope {
				found = true
			}
		}
		if found {
			break
		}
		clickMenuLabel(t, session.Driver(), "Next character sets")
	}
	if !found {
		t.Fatal("OEM 720 was not reachable at compact size")
	}
	if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
		t.Fatalf("OEM menu layout: %+v", problems)
	}
	if err := session.Screenshot(output); err != nil {
		t.Fatal(err)
	}
}

func TestOpenAsEncodingCancelAndInvalidBytesPreserveDirtyTab(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	dirty := state.space.Current()
	dirty.Editor.Insert("keep")
	if err := driver.QueueOpenFiles(nil, app.ErrCanceled); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickMenuLabel(t, driver, "Open as Encoding...")
	clickCharacterSet(t, driver, "Shift-JIS")
	if len(state.space.Documents) != 1 || state.space.Current() != dirty || dirty.Editor.String() != "keep" {
		t.Fatal("canceled encoded picker changed the dirty tab")
	}
	path := filepath.Join(t.TempDir(), "invalid.txt")
	if err := os.WriteFile(path, []byte{0x82}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueOpenFiles([]string{path}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickMenuLabel(t, driver, "Open as Encoding...")
	clickCharacterSet(t, driver, "Shift-JIS")
	if len(state.space.Documents) != 1 || state.space.Current() != dirty || dirty.Editor.String() != "keep" || !strings.Contains(state.notice, "invalid") {
		t.Fatalf("invalid encoded file changed dirty tab: docs=%d notice=%q", len(state.space.Documents), state.notice)
	}
}

func TestCharacterSetConversionUsesActiveSplitPane(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("left")
	right := state.space.NewDocument()
	right.Editor.Insert("日本")
	state.toggleSplit()
	state.paneDocs = [2]*document.Document{left, right}
	state.paneOwner[left], state.paneOwner[right] = 0, 1
	state.activePane = 1
	state.space.Select(state.documentIndex(right))
	clickID(t, driver, menuScope.At(4))
	clickMenuLabel(t, driver, "Character sets")
	clickCharacterSet(t, driver, "Shift-JIS")
	if left.Encoding() != document.UTF8 || right.Encoding() != document.ShiftJIS || left.Editor.String() != "left" || right.Editor.String() != "日本" {
		t.Fatalf("split conversion changed wrong pane: left=%v right=%v", left.Encoding(), right.Encoding())
	}
	right.SetEncoding(document.UTF8)
	state.readOnly[right] = true
	clickID(t, driver, menuScope.At(4))
	clickMenuLabel(t, driver, "Character sets")
	clickCharacterSet(t, driver, "Shift-JIS")
	if right.Encoding() != document.UTF8 || state.notice != "Read-only document" {
		t.Fatalf("read-only conversion = %v, notice=%q", right.Encoding(), state.notice)
	}
}

func TestOpenAsEncodingAddsFileToActiveSplitPane(t *testing.T) {
	state, driver := startDriver(t)
	left := state.space.Current()
	left.Editor.Insert("keep left")
	state.toggleSplit()
	state.activePane = 1
	path := filepath.Join(t.TempDir(), "japanese.txt")
	if err := os.WriteFile(path, []byte{0x93, 0xfa, 0x96, 0x7b}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueOpenFiles([]string{path}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(0))
	clickMenuLabel(t, driver, "Open as Encoding...")
	clickCharacterSet(t, driver, "Shift-JIS")
	if doc := state.space.Current(); len(state.space.Documents) != 2 || state.paneDocs[0] != left || state.paneDocs[1] != doc ||
		state.activePane != 1 || doc.Encoding() != document.ShiftJIS || doc.Editor.String() != "日本" || left.Editor.String() != "keep left" {
		t.Fatalf("encoded open in split view = docs:%d panes:%+v active:%d notice:%q", len(state.space.Documents), state.paneDocs, state.activePane, state.notice)
	}
}
