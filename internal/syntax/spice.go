package syntax

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Notepad++ v8.9.8.1, PowerEditor/src/langs.model.xml, language spice.
var spiceWords = func() map[string]chroma.TokenType {
	groups := []struct {
		kind  chroma.TokenType
		words string
	}{
		{chroma.Keyword, `ac alias alter alterparam append askvalues assertvalid autoscale break compose copy copytodoc dc delete destroy destroyvec diff display disto dowhile echo else end errorstop fftinit filter foreach fourier freqtotime function functionundef goto homecursors if isdisplayed label let linearize listing load loadaccumulator makelabel movelabel makesmithplot movecursorleft movecursorright msgbox nameplot newplot nextparam noise nopoints op plot plotf plotref poly print printcursors printevent printname printplot printstatus printtext printtol printunits printval printvector pwl pz quit removesmithplot rename repeat resume rotate runs rusage save sendplot sendscript sens set setcursor setdoc setlabel setlabeltype setmargins setnthtrigger setunits setvec setparam setplot setquery setscaletype settracecolor settracestyle setsource settrigger setvec setxlimits setylimits show showmod sort status step stop switch tf timetofreq timetowave tran unalias unlet unset unalterparam update version view wavefilter wavetotime where while write`},
		{chroma.NameFunction, `abs askvalue atan average ceil cos db differentiate differentiatex exp finalvalue floor getcursorx getcursory getcursory0 getcursory1 getparam im ln initialvalue integrate integratex interpolate isdef isdisplayed j log length mag, max maxscale mean meanpts min minscale nextplot nextvector norm operatingpoint ph phase phaseextend pk_pk pos pulse re rms rmspts rnd sameplot sin sqrt stddev stddevpts tan tfall tolerance trise unitvec vector`},
		{chroma.KeywordDeclaration, `param nodeset include options dcconv subckt ends model`},
	}
	words := make(map[string]chroma.TokenType)
	for _, group := range groups {
		for _, word := range strings.Fields(group.words) {
			if _, exists := words[word]; !exists {
				words[word] = group.kind
			}
		}
	}
	return words
}()

type spiceLexer struct{ config chroma.Config }

var _ = lexers.Register(&spiceLexer{config: chroma.Config{Name: "Spice"}})

func (l *spiceLexer) Config() *chroma.Config                         { return &l.config }
func (l *spiceLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *spiceLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*spiceLexer) AnalyseText(string) float32                       { return 0 }

func (*spiceLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanSpiceLine), nil
}

func scanSpiceLine(line string, _ struct{}, emit func(int, int, chroma.TokenType)) struct{} {
	for i := 0; i < len(line); {
		start := i
		ch := line[i]
		switch {
		case ch == '*' && (i == 0 || strings.HasPrefix(line[i:], "*~")):
			emit(i, len(line), chroma.CommentSingle)
			return struct{}{}
		case spiceSpace(ch):
			i++
		case spiceDelimiter(ch):
			i++
			emit(start, i, chroma.Punctuation)
		case ch == '#' || ch >= '0' && ch <= '9':
			i = spiceNumberEnd(line, start+1)
			emit(start, i, chroma.LiteralNumber)
		default:
			for i < len(line) && !spiceSpace(line[i]) && !spiceDelimiter(line[i]) {
				i++
			}
			if kind, ok := spiceWords[strings.ToLower(line[start:i])]; ok {
				emit(start, i, kind)
			}
		}
	}
	return struct{}{}
}

func spiceNumberEnd(line string, i int) int {
	for i < len(line) {
		ch := line[i]
		if spiceSpace(ch) || spiceDelimiter(ch) {
			decimalPoint := ch == '.' && !strings.HasPrefix(line[i:], "..")
			exponentSign := (ch == '+' || ch == '-') && i+1 < len(line) &&
				(line[i-1] == 'e' || line[i-1] == 'E') && line[i+1] >= '0' && line[i+1] <= '9'
			if !decimalPoint && !exponentSign {
				break
			}
		}
		i++
	}
	return i
}

func spiceSpace(ch byte) bool { return ch == ' ' || ch == '\t' || ch == '\v' || ch == '\f' }

func spiceDelimiter(ch byte) bool { return strings.IndexByte("&'()*+,-./:;<=>|", ch) >= 0 }
