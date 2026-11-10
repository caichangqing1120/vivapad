package syntax

import (
	"reflect"
	"strings"
	"testing"
)

func TestGoSymbolsUseASTTypesMethodsAndByteOffsets(t *testing.T) {
	value := "package sample\n// 中文\ntype Server struct{}\nfunc (s *Server) ServeHTTP() {}\nfunc main() {}\n"
	got, err := Symbols(".go", "sample.go", value)
	if err != nil {
		t.Fatal(err)
	}
	want := []Symbol{
		{Kind: "type", Name: "Server", Line: 3, ByteOffset: strings.Index(value, "Server struct")},
		{Kind: "function", Name: "(*Server).ServeHTTP", Line: 4, ByteOffset: strings.Index(value, "ServeHTTP")},
		{Kind: "function", Name: "main", Line: 5, ByteOffset: strings.Index(value, "main()")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Go symbols = %+v, want %+v", got, want)
	}
}

func TestPythonAndRustSymbolsIgnoreStringsAndLocateRealDefinitions(t *testing.T) {
	python := "class Client:\n    def __init__(self): pass\n    def fetch(self): pass\ndef top(): pass\ntop()\ntext = 'def fake(): pass'\n"
	got, err := Symbols(".py", "client.py", python)
	if err != nil {
		t.Fatal(err)
	}
	want := []Symbol{
		{Kind: "type", Name: "Client", Line: 1, ByteOffset: strings.Index(python, "Client")},
		{Kind: "function", Name: "__init__", Line: 2, ByteOffset: strings.Index(python, "__init__")},
		{Kind: "function", Name: "fetch", Line: 3, ByteOffset: strings.Index(python, "fetch")},
		{Kind: "function", Name: "top", Line: 4, ByteOffset: strings.Index(python, "top()")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Python symbols = %+v, want %+v", got, want)
	}
	rust := "struct Node {}\nimpl Node {\n  fn visit(&self) {}\n}\nfn main() {}\nmain();\nlet x = \"fn fake() {}\";\n"
	got, err = Symbols(".rs", "main.rs", rust)
	if err != nil {
		t.Fatal(err)
	}
	want = []Symbol{
		{Kind: "function", Name: "visit", Line: 3, ByteOffset: strings.Index(rust, "visit")},
		{Kind: "function", Name: "main", Line: 5, ByteOffset: strings.Index(rust, "main()")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Rust symbols = %+v, want %+v", got, want)
	}
}

func TestSymbolsRemainSafeForIncompleteUnsupportedAndOversizedSources(t *testing.T) {
	value := "package sample\nfunc good() {}\nfunc broken(\n"
	got, err := Symbols(".go", "broken.go", value)
	if err == nil || len(got) != 1 || got[0].Name != "good" {
		t.Fatalf("incomplete Go symbols = %+v, %v", got, err)
	}
	if unsupported, err := Symbols(".html", "page.html", "<script>function fake() {}</script>"); err != nil || len(unsupported) != 0 {
		t.Fatalf("unsupported symbol source = %+v, %v", unsupported, err)
	}
	if oversized, err := Symbols(".py", "big.py", strings.Repeat("x", 2<<20+1)); err == nil || len(oversized) != 0 {
		t.Fatalf("oversized symbol source = %+v, %v", oversized, err)
	}
}
