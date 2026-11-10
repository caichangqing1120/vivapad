package runcommand

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPrepareQuotedArgumentsAndAllOriginalVariables(t *testing.T) {
	vars := Variables{Path: filepath.Join(t.TempDir(), "file name.go"), Word: "hello 世界", Line: 4, Column: 7, LineText: "  println(1)", AppPath: filepath.Join(t.TempDir(), "VivaPad")}
	template := `"/tmp/My Tool" "$(FULL_CURRENT_PATH)" $(CURRENT_DIRECTORY) $(FILE_NAME) $(NAME_PART) $(EXT_PART) "$(CURRENT_WORD)" $(NPP_DIRECTORY) $(NPP_FULL_FILE_PATH) $(CURRENT_LINE) $(CURRENT_COLUMN) "$(CURRENT_LINESTR)" $(UNKNOWN)`
	got, err := Prepare(template, vars)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{vars.Path, filepath.Dir(vars.Path), "file name.go", "file name", ".go", vars.Word, filepath.Dir(vars.AppPath), vars.AppPath, "4", "7", vars.LineText, "$(UNKNOWN)"}
	if got.Executable != "/tmp/My Tool" || got.Dir != filepath.Dir(vars.Path) || !reflect.DeepEqual(got.Args, want) {
		t.Fatalf("prepared = %+v, want args %q", got, want)
	}
}

func TestExpandVariablesCanBeSharedWithPrintHeaders(t *testing.T) {
	vars := Variables{Path: filepath.Join(t.TempDir(), "notes.go"), Word: "猫", Line: 4, Column: 7}
	got := ExpandVariables("$(FILE_NAME) $(FULL_CURRENT_PATH) $(CURRENT_WORD) $(CURRENT_LINE):$(CURRENT_COLUMN) $(UNKNOWN) $(FILE_NAME)", vars)
	want := "notes.go " + vars.Path + " 猫 4:7 $(UNKNOWN) notes.go"
	if got != want {
		t.Fatalf("expanded header = %q, want %q", got, want)
	}
}

func TestPrepareDoesNotInterpretUntrustedVariableAsShellSyntax(t *testing.T) {
	vars := Variables{Word: `; touch /tmp/notepad-run-injected; "quoted"`}
	got, err := Prepare(`echo "$(CURRENT_WORD)"`, vars)
	if err != nil || got.Executable != "echo" || !reflect.DeepEqual(got.Args, []string{vars.Word}) {
		t.Fatalf("variable changed argv: %+v, %v", got, err)
	}
	for _, template := range []string{"", `"broken`, "echo\nsecret", strings.Repeat("x", 4097)} {
		if _, err := Prepare(template, vars); err == nil {
			t.Fatalf("accepted invalid run command %q", template)
		}
	}
}

func TestLaunchStartsProcessWithArgumentsAndWorkingDirectory(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	output := filepath.Join(root, "result.json")
	t.Setenv("VIVAPAD_RUN_HELPER", "1")
	err = Launch(Prepared{Executable: executable, Args: []string{"-test.run=^TestRunLaunchHelper$", "--", output, "argument with spaces"}, Dir: root})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.After(5 * time.Second)
	for {
		data, err := os.ReadFile(output)
		if err == nil {
			var actual struct {
				Dir  string
				Args []string
			}
			if err := json.Unmarshal(data, &actual); err != nil {
				t.Fatal(err)
			}
			if actual.Dir != root || !reflect.DeepEqual(actual.Args, []string{"argument with spaces"}) {
				t.Fatalf("launched process = %+v", actual)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatalf("launched process did not write output: %v", err)
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err := Launch(Prepared{Executable: filepath.Join(root, "missing")}); err == nil {
		t.Fatal("missing executable was reported as launched")
	}
}

func TestRunLaunchHelper(t *testing.T) {
	if os.Getenv("VIVAPAD_RUN_HELPER") != "1" {
		return
	}
	index := -1
	for position, arg := range os.Args {
		if arg == "--" {
			index = position
			break
		}
	}
	if index < 0 || len(os.Args) != index+3 {
		t.Fatalf("unexpected helper arguments %q", os.Args)
	}
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(struct {
		Dir  string
		Args []string
	}{directory, os.Args[index+2:]})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Args[index+1]+".tmp", data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(os.Args[index+1]+".tmp", os.Args[index+1]); err != nil {
		t.Fatal(err)
	}
}
