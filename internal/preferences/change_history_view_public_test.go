package preferences

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestChangeHistoryViewDefaultMigrationAndExplicitOff(t *testing.T) {
	if Default().ChangeHistoryView != "margin" {
		t.Fatalf("new default view = %q", Default().ChangeHistoryView)
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	old := Default()
	old.ChangeHistoryView = ""
	data, err := json.Marshal(diskState{Version: 5, State: old})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || loaded.ChangeHistoryView != "margin" {
		t.Fatalf("v5 migration = %q, %v", loaded.ChangeHistoryView, err)
	}
	loaded.ChangeHistoryView = "off"
	if err := Save(path, loaded); err != nil {
		t.Fatal(err)
	}
	if restored, err := Load(path); err != nil || restored.ChangeHistoryView != "off" {
		t.Fatalf("explicit off was lost: %q, %v", restored.ChangeHistoryView, err)
	}
	loaded.ChangeHistoryView = "unknown"
	if err := Save(path, loaded); err == nil {
		t.Fatal("invalid display mode was persisted")
	}
}
