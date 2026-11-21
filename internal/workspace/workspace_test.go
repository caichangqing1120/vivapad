package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"vivapad/internal/document"
)

func TestReloadKeepsItsOriginalBindingWhenDiskPathIsRedirected(t *testing.T) {
	root := t.TempDir()
	path, target := filepath.Join(root, "original.txt"), filepath.Join(root, "other.txt")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("other"), 0600); err != nil {
		t.Fatal(err)
	}
	space := New()
	if err := space.Open(path); err != nil {
		t.Fatal(err)
	}
	doc, bound := space.Current(), space.Current().Path
	doc.Editor.Insert("draft ")
	value := doc.Editor.String()
	if err := os.Rename(path, filepath.Join(root, "parked.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("symbolic links unavailable: %v", err)
		}
		t.Fatal(err)
	}
	if _, err := space.ReloadDiscard(space.Active); err == nil {
		t.Fatal("reload accepted a redirected document path")
	}
	if space.Current() != doc || doc.Path != bound || doc.Editor.String() != value || !doc.Dirty() {
		t.Fatal("refused reload replaced the document or lost its draft")
	}
}

func TestNewSwitchAndCloseTabs(t *testing.T) {
	space := New()
	if len(space.Documents) != 1 || space.Current().Name != "new 1" {
		t.Fatalf("initial workspace = %+v", space)
	}
	space.NewDocument()
	if len(space.Documents) != 2 || space.Active != 1 || space.Current().Name != "new 2" {
		t.Fatalf("new tab = %+v", space)
	}
	if !space.Select(0) || space.Current().Name != "new 1" || space.Select(9) {
		t.Fatal("selection accepted invalid index or missed first tab")
	}
	if err := space.Close(0, false); err != nil || len(space.Documents) != 1 || space.Current().Name != "new 2" {
		t.Fatalf("close first = %+v, %v", space, err)
	}
	if err := space.Close(0, false); err != nil || len(space.Documents) != 1 || space.Current().Name != "new 3" {
		t.Fatalf("closing last tab did not replace with blank = %+v, %v", space, err)
	}
}

func TestOpenSamePathSelectsExistingTab(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	space := New()
	if err := space.Open(path); err != nil {
		t.Fatal(err)
	}
	space.NewDocument()
	count := len(space.Documents)
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := space.Open(path); err != nil || len(space.Documents) != count || space.Current().Path != canonical {
		t.Fatalf("duplicate = %+v, %v", space, err)
	}
}

func TestCloseDirtyTabRequiresExplicitDiscard(t *testing.T) {
	space := New()
	space.Current().Editor.Insert("keep")
	if err := space.Close(0, false); !errors.Is(err, ErrDirty) {
		t.Fatalf("close dirty = %v", err)
	}
	if len(space.Documents) != 1 || space.Current().Editor.String() != "keep" {
		t.Fatal("a declined close lost content")
	}
	if err := space.Close(0, true); err != nil || space.Current().Editor.String() != "" {
		t.Fatalf("discard close = %v, %+v", err, space.Current())
	}
}

func TestAddRecoveredReplacesBlankAndKeepsUntitledNumbersUnique(t *testing.T) {
	space := New()
	recovered := document.New("new 7")
	space.AddRecovered(recovered)
	if len(space.Documents) != 1 || space.Current() != recovered {
		t.Fatalf("recovered tab = %+v", space)
	}
	if got := space.NewDocument().Name; got != "new 8" {
		t.Fatalf("new tab after recovery = %q", got)
	}
}

func TestReloadCleanTabAndRefuseDirtyTab(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reload.txt")
	if err := os.WriteFile(path, []byte("before"), 0644); err != nil {
		t.Fatal(err)
	}
	space := New()
	if err := space.Open(path); err != nil {
		t.Fatal(err)
	}
	old := space.Current()
	if err := os.WriteFile(path, []byte("after"), 0644); err != nil {
		t.Fatal(err)
	}
	replaced, err := space.Reload(0)
	if err != nil || replaced != old || space.Current().Editor.String() != "after" {
		t.Fatalf("reload = old:%p current:%+v err:%v", replaced, space.Current(), err)
	}
	space.Current().Editor.Insert("local")
	if _, err := space.Reload(0); !errors.Is(err, ErrDirty) || space.Current().Editor.String() != "localafter" {
		t.Fatalf("dirty reload = %+v, %v", space.Current(), err)
	}
}

func TestExplicitEncodingDoesNotReinterpretAnOpenTab(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.txt")
	if err := os.WriteFile(path, []byte{0xd6, 0xd0}, 0600); err != nil {
		t.Fatal(err)
	}
	space := New()
	if err := space.OpenWithEncoding(path, document.GB18030); err != nil {
		t.Fatal(err)
	}
	if len(space.Documents) != 1 || space.Current().Editor.String() != "中" {
		t.Fatalf("opened legacy tab = %+v", space.Current())
	}
	if err := space.OpenWithEncoding(path, document.Windows1252); err == nil || len(space.Documents) != 1 || space.Current().Encoding() != document.GB18030 {
		t.Fatalf("conflicting interpretation changed tab: %+v, %v", space.Current(), err)
	}
}

func TestReloadDiscardUsesSavedEncodingAndKeepsTabOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.txt")
	if err := os.WriteFile(path, []byte{0xd6, 0xd0}, 0600); err != nil {
		t.Fatal(err)
	}
	space := New()
	if err := space.OpenWithEncoding(path, document.GB18030); err != nil {
		t.Fatal(err)
	}
	old := space.Current()
	old.SetEncoding(document.UTF8)
	old.Editor.Insert(" local")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := space.ReloadDiscard(0); err == nil || space.Current() != old || !old.Dirty() {
		t.Fatalf("failed reload lost dirty tab: doc=%+v err=%v", space.Current(), err)
	}
	if err := os.WriteFile(path, []byte{0xce, 0xc4}, 0600); err != nil {
		t.Fatal(err)
	}
	replaced, err := space.ReloadDiscard(0)
	if err != nil || replaced != old || space.Current().Editor.String() != "文" || space.Current().Encoding() != document.GB18030 || space.Current().Dirty() {
		t.Fatalf("discard reload = replaced:%p current:%+v err:%v", replaced, space.Current(), err)
	}
}

func TestAddOpenedAdoptsPrevalidatedDocumentWithoutDuplicate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opened.txt")
	if err := os.WriteFile(path, []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
	prepared, err := document.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	space := New()
	if err := space.AddOpened(prepared); err != nil || len(space.Documents) != 1 || space.Current() != prepared {
		t.Fatalf("adopted tab = %+v, %v", space, err)
	}
	if err := space.AddOpened(prepared); err != nil || len(space.Documents) != 1 || space.Current() != prepared {
		t.Fatalf("duplicate preopened tab = %+v, %v", space, err)
	}
	if err := space.AddOpened(document.New("new 1")); err == nil {
		t.Fatal("untitled document was adopted as a disk file")
	}
}
