package syntax

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

type Symbol struct {
	Kind       string
	Name       string
	Line       int
	ByteOffset int
}

func Symbols(mode, filename, value string) ([]Symbol, error) {
	if len(value) > 2<<20 {
		return nil, fmt.Errorf("Function List source exceeds 2 MiB")
	}
	switch mode {
	case ".go":
		return goSymbols(filename, value)
	case ".py", ".rs":
		return lexerSymbols(mode, value)
	}
	return nil, nil
}

func goSymbols(filename, value string) ([]Symbol, error) {
	set := token.NewFileSet()
	file, parseError := parser.ParseFile(set, filename, value, parser.AllErrors)
	if file == nil {
		return nil, parseError
	}
	// Function List positions refer to the open buffer, so ignore //line mappings.
	var result []Symbol
	for _, declaration := range file.Decls {
		switch node := declaration.(type) {
		case *ast.GenDecl:
			if node.Tok != token.TYPE {
				continue
			}
			for _, declaration := range node.Specs {
				if spec, ok := declaration.(*ast.TypeSpec); ok {
					position := set.PositionFor(spec.Name.Pos(), false)
					result = append(result, Symbol{Kind: "type", Name: spec.Name.Name, Line: position.Line, ByteOffset: position.Offset})
				}
			}
		case *ast.FuncDecl:
			if node.Body == nil {
				continue
			}
			name := node.Name.Name
			if node.Recv != nil && len(node.Recv.List) != 0 {
				var receiver bytes.Buffer
				if err := format.Node(&receiver, set, node.Recv.List[0].Type); err == nil {
					name = "(" + receiver.String() + ")." + name
				}
			}
			position := set.PositionFor(node.Name.Pos(), false)
			result = append(result, Symbol{Kind: "function", Name: name, Line: position.Line, ByteOffset: position.Offset})
		}
	}
	return result, parseError
}

func lexerSymbols(mode, value string) ([]Symbol, error) {
	spec, ok := LanguageByMode(mode)
	if !ok {
		return nil, nil
	}
	lexer := lexers.Get(spec.Lexer)
	if lexer == nil {
		return nil, fmt.Errorf("Function List lexer unavailable for %s", spec.Label)
	}
	iterator, err := lexer.Tokenise(&chroma.TokeniseOptions{State: "root"}, value)
	if err != nil {
		return nil, err
	}
	var result []Symbol
	offset, line := 0, 1
	for lexeme := iterator(); lexeme != chroma.EOF; lexeme = iterator() {
		kind := ""
		if lexeme.Type == chroma.NameFunction || lexeme.Type.InSubCategory(chroma.NameFunction) {
			kind = "function"
		} else if mode == ".py" && lexeme.Type == chroma.NameClass {
			kind = "type"
		}
		if kind != "" {
			name := strings.TrimSpace(lexeme.Value)
			if name != "" && !strings.ContainsAny(name, " \t\r\n") {
				result = append(result, Symbol{Kind: kind, Name: name, Line: line, ByteOffset: offset + strings.Index(lexeme.Value, name)})
			}
		}
		offset += len(lexeme.Value)
		line += strings.Count(lexeme.Value, "\n")
	}
	return result, nil
}
