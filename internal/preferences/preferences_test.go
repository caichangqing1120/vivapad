package preferences

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"vivapad/internal/printing"
)

func TestPrivateSettingsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "VivaPad", "settings.json")
	want := State{FontSize: 17, SubFontSize: 17, ZoomSync: true, TabWidth: 8, Wrap: true, LineNumbers: false, AutoIndent: false, AutoClose: true, IndentMode: "tabs", DateTimeFormat: "dd/MM/yyyy HH:mm", DateTimeReverseOrder: true, ShowWhitespace: true, ShowEOL: true, NPCMode: "abbreviation",
		Print: printing.Settings{LineNumbers: false, ColorMode: printing.ColorBlackOnWhite, FormFeedPageBreak: true,
			MarginMM: printing.Margins{Left: 9, Top: 12, Right: 10, Bottom: 13}, Header: printing.TextBand{Left: "$(FILE_NAME)", Right: "$(CURRENT_PRINTING_PAGE)"}, Footer: printing.TextBand{Middle: "$(SHORT_DATE)"},
			HeaderFont: printing.BandFont{Size: 12, Bold: true}, FooterFont: printing.BandFont{Size: 8, Italic: true}}}
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("loaded = %+v, %v; want %+v", got, err, want)
	}
	info, err := os.Stat(path)
	if err != nil || !privateMode(info.Mode()) {
		t.Fatalf("settings mode = %+v, %v", info, err)
	}
	info, err = os.Stat(filepath.Dir(path))
	if err != nil || !privateDirMode(info.Mode()) {
		t.Fatalf("settings directory mode = %+v, %v", info, err)
	}
	encoded, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(encoded), `"print_settings":{"line_numbers":false,"color_mode":4`) ||
		!strings.Contains(string(encoded), `"margin_mm":{"left":9,"top":12,"right":10,"bottom":13}`) {
		t.Fatalf("print settings JSON contract = %s, %v", encoded, err)
	}
}

func TestMissingAndInvalidSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if got, err := Load(path); err != nil || !reflect.DeepEqual(got, Default()) {
		t.Fatalf("missing settings = %+v, %v", got, err)
	}
	for name, content := range map[string]string{
		"invalid JSON":   "{",
		"future version": `{"version":6}`,
		"bad width":      `{"version":1,"font_size":13,"tab_width":0,"line_numbers":true,"auto_indent":true,"auto_close":true,"indent_mode":"language"}`,
		"unknown field":  `{"version":1,"font_size":13,"tab_width":4,"line_numbers":true,"auto_indent":true,"auto_close":true,"indent_mode":"language","surprise":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("invalid settings accepted")
			}
		})
	}
}

func TestIndentGuidesDefaultOnAndV3Migration(t *testing.T) {
	if !Default().ShowIndentGuides {
		t.Fatal("source default indent guides should be visible")
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	old := Default()
	old.ShowIndentGuides = false
	legacy, err := json.Marshal(diskState{Version: 3, State: old})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || !loaded.ShowIndentGuides {
		t.Fatalf("v3 indent guide migration = %+v, %v", loaded, err)
	}
	loaded.ShowIndentGuides = false
	if err := Save(path, loaded); err != nil {
		t.Fatal(err)
	}
	if restored, err := Load(path); err != nil || restored.ShowIndentGuides {
		t.Fatalf("v4 explicit disabled guide = %+v, %v", restored, err)
	}
}

func TestV4ControlCharacterDefaultMigratesWithoutResettingSymbols(t *testing.T) {
	if !Default().ShowControls || Default().ShowNonPrinting {
		t.Fatalf("source symbol defaults = %+v", Default())
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	old := Default()
	old.ShowWhitespace, old.ShowEOL = true, false
	old.ShowControls = false
	legacy, err := json.Marshal(diskState{Version: 4, State: old})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || !loaded.ShowControls || !loaded.ShowWhitespace || loaded.ShowEOL || loaded.ShowNonPrinting {
		t.Fatalf("v4 symbol migration = %+v, %v", loaded, err)
	}
	loaded.ShowControls = false
	if err := Save(path, loaded); err != nil {
		t.Fatal(err)
	}
	if restored, err := Load(path); err != nil || restored.ShowControls {
		t.Fatalf("v5 explicit hidden controls = %+v, %v", restored, err)
	}
}

func TestNPCModeDefaultLegacyLoadAndValidation(t *testing.T) {
	if Default().NPCMode != "abbreviation" || Default().NPCIncludeControls {
		t.Fatalf("source NPC defaults = %+v", Default())
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	old := Default()
	old.NPCMode = ""
	legacy, err := json.Marshal(diskState{Version: 5, State: old})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, legacy, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || loaded.NPCMode != "abbreviation" || loaded.NPCIncludeControls {
		t.Fatalf("v5 NPC mode migration = %+v, %v", loaded, err)
	}
	loaded.NPCMode, loaded.NPCIncludeControls = "codepoint", true
	if err := Save(path, loaded); err != nil {
		t.Fatal(err)
	}
	if restored, err := Load(path); err != nil || !reflect.DeepEqual(restored, loaded) {
		t.Fatalf("NPC settings roundtrip = %+v, %v", restored, err)
	}
	loaded.NPCMode = "unknown"
	if err := Save(path, loaded); err == nil {
		t.Fatal("unsupported NPC representation mode saved")
	}
}

func TestSaveRejectsInvalidSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	settings := Default()
	settings.IndentMode = "mixed"
	if err := Save(path, settings); err == nil {
		t.Fatal("saved unsupported indentation mode")
	}
}

func TestLegacySettingsRestoreDefaultDateTimeFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	legacy := `{"version":1,"font_size":13,"tab_width":4,"line_numbers":true,"auto_indent":true,"auto_close":true,"indent_mode":"language"}`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	stored, err := Load(path)
	if err != nil || stored.DateTimeFormat != "yyyy-MM-dd HH:mm:ss" || stored.DateTimeReverseOrder || stored.SubFontSize != 13 || stored.ZoomSync {
		t.Fatalf("legacy date-time settings = %+v, %v", stored, err)
	}
}

func TestV1SharedZoomMigratesToBothViews(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	legacy := `{"version":1,"font_size":18,"tab_width":4,"line_numbers":true,"auto_indent":true,"auto_close":true,"indent_mode":"language"}`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	stored, err := Load(path)
	if err != nil || stored.FontSize != 18 || stored.SubFontSize != 18 || stored.ZoomSync {
		t.Fatalf("v1 shared font size = %+v, %v", stored, err)
	}
	stored.SubFontSize = 25
	if err := Save(path, stored); err == nil {
		t.Fatal("accepted invalid sub-view font size")
	}
}

func TestInvalidDateTimeFormatCannotBeSaved(t *testing.T) {
	settings := Default()
	settings.DateTimeFormat = "yyyy\nMM"
	if err := Save(filepath.Join(t.TempDir(), "settings.json"), settings); err == nil {
		t.Fatal("settings with a control character were saved")
	}
}

func TestV2PrintSettingsMigrateToSourceDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	legacy := `{"version":2,"font_size":13,"sub_font_size":13,"tab_width":4,"line_numbers":true,"auto_indent":true,"auto_close":true,"indent_mode":"language"}`
	if err := os.WriteFile(path, []byte(legacy), 0600); err != nil {
		t.Fatal(err)
	}
	stored, err := Load(path)
	if err != nil || stored.Print != printing.DefaultSettings() {
		t.Fatalf("v2 print defaults = %+v, %v", stored.Print, err)
	}
}

func TestPrivateSettingsRejectInvalidPrintConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	for _, changed := range []func(*printing.Settings){
		func(value *printing.Settings) { value.ColorMode = 99 },
		func(value *printing.Settings) { value.MarginMM.Left = -1 },
		func(value *printing.Settings) { value.Header.Left = string(make([]byte, 256)) },
		func(value *printing.Settings) { value.Header.Left = "first\nsecond" },
		func(value *printing.Settings) { value.Footer.Right = strings.Repeat("🙂", 128) },
		func(value *printing.Settings) { value.HeaderFont.Size = 15 },
		func(value *printing.Settings) { value.FooterFont.Size = 5 },
	} {
		stored := Default()
		changed(&stored.Print)
		if err := Save(path, stored); err == nil {
			t.Fatalf("invalid print settings saved: %+v", stored.Print)
		}
	}
}

// privateMode reports whether mode carries the private-file metadata this
// platform supports: POSIX 0600 on macOS and Linux, or a writable regular file
// on Windows, where Go reports synthetic permission bits and VivaPad keeps the
// privacy of its stores in the profile directory ACL (a separate gate).
func privateMode(mode os.FileMode) bool {
	if runtime.GOOS == "windows" {
		return mode.IsRegular() && mode.Perm()&0200 != 0
	}
	return mode.Perm() == 0600
}

// privateDirMode reports whether mode carries a private directory's metadata:
// POSIX 0700 on macOS and Linux, or a writable directory on Windows.
func privateDirMode(mode os.FileMode) bool {
	if runtime.GOOS == "windows" {
		return mode.IsDir() && mode.Perm()&0200 != 0
	}
	return mode.Perm() == 0700
}
