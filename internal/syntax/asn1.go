package syntax

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Notepad++ v8.9.8.1, PowerEditor/src/langs.model.xml, language asn1.
var asn1Words = makeASN1Words([4]string{
	`ACCESS AGENT AUGMENTS BEGIN BITS CAPABILITIES CHOICE COMPLIANCE CONTACT CONVENTION DEFINITIONS DEFVAL DESCRIPTION DISPLAY END ENTERPRISE EXPORTS FALSE FROM GROUP GROUPS HINT IDENTITY IMPLIED IMPORTS INCLUDES INDEX INFO LAST MANDATORY MAX MIN MODULE NOTATION NOTIFICATION NULL OBJECTS OBJECT-TYPE OF ORGANIZATION PRODUCT RELEASE REFERENCE REQUIRES REVISION SEQUENCE SIZE STATUS SUPPORTS SYNTAX TEXTUAL TRAP TYPE TRAP-TYPE UPDATED VALUE VARIABLES VARIATION WRITE`,
	`accessible create current deprecated for mandatory not notify not-accessible obsolete only optional read read-only read-write write`,
	`ABSENT ANY APPLICATION BIT BOOLEAN BY COMPONENT COMPONENTS DEFAULT DEFINED ENUMERATED EXPLICIT EXTERNAL IMPLICIT INIFINITY MAX MIN MINUS OPTIONAL PRESENT PRIVATE REAL SET TAGS TRUE`,
	`Counter Counter32 Counter64 DisplayString Gauge Gauge32 IDENTIFIER INTEGER Integer32 IpAddress NetworkAddress NsapAddress OBJECT OCTET Opaque PhysAddress STRING TimeTicks UInteger32 UNITS Unsigned32`,
})

func makeASN1Words(groups [4]string) map[string]chroma.TokenType {
	kinds := [4]chroma.TokenType{chroma.Keyword, chroma.KeywordConstant, chroma.NameClass, chroma.KeywordType}
	words := make(map[string]chroma.TokenType)
	for i, group := range groups {
		for _, word := range strings.Fields(group) {
			if _, seen := words[word]; !seen {
				words[word] = kinds[i]
			}
		}
	}
	return words
}

type asn1State struct {
	quoted, assignment, oid bool
}

type asn1Lexer struct{ config chroma.Config }

var _ = lexers.Register(&asn1Lexer{config: chroma.Config{Name: "ASN.1", Aliases: []string{"asn1"}}})

func (l *asn1Lexer) Config() *chroma.Config                         { return &l.config }
func (l *asn1Lexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *asn1Lexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*asn1Lexer) AnalyseText(string) float32                       { return 0 }

func (*asn1Lexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanASN1Line), nil
}

type ASN1Highlighter struct{ *lineHighlighter[asn1State] }

func NewASN1Highlighter() *ASN1Highlighter {
	return &ASN1Highlighter{newLineHighlighter(scanASN1Line)}
}

func scanASN1Line(line string, state asn1State, emit func(int, int, chroma.TokenType)) asn1State {
	style := func(start, end int, kind chroma.TokenType) {
		if emit != nil && start < end {
			emit(start, end, kind)
		}
	}
	for i := 0; i < len(line); {
		start := i
		ch := line[i]
		if state.quoted || ch == '"' {
			if !state.quoted {
				i++
				state.assignment = false
			}
			end := strings.IndexByte(line[i:], '"')
			state.quoted = end < 0
			if end < 0 {
				i = len(line)
			} else {
				i += end + 1
			}
			style(start, i, chroma.LiteralStringDouble)
			continue
		}
		if strings.HasPrefix(line[i:], "--") {
			style(i, len(line), chroma.CommentSingle)
			break
		}
		if ch == ' ' || ch == '\t' {
			i++
			continue
		}
		assigned := state.assignment
		state.assignment = false
		switch {
		case !state.oid && strings.HasPrefix(line[i:], "::="):
			i += 3
			style(start, i, chroma.Operator)
			state.assignment = true
		case ch == '{' && assigned:
			state.oid = true
			i++
		case ch == '}':
			state.oid = false
			i++
		case asn1Digit(ch):
			i++
			for i < len(line) && asn1Digit(line[i]) {
				i++
			}
			kind := chroma.LiteralNumberInteger
			if state.oid || assigned {
				kind = chroma.LiteralNumber
			}
			style(start, i, kind)
		case asn1Letter(ch) || ch == '-':
			i++
			for i < len(line) && (asn1Letter(line[i]) || asn1Digit(line[i]) || line[i] == '-' && !strings.HasPrefix(line[i:], "--")) {
				i++
			}
			if emit != nil && !state.oid {
				if kind, ok := asn1Words[line[start:i]]; ok {
					style(start, i, kind)
				}
			}
		default:
			i++
		}
	}
	return state
}

func asn1Digit(ch byte) bool  { return ch >= '0' && ch <= '9' }
func asn1Letter(ch byte) bool { return ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' }
