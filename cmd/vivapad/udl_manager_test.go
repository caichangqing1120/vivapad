package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"vivapad/internal/syntax"
)

func TestUDLManagerAddPreset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "site.nginx")
	if err := os.WriteFile(path, []byte("server { listen 8080; }"), 0600); err != nil {
		t.Fatal(err)
	}
	state, driver := m5Driver(t, path)
	clickID(t, driver, menuScope.At(5))
	clickID(t, driver, udlManagerID)
	clickID(t, driver, udlPresetID)
	clickID(t, driver, udlPresetScope.At(0))
	if state.m5 == nil || state.m5.index != 0 || state.m5.fields[0].String() != "Nginx" || state.languageLabel(state.space.Current()) != "Nginx" {
		t.Fatalf("preset not selected and applied: %+v", state.udls)
	}
	clickID(t, driver, udlPresetID)
	clickID(t, driver, udlPresetScope.At(0))
	stored, err := syntax.LoadUDLs(state.udlPath)
	if err != nil || len(stored) != 2 || stored[1].Name != "Nginx (2)" || len(stored[1].Extensions) != 0 || state.m5.index != 1 {
		t.Fatalf("preset conflict = %+v, %v; dialog=%+v", stored, err, state.m5)
	}
}

func TestUDLManagerImportJSON(t *testing.T) {
	for index, test := range []struct {
		name, feedback string
		want           []syntax.UDL
	}{
		{"skip", "Imported 1 · Skipped 1 · Overwritten 0 · Renamed 0", []syntax.UDL{{Name: "Demo", Extensions: []string{"demo"}}, {Name: "Other"}}},
		{"overwrite", "Imported 1 · Skipped 0 · Overwritten 1 · Renamed 0", []syntax.UDL{{Name: "DEMO", Extensions: []string{"demo"}, LineComment: "#"}, {Name: "Other"}}},
		{"rename", "Imported 1 · Skipped 0 · Overwritten 0 · Renamed 1", []syntax.UDL{{Name: "Demo", Extensions: []string{"demo"}}, {Name: "DEMO (2)", LineComment: "#"}, {Name: "Other"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			state, driver := m5Driver(t)
			if err := state.setUDLDefinitions([]syntax.UDL{{Name: "Demo", Extensions: []string{"demo"}}}, "", ""); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "incoming.json")
			data, err := syntax.ExportUDLBundle([]syntax.UDL{{Name: "DEMO", Extensions: []string{"demo"}, LineComment: "#"}, {Name: "Other"}})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			state.command(udlManagerCommand)
			if err := driver.Pump(); err != nil {
				t.Fatal(err)
			}
			if err := driver.QueueOpenFiles([]string{path}, nil); err != nil {
				t.Fatal(err)
			}
			clickID(t, driver, udlImportID)
			clickID(t, driver, udlPolicyScope.At(index))
			clickID(t, driver, udlChooseImportID)
			request, ok := driver.LastOpenFilesRequest()
			if !ok || request.AllowMultiple || len(request.Filters) != 1 || request.Filters[0].Pattern != "*.json" {
				t.Fatalf("import picker = %+v, %v", request, ok)
			}
			stored, err := syntax.LoadUDLs(state.udlPath)
			if err != nil || !reflect.DeepEqual(stored, test.want) || !reflect.DeepEqual(state.udls, test.want) {
				t.Fatalf("import = %+v, %v; state=%+v", stored, err, state.udls)
			}
			if state.m5 == nil || !strings.Contains(state.m5.message, test.feedback) || state.m5.fields[0].String() != test.want[0].Name {
				t.Fatalf("import feedback or selection incorrect: %+v", state.m5)
			}
		})
	}
}

func TestUDLManagerImportInvalidAndCanceled(t *testing.T) {
	state, driver := m5Driver(t)
	state.language = "zh-CN"
	state.resolveInterfaceLanguage()
	state.command(udlManagerCommand)
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, udlImportID)
	if err := driver.QueueOpenFiles(nil, app.ErrCanceled); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, udlChooseImportID)
	if state.m5 == nil || state.m5.message != "" {
		t.Fatalf("cancel changed the manager: %+v", state.m5)
	}
	invalid := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(invalid, []byte(`{"version":3,"udls":[{"name":"Demo"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueOpenFiles([]string{invalid}, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, udlChooseImportID)
	if len(state.udls) != 0 || !strings.Contains(state.m5.message, "导入失败") || !strings.Contains(state.m5.message, "version 3") {
		t.Fatalf("bad import modified state or missed error: %+v, %q", state.udls, state.m5.message)
	}
	request, _ := driver.LastOpenFilesRequest()
	if request.Title != "导入 JSON" {
		t.Fatalf("localized picker title = %q", request.Title)
	}
	if _, err := os.Stat(state.udlPath); !os.IsNotExist(err) {
		t.Fatalf("invalid import touched storage: %v", err)
	}
}

func TestUDLManagerExportJSON(t *testing.T) {
	state, driver := m5Driver(t)
	definitions := []syntax.UDL{{Name: "First"}, {Name: "Second", LineComment: "#"}}
	if err := state.setUDLDefinitions(definitions, "", ""); err != nil {
		t.Fatal(err)
	}
	state.command(udlManagerCommand)
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, udlNextID)
	for _, all := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "export.json")
		if err := os.WriteFile(path, []byte("old content"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := driver.QueueSaveFile(path, nil); err != nil {
			t.Fatal(err)
		}
		clickID(t, driver, udlExportID)
		want := definitions[1:]
		if all {
			clickID(t, driver, udlExportAllID)
			want = definitions
		} else {
			clickID(t, driver, udlExportSelectedID)
		}
		request, ok := driver.LastSaveFileRequest()
		if !ok || request.DefaultExtension != "json" || !strings.HasSuffix(request.SuggestedName, ".json") {
			t.Fatalf("save picker = %+v, %v", request, ok)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := syntax.ParseUDLBundle(data)
		if err != nil || !reflect.DeepEqual(decoded, want) || !strings.Contains(string(data), "\n  \"version\": 1") {
			t.Fatalf("export all=%t: %s, %v", all, data, err)
		}
		if state.m5 == nil || !strings.Contains(state.m5.message, "Exported") || state.m5.index != 1 {
			t.Fatalf("export feedback or selection incorrect: %+v", state.m5)
		}
		directory, err := os.ReadDir(filepath.Dir(path))
		if err != nil || len(directory) != 1 {
			t.Fatalf("export left temporary files: %v, %v", directory, err)
		}
	}
	clickID(t, driver, udlExportID)
	if err := driver.QueueSaveFile("", app.ErrCanceled); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, udlExportAllID)
	if err := driver.QueueSaveFile(state.udlPath, nil); err != nil {
		t.Fatal(err)
	}
	clickID(t, driver, udlExportAllID)
	if !strings.Contains(state.m5.message, "internal VivaPad data file") {
		t.Fatalf("private destination error = %q", state.m5.message)
	}
	stored, err := syntax.LoadUDLs(state.udlPath)
	if err != nil || !reflect.DeepEqual(stored, definitions) {
		t.Fatalf("export changed private storage: %+v, %v", stored, err)
	}
}
