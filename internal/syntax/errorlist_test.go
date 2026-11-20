package syntax

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

func TestErrorListRecognizesSourceDiagnosticFormats(t *testing.T) {
	fixtures := []struct {
		line string
		kind chroma.TokenType
	}{
		{`C:\my project\main.c:7:4: error: invalid type`, chroma.GenericError},
		{"/tmp/caf\u00e9.cpp:11: warning: unused", chroma.GenericError},
		{`C:\my project\app.cpp(23,4): error C0001: failed`, chroma.GenericError},
		{`app.cpp(23): NOTE C0001: failed`, chroma.GenericError},
		{`NMAKE : fatal error U1077: exit code 1`, chroma.GenericError},
		{`app.obj : warning LNK4099: missing data`, chroma.GenericError},
		{`app.cpp: warning C4000: unknown pragma`, chroma.GenericError},
		{`  File "example.py", line 5, in main`, chroma.GenericTraceback},
		{"\tat demo.Main.run(Main.java:5)", chroma.GenericTraceback},
		{`   at Demo.Main() in demo.cs:line 20`, chroma.GenericTraceback},
		{`PHP Warning: missing value in demo.php on line 5`, chroma.GenericTraceback},
		{`Undefined value at demo.pl line 5.`, chroma.GenericTraceback},
		{`demo.lua: at line 5 in file demo.lua`, chroma.GenericTraceback},
		{`lua: demo.lua:9: attempt to index nil`, chroma.GenericTraceback},
		{`Warning E1024: invalid value`, chroma.GenericError},
		{`fortcom: Error: invalid type`, chroma.GenericError},
		{`cf90-123: invalid type`, chroma.GenericError},
		{`Line 45, file demo.f90`, chroma.GenericError},
		{`line 42 column 3 - Warning: missing tag`, chroma.GenericError},
		{`./demo.sh: line 7: missing command`, chroma.GenericError},
		{`In file included from main.c:7:`, chroma.GenericEmph},
		{`   73 |     call();`, chroma.GenericEmph},
		{"symbol\tmain.c\t/^symbol();$/;\"", chroma.GenericEmph},
		{"entry\tmain.c\t42", chroma.GenericEmph},
		{`>make`, chroma.GenericPrompt},
		{`+++ revised.c`, chroma.GenericHeading},
		{`--- original.c`, chroma.GenericHeading},
		{`+new line`, chroma.GenericInserted},
		{`-old line`, chroma.GenericDeleted},
		{`< old context`, chroma.GenericDeleted},
		{`! changed context`, chroma.GenericEmph},
		{`docs: text, not a location`, chroma.Text},
		{`app.cpp(23): summary`, chroma.Text},
		{`error: line 7 is unfinished`, chroma.Text},
		{`file.txt:broken: text`, chroma.Text},
		{"  ordinary output", chroma.Text},
	}
	lexer := lexers.Get("ErrorList")
	if lexer == nil || lexer.Config().Name != "ErrorList" {
		t.Fatal("ErrorList lexer is missing")
	}
	var source strings.Builder
	for i, fixture := range fixtures {
		if i > 0 {
			source.WriteString([]string{"\r\n", "\n", "\r"}[i%3])
		}
		source.WriteString(fixture.line)
	}
	next, err := lexer.Tokenise(nil, source.String())
	if err != nil {
		t.Fatal(err)
	}
	var decoded strings.Builder
	styles := make([]chroma.TokenType, 0, source.Len())
	for token := next(); token != chroma.EOF; token = next() {
		decoded.WriteString(token.Value)
		for range len(token.Value) {
			styles = append(styles, token.Type)
		}
	}
	if decoded.String() != source.String() {
		t.Fatal("tokenisation changed source bytes")
	}
	pos := 0
	for i, fixture := range fixtures {
		if i > 0 {
			pos += len([]string{"\r\n", "\n", "\r"}[i%3])
		}
		for _, style := range styles[pos : pos+len(fixture.line)] {
			if style != fixture.kind {
				t.Errorf("%q = %v, want %v", fixture.line, style, fixture.kind)
				break
			}
		}
		pos += len(fixture.line)
	}
}
