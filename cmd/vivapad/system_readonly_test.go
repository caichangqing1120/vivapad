package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
)

func TestFileReadOnlyAttributeIsSeparateFromInternalFlagAndSharedByClones(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	state.command("Clone to Other View")
	state.command("Toggle File Read-Only Attribute")
	if !state.isReadOnly(doc) || !state.menuSelected("File Read-Only Attribute") || state.menuSelected("Read-Only on Current Document") ||
		state.paneDocument(0) != doc || state.paneDocument(1) != doc || doc.Dirty() {
		t.Fatalf("attribute toggle changed internal/split state: readonly=%v, dirty=%v, notice=%q", state.isReadOnly(doc), doc.Dirty(), state.notice)
	}
	state.command("Clear Read-Only for All Documents")
	if !state.isReadOnly(doc) {
		t.Fatal("clearing the internal flag cleared the disk attribute")
	}
	state.command("Toggle File Read-Only Attribute")
	if state.isReadOnly(doc) || state.menuSelected("File Read-Only Attribute") {
		t.Fatalf("second toggle left file read-only: %q", state.notice)
	}
}

func TestExistingFileAttributeProtectsNativeInputAndDirtySave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	doc.Editor.Insert("after ")
	if err := os.Chmod(path, 0400); err != nil {
		t.Fatal(err)
	}
	if !state.isReadOnly(doc) {
		t.Fatal("external file-attribute change did not update editor protection")
	}
	driver, err := app.NewDriver(newConfig(state), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	clickID(t, driver, editorID)
	_ = driver.Type("X")
	state.command("Save")
	if doc.Editor.String() != "after before" || !doc.Dirty() || !strings.Contains(strings.ToLower(state.notice), "permission") {
		t.Fatalf("read-only edit/save: text=%q dirty=%v notice=%q", doc.Editor.String(), doc.Dirty(), state.notice)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "before" {
		t.Fatalf("read-only file was overwritten: %q, %v", data, err)
	}
}

func TestOpeningReadOnlyFileKeepsInternalFlagSeparate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "already-readonly.txt")
	if err := os.WriteFile(path, []byte("before"), 0400); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0400); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	doc := state.space.Current()
	if !state.isReadOnly(doc) || !state.menuSelected(fileReadOnlyLabel()) || state.menuSelected("Read-Only on Current Document") {
		t.Fatalf("opening existing read-only file: readonly=%v internal=%v", state.isReadOnly(doc), state.readOnly[doc])
	}
	state.command("Read-Only on Current Document")
	if !state.readOnly[doc] {
		t.Fatal("the internal flag could not be set while the file is read-only")
	}
	state.command("Toggle File Read-Only Attribute")
	if !state.isReadOnly(doc) || state.menuSelected(fileReadOnlyLabel()) || !state.menuSelected("Read-Only on Current Document") {
		t.Fatalf("clearing disk attribute also cleared internal mode: %q", state.notice)
	}
}

func TestFileReadOnlyMenuAtCompactSizeAndUntitledFailure(t *testing.T) {
	state, driver := startDriver(t)
	if err := driver.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, menuScope.At(1))
	hit := revealEditMenuItem(t, driver, 17)
	if hit.Y < menuHeight || hit.Y+hit.Height > 400-statusHeight {
		t.Fatalf("file attribute row clipped: %+v", hit)
	}
	clickID(t, driver, menuItemScope.At(17))
	if state.isReadOnly(state.space.Current()) || !strings.Contains(state.notice, "saved file") {
		t.Fatalf("untitled toggle changed editor or failed silently: readonly=%v notice=%q", state.isReadOnly(state.space.Current()), state.notice)
	}
	if problems := headless.CheckLayout(driver.Snapshot()); len(problems) != 0 {
		t.Fatalf("compact Edit menu layout: %+v", problems)
	}
}

func TestCaptureFileReadOnlyMenuWhenRequested(t *testing.T) {
	output := os.Getenv("VIVAPAD_SYSTEM_READONLY_SHOTS")
	if output == "" {
		t.Skip("set VIVAPAD_SYSTEM_READONLY_SHOTS to capture file-attribute menu")
	}
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("read-only test"), 0600); err != nil {
		t.Fatal(err)
	}
	state := newModelWithFiles([]string{path})
	session, err := headless.Open(newConfig(state), nil, headless.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Resize(640, 400); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		name    string
		checked bool
	}{
		{"menu", false}, {"checked", true},
	} {
		clickID(t, session.Driver(), menuScope.At(1))
		revealEditMenuItem(t, session.Driver(), 17)
		if problems := headless.CheckLayout(session.Snapshot()); len(problems) != 0 {
			t.Fatalf("%s menu layout: %+v", item.name, problems)
		}
		if state.menuSelected(fileReadOnlyLabel()) != item.checked {
			t.Fatalf("%s menu check = %v", item.name, state.menuSelected(fileReadOnlyLabel()))
		}
		if err := session.Screenshot(filepath.Join(output, "file-readonly-"+item.name+"-compact.png")); err != nil {
			t.Fatal(err)
		}
		if !item.checked {
			clickID(t, session.Driver(), menuItemScope.At(17))
		}
	}
}
