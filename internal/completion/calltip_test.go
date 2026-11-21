package completion

import (
	"slices"
	"strings"
	"testing"
)

func TestParameterHintTracksNestedMultilineCall(t *testing.T) {
	const data = `<NotepadPlus><AutoComplete><Environment ignoreCase="yes" additionalWordChar="."/><KeyWord name="api.Copy" func="yes"><Overload retVal="int"><Param name="first"/><Param name="second"/></Overload><Overload><Param name="source"/><Param name="target"/></Overload></KeyWord><KeyWord name="inner" func="yes"><Overload><Param name="a"/><Param name="b"/></Overload></KeyWord></AutoComplete></NotepadPlus>`
	catalog, err := ParseFunctionCatalog(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		source, name string
		parameter    int
	}{
		{"API.Copy(\r\n [1,2], # ignore ),\n inner(\"a,b\", ", "inner", 1},
		{"API.Copy(\r\n [1,2], # ignore ),\n inner(\"a,b\", 3), ", "api.Copy", 2},
		{"API.Copy(\"\"\"ignore\n, inner(\"\"\", ", "api.Copy", 1},
		{"API.Copy({1: [2, 3]}, unknown(4, ", "api.Copy", 1},
		{"# API.Copy(", "", 0},
		{"API.Copy(1, 2)\n", "", 0},
	} {
		hint := catalog.CallAt(tc.source, len(tc.source), CallSyntax{LineComment: "#", TripleQuotes: true})
		if hint.Name != tc.name || hint.Argument != tc.parameter {
			t.Errorf("%q: got %q argument %d, want %q argument %d", tc.source, hint.Name, hint.Argument, tc.name, tc.parameter)
		}
	}
	hint := catalog.CallAt("api.Copy(", 9, CallSyntax{LineComment: "#", TripleQuotes: true})
	if len(hint.Overloads) != 2 || hint.Overloads[0].Return != "int" || !slices.Equal(hint.Overloads[1].Params, []string{"source", "target"}) {
		t.Fatalf("overload order or contents lost: %+v", hint.Overloads)
	}
}

func TestBundledGoParameterHint(t *testing.T) {
	catalog, err := LoadFunctionCatalog("", "go")
	if err != nil {
		t.Fatal(err)
	}
	const source = "append([]int{1, 2}, /* ) , /* */ `,` , "
	hint := catalog.CallAt(source, len(source), CallSyntax{LineComment: "//"})
	if hint.Name != "append" || hint.Argument != 2 || len(hint.Overloads) != 1 || len(hint.Overloads[0].Params) != 2 {
		t.Fatalf("bundled Go hint = %+v", hint)
	}
}

func TestRustParameterHintIgnoresRawStringContents(t *testing.T) {
	const data = `<NotepadPlus><AutoComplete language="Rust"><KeyWord name="demo" func="yes"><Overload><Param name="first"/><Param name="second"/></Overload></KeyWord></AutoComplete></NotepadPlus>`
	catalog, err := ParseFunctionCatalog(strings.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		source   string
		argument int
	}{
		{`demo(r#"a", ) demo(, "#, `, 1},
		{"demo(br##\"a\"#\n, ) demo(, \"##, ", 1},
		{`demo(cr#"a", ) demo(, "#, `, 1},
		{`demo(r"a\", `, 1},
		{`demo(r##"open"#, ) demo(, `, 0},
		{`demo(r#name, "regular, )", `, 2},
	} {
		hint := catalog.CallAt(tc.source, len(tc.source), CallSyntax{LineComment: "//"})
		if hint.Name != "demo" || hint.Open != 4 || hint.Argument != tc.argument {
			t.Errorf("%q: got %+v, want outer demo argument %d", tc.source, hint, tc.argument)
		}
	}
}
