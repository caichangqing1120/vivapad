package syntax

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Notepad++ v8.9.8.1, PowerEditor/src/langs.model.xml, language kix.
const kixCommands = `? and beep big break call cd cls color cookie1 copy debug del dim display do until exit flushkb for each next function endfunction get gets global go gosub goto if else endif md or password play quit rd redim return run select case endselect set setl setm settime shell sleep small use while loop`
const kixFunctions = `abs addkey addprinterconnection addprogramgroup addprogramitem asc ascan at backupeventlog box cdbl chr cint cleareventlog close comparefiletimes createobject cstr dectohex delkey delprinterconnection delprogramgroup delprogramitem deltree delvalue dir enumgroup enumipinfo enumkey enumlocalgroup enumvalue execute exist existkey expandenvironmentvars fix formatnumber freefilehandle getdiskspace getfileattr getfilesize getfiletime getfileversion getobject iif ingroup instr instrrev int isdeclared join kbhit keyexist lcase left len loadhive loadkey logevent logoff ltrim memorysize messagebox open readline readprofilestring readtype readvalue redirectoutput right rnd round rtrim savekey sendkeys sendmessage setascii setconsole setdefaultprinter setfileattr setfocus setoption setsystemstate settitle setwallpaper showprogramgroup shutdown sidtoname split srnd substr trim ubound ucase unloadhive val vartype vartypename writeline writeprofilestring writevalue`
const kixMacros = `address build color comment cpu crlf csd curdir date day domain dos error fullname homedir homedrive homeshr hostname inwin ipaddress0 ipaddress1 ipaddress2 ipaddress3 kix lanroot ldomain ldrive lm logonmode longhomedir lserver maxpwage mdayno mhz monthno month msecs pid primarygroup priv productsuite producttype pwage ras result rserver scriptdir scriptexe scriptname serror sid site startdir syslang ticks time userid userlang wdayno wksta wuserid ydayno year`

var kixWords = func() map[string]chroma.TokenType {
	words := make(map[string]chroma.TokenType)
	for _, word := range strings.Fields(kixFunctions) {
		words[word] = chroma.NameFunction
	}
	for _, word := range strings.Fields(kixCommands) {
		words[word] = chroma.Keyword
	}
	for _, word := range strings.Fields(kixMacros) {
		words["@"+word] = chroma.NameConstant
	}
	return words
}()

type kixLexer struct{ config chroma.Config }

var _ = lexers.Register(&kixLexer{config: chroma.Config{Name: "KIXtart", Aliases: []string{"kix"}}})

func (l *kixLexer) Config() *chroma.Config                         { return &l.config }
func (l *kixLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *kixLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*kixLexer) AnalyseText(string) float32                       { return 0 }

func (*kixLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanKixLine), nil
}

type KixHighlighter struct{ *lineHighlighter[byte] }

func NewKixHighlighter() *KixHighlighter {
	h := newLineHighlighter(scanKixLine)
	h.color = kixTokenColor
	return &KixHighlighter{h}
}

func scanKixLine(line string, state byte, emit func(int, int, chroma.TokenType)) byte {
	style := func(start, end int, kind chroma.TokenType) {
		if emit != nil && start < end {
			emit(start, end, kind)
		}
	}
	for i := 0; i < len(line); {
		start := i
		ch := line[i]
		switch {
		case state == '/' || state == 0 && strings.HasPrefix(line[i:], "/*"):
			if state == 0 {
				i += 2
			}
			end := strings.Index(line[i:], "*/")
			if end < 0 {
				style(start, len(line), chroma.CommentMultiline)
				return '/'
			}
			i += end + 2
			style(start, i, chroma.CommentMultiline)
			state = 0
		case state != 0 || ch == '"' || ch == '\'':
			if state == 0 {
				state = ch
				i++
			}
			kind := chroma.LiteralStringDouble
			if state == '\'' {
				kind = chroma.LiteralStringSingle
			}
			end := strings.IndexByte(line[i:], state)
			if end < 0 {
				style(start, len(line), kind)
				return state
			}
			i += end + 1
			style(start, i, kind)
			state = 0
		case ch == ';':
			style(start, len(line), chroma.CommentSingle)
			return 0
		case kixDigit(ch) || (ch == '.' || ch == '&') && i+1 < len(line) && kixDigit(line[i+1]):
			i++
			for i < len(line) && kixDigit(line[i]) {
				i++
			}
			style(start, i, chroma.LiteralNumber)
		case ch == '$' || ch == '@' || kixWordByte(ch):
			i++
			for i < len(line) && kixWordByte(line[i]) {
				i++
			}
			if ch == '$' {
				style(start, i, chroma.NameVariable)
			} else if emit != nil {
				if kind, ok := kixWords[strings.ToLower(line[start:i])]; ok {
					style(start, i, kind)
				}
			}
		case ch == '?':
			i++
			style(start, i, chroma.Keyword)
		case strings.IndexByte("+-*/&|<>=", ch) >= 0:
			i++
			style(start, i, chroma.Operator)
		default:
			i++
		}
	}
	return state
}

func kixDigit(ch byte) bool { return ch >= '0' && ch <= '9' }

func kixWordByte(ch byte) bool {
	return ch >= 0x80 || ch == '_' || kixDigit(ch) || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z'
}
