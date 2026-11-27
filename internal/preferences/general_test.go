package preferences

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGeneralSettingsMigrationAndRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	legacy := `{"version":2,"font_size":17,"sub_font_size":15,"tab_width":8,"indent_mode":"tabs","wrap":true,"theme":"modern-dark"}`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	state, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.Language != "system" || state.UITextSize != "medium" || state.ShowTabNumbers || state.AutoCheckUpdates || state.FontSize != 17 || state.SubFontSize != 15 || !state.Wrap || state.Theme != ModernDark {
		t.Fatalf("unexpected migration: %+v", state)
	}
	state.Language, state.UITextSize, state.ShowTabNumbers, state.AutoCheckUpdates = "zh-CN", "large", true, true
	if err := Save(path, state); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || got.Language != "zh-CN" || got.UITextSize != "large" || !got.ShowTabNumbers || !got.AutoCheckUpdates || got.FontSize != 17 {
		t.Fatalf("round trip = %+v, %v", got, err)
	}
	state.Language = "unknown"
	if err := Save(path, state); err == nil {
		t.Fatal("saved an invalid language")
	}
}
