package syntax

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
)

func TestBuiltinUDLPresets(t *testing.T) {
	tests := []struct {
		name, extensions, sample, keyword, quoted, comment string
	}{
		{"Nginx", "conf nginx", `server_name "demo"; # server`, "server_name", `"demo"`, "# server"},
		{"Dockerfile", "dockerfile", `FROM "image" # FROM`, "FROM", `"image"`, "# FROM"},
		{"Log / StackTrace", "log", "NOTERROR ERROR: details", "ERROR", "", ""},
		{"GraphQL", "graphql gql", `query { field(arg: "query") } # query`, "query", `"query"`, "# query"},
		{"Prisma", "prisma", `model Demo { name String @default("model") } // model`, "model", `"model"`, "// model"},
		{"Protobuf", "proto", `syntax = "proto3"; /* syntax */`, "syntax", `"proto3"`, "/* syntax */"},
		{"Makefile", "mk make", "PHONY .PHONY: test # .PHONY", ".PHONY", "", "# .PHONY"},
		{"Env / Properties", "env properties cfg", `enabled=true name="true" # true`, "true", `"true"`, "# true"},
	}
	presets := BuiltinUDLPresets()
	if len(presets) != len(tests) {
		t.Fatalf("got %d presets, want %d", len(presets), len(tests))
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			preset := presets[index]
			if preset.Name != test.name || strings.Join(preset.Extensions, " ") != test.extensions {
				t.Fatalf("preset = %+v", preset)
			}
			runs := preset.Highlighter(test.sample)("", 0, test.sample, nil)
			previous := 0
			for _, run := range runs {
				if run.Start < previous || run.End <= run.Start || run.End > len(test.sample) {
					t.Fatalf("invalid highlight runs: %+v", runs)
				}
				previous = run.End
			}
			for _, token := range []struct {
				text string
				kind chroma.TokenType
			}{{test.keyword, chroma.Keyword}, {test.quoted, chroma.LiteralString}, {test.comment, chroma.Comment}} {
				if token.text == "" {
					continue
				}
				color, _ := tokenColor(token.kind)
				found := false
				for _, run := range runs {
					found = found || run.Color == color && test.sample[run.Start:run.End] == token.text
				}
				if !found {
					t.Errorf("%q was not highlighted as %v: %+v", token.text, token.kind, runs)
				}
			}
		})
	}
	encoded, err := ExportUDLBundle(presets)
	if err != nil || !bytes.Contains(encoded, []byte("\n  \"version\": 1")) {
		t.Fatalf("export = %s, %v", encoded, err)
	}
	decoded, err := ParseUDLBundle(encoded)
	if err != nil || !reflect.DeepEqual(decoded, presets) {
		t.Fatalf("bundle round trip = %+v, %v", decoded, err)
	}
	path := filepath.Join(t.TempDir(), "udl.json")
	if err := SaveUDLs(path, decoded); err != nil {
		t.Fatal(err)
	}
	stored, err := LoadUDLs(path)
	if err != nil || !reflect.DeepEqual(stored, presets) {
		t.Fatalf("storage round trip = %+v, %v", stored, err)
	}
	presets[0].Extensions[0], presets[0].Keywords[0], presets[0].Quotes[0] = "changed", "changed", "changed"
	if !reflect.DeepEqual(BuiltinUDLPresets(), decoded) {
		t.Fatal("mutating a returned preset changed the preset catalog")
	}
}

func TestUDLBundleRejectsInvalidData(t *testing.T) {
	for _, test := range []struct {
		name, input, errorText string
	}{
		{"empty bundle", `{"version":1,"udls":[]}`, "at least one"},
		{"bad version", `{"version":2,"udls":[{"name":"Demo"}]}`, "version"},
		{"missing version", `{"udls":[{"name":"Demo"}]}`, "version"},
		{"unknown root field", `{"version":1,"udls":[{"name":"Demo"}],"future":true}`, "unknown field"},
		{"unknown definition field", `{"version":1,"udls":[{"name":"Demo","typo":"#"}]}`, "unknown field"},
		{"wrong type", `{"version":1,"udls":[{"name":"Demo","quotes":"\""}]}`, "cannot unmarshal"},
		{"null field", `{"version":1,"udls":[{"name":"Demo","line_comment":null}]}`, "null"},
		{"duplicate field", `{"version":1,"Version":1,"udls":[{"name":"Demo"}]}`, "duplicate field"},
		{"bad delimiters", `{"version":1,"udls":[{"name":"Demo","block_end":"*/"}]}`, "both delimiters"},
		{"bad keyword", `{"version":1,"udls":[{"name":"Demo","keywords":["x.y"]}]}`, "invalid keyword"},
		{"duplicate names", `{"version":1,"udls":[{"name":"Demo"},{"name":"demo"}]}`, "unique"},
		{"duplicate extensions", `{"version":1,"udls":[{"name":"A","extensions":["demo"]},{"name":"B","extensions":[".DEMO"]}]}`, "repeated extension"},
		{"trailing value", `{"version":1,"udls":[{"name":"Demo"}]} {}`, "trailing"},
		{"malformed", `{"version":1,"udls":[`, "JSON"},
		{"legacy array", `[{"name":"Demo"}]`, "JSON"},
		{"bare definition", `{"name":"Demo"}`, "unknown field"},
		{"oversized", strings.Repeat(" ", 2<<20+1), "2 MiB"},
		{"invalid UTF-8", "\xff", "UTF-8"},
	} {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := ParseUDLBundle([]byte(test.input))
			if err == nil || !strings.Contains(err.Error(), test.errorText) || parsed != nil {
				t.Fatalf("parse = %+v, %v; want %q", parsed, err, test.errorText)
			}
		})
	}
	for _, definitions := range [][]UDL{nil, {{Name: ""}}, {{Name: "\xff"}}, {{Name: "Bad", LineComment: "two words"}}, {{Name: "Huge", Keywords: []string{strings.Repeat("a", 2<<20)}}}} {
		if data, err := ExportUDLBundle(definitions); err == nil || data != nil {
			t.Fatal("export accepted invalid definitions")
		}
	}
}

func TestMergeUDLsConflictPolicies(t *testing.T) {
	existing := []UDL{{Name: "Demo", Extensions: []string{"old"}, Keywords: []string{"let"}, Quotes: []string{`"`}}, {Name: "Demo (2)", Extensions: []string{"other"}}}
	incoming := []UDL{{Name: "dEMO", Extensions: []string{".OLD", ".NEW"}, Keywords: []string{"const"}}, {Name: "New"}}
	for _, test := range []struct {
		name   string
		policy ConflictPolicy
		want   []UDL
		report MergeReport
	}{
		{"skip", ConflictSkip, []UDL{existing[0], existing[1], incoming[1]}, MergeReport{Imported: 1, Skipped: 1}},
		{"overwrite", ConflictOverwrite, []UDL{{Name: "dEMO", Extensions: []string{"old", "new"}, Keywords: []string{"const"}}, existing[1], incoming[1]}, MergeReport{Imported: 1, Overwritten: 1}},
		{"rename", ConflictRename, []UDL{existing[0], existing[1], {Name: "dEMO (3)", Extensions: []string{"new"}, Keywords: []string{"const"}}, incoming[1]}, MergeReport{Imported: 1, Renamed: 1, UnassignedExtensions: 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			merged, report, err := MergeUDLs(existing, incoming, test.policy)
			if err != nil || report != test.report || !reflect.DeepEqual(merged, test.want) {
				t.Fatalf("merge = %+v, %+v, %v; want %+v, %+v", merged, report, err, test.want, test.report)
			}
			merged[0].Extensions[0], merged[0].Keywords[0] = "changed", "changed"
			if len(merged[0].Quotes) != 0 {
				merged[0].Quotes[0] = "changed"
			}
			if existing[0].Extensions[0] != "old" || existing[0].Keywords[0] != "let" || existing[0].Quotes[0] != `"` || incoming[0].Extensions[0] != ".OLD" || incoming[0].Keywords[0] != "const" {
				t.Fatal("merged result aliases inputs")
			}
		})
	}
}

func TestMergeUDLsRenameKeepsNaturalNamesAndFitsUTF8(t *testing.T) {
	merged, _, err := MergeUDLs([]UDL{{Name: "Demo"}}, []UDL{{Name: "Demo"}, {Name: "Demo (2)"}}, ConflictRename)
	if err != nil || merged[1].Name != "Demo (3)" || merged[2].Name != "Demo (2)" {
		t.Fatalf("rename reserved incoming name = %+v, %v", merged, err)
	}
	name := strings.Repeat("語", 32)
	merged, _, err = MergeUDLs([]UDL{{Name: name}}, []UDL{{Name: name}}, ConflictRename)
	if err != nil || !utf8.ValidString(merged[1].Name) || len(merged[1].Name) > 96 || !strings.HasSuffix(merged[1].Name, " (2)") {
		t.Fatalf("rename long name = %+v, %v", merged, err)
	}
}

func TestMergeUDLsRejectsInvalidResultAtomically(t *testing.T) {
	existing := []UDL{{Name: "Demo", Extensions: []string{"demo"}}}
	for _, test := range []struct {
		incoming []UDL
		policy   ConflictPolicy
	}{
		{[]UDL{{Name: "Other", Extensions: []string{"demo"}}}, ConflictRename},
		{[]UDL{{Name: "Demo", BlockStart: "/*"}}, ConflictSkip},
		{[]UDL{{Name: "Other"}}, ConflictPolicy(99)},
	} {
		if merged, report, err := MergeUDLs(existing, test.incoming, test.policy); merged != nil || report != (MergeReport{}) || err == nil {
			t.Fatalf("invalid merge = %+v, %+v, %v", merged, report, err)
		}
	}
}
