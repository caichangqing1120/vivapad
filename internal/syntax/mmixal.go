package syntax

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Notepad++ v8.9.8.1, PowerEditor/src/langs.model.xml, language mmixal.
const mmixalOpcodes = `2ADDU 4ADDU 8ADDU 16ADDU ADD ADDU AND ANDNH ANDNL ANDNMH ANDNML BDIF BEV BN BNN BNP BNZ BOD BP BSPEC BYTE BZ CMP CMPU CSEV CSN CSNN CSNP CSNZ CSOD CSP CSWAP CSZ DIV DIVU ESPEC EXPR FADD FCMP FCMPE FDIV FEQL FEQLE FIX FIXU FLOT FLOTU FMUL FREM FSQRT FSUB FUN FUNE GET GETA GO GREG I_BIT INCH INCL INCMH INCML IS JMP LDA LDB LDBU LDHT LDO LDOU LDSF LDT LDTU LDUNC LDVTS LDW LDWU LOC LOCAL MOR MUL MULU MUX MXOR NAND NEG NEGU NNIX NOR NXOR O_BIT OCTA ODIF OR ORH ORL ORMH ORML ORN PBEV PBN PBNN PBNP PBNZ PBOD PBP PBZ POP PREFIX PREGO PRELD PREST PUSHGO PUSHJ PUT RESUME SAVE SET SETH SETL SETMH SETML SFLOT SFLOTU SL SLU SR SRU STB STBU STCO STHT STO STOU STSF STT STTU STUNC STW STWU SUB SUBU SWYM SYNC SYNCD TDIF TETRA TRAP TRIP UNSAVE WDIF WYDEXOR ZSEV ZSN ZSNN ZSNP ZSNZ ZSOD ZSP ZSZ`
const mmixalRegisters = `rA rB rC rD rE rF rG rH rI rJ rK rL rM rN rO rP rQ rR rS rT rU rV rW rX rY rZ rBB rTT rWW rXX rYY rZZ`
const mmixalSymbols = `@ Text_Segment Data_Segment Pool_Segment Stack_Segment StdErr StdIn StdOut Fopen Fclose Fread Fwrite Fgets Fputs Fgetws Fputws Ftell Fseek TextRead TextWrite BinaryRead BinaryWrite BinaryReadWrite`

var mmixalOpcodeWords = func() map[string]bool {
	words := make(map[string]bool)
	for _, word := range strings.Fields(mmixalOpcodes) {
		words[word] = true
	}
	return words
}()

var mmixalOperandWords = func() map[string]chroma.TokenType {
	words := make(map[string]chroma.TokenType)
	for _, word := range strings.Fields(mmixalRegisters) {
		words[word] = chroma.NameVariable
	}
	for _, word := range strings.Fields(mmixalSymbols) {
		words[word] = chroma.NameConstant
	}
	return words
}()

type mmixalLexer struct{ config chroma.Config }

var _ = lexers.Register(&mmixalLexer{config: chroma.Config{Name: "MMIXAL"}})

func (l *mmixalLexer) Config() *chroma.Config                         { return &l.config }
func (l *mmixalLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return l }
func (l *mmixalLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return l }
func (*mmixalLexer) AnalyseText(string) float32                       { return 0 }

func (*mmixalLexer) Tokenise(_ *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	return tokeniseSyntaxLines(value, scanMMIXALLine), nil
}

func scanMMIXALLine(line string, _ struct{}, emit func(int, int, chroma.TokenType)) struct{} {
	if strings.HasPrefix(line, "@i") {
		emit(0, len(line), chroma.CommentPreproc)
		return struct{}{}
	}
	i := mmixalSkipSpace(line, 0)
	if i == len(line) {
		return struct{}{}
	}
	if !mmixalWordByte(line[i]) {
		emit(i, len(line), chroma.CommentSingle)
		return struct{}{}
	}
	if i == 0 {
		i = mmixalWordEnd(line, 0)
		emit(0, i, chroma.NameLabel)
		i = mmixalSkipSpace(line, i)
	}
	if end := mmixalWordEnd(line, i); end > i {
		kind := chroma.Error
		if mmixalOpcodeWords[line[i:end]] {
			kind = chroma.Keyword
		}
		emit(i, end, kind)
		i = end
	}
	for i = mmixalSkipSpace(line, i); i < len(line); {
		start := i
		ch := line[i]
		i++
		switch {
		case mmixalSpace(ch):
			emit(start, len(line), chroma.CommentSingle)
			return struct{}{}
		case ch == '"' || ch == '\'':
			if end := strings.IndexByte(line[i:], ch); end >= 0 {
				i += end + 1
			} else {
				i = len(line)
			}
			kind := chroma.LiteralStringDouble
			if ch == '\'' {
				kind = chroma.LiteralStringChar
			}
			emit(start, i, kind)
		case ch == '$' || ch == '#':
			for i < len(line) && (mmixalDigit(line[i]) || ch == '#' && mmixalHexLetter(line[i])) {
				i++
			}
			kind := chroma.NameVariable
			if ch == '#' {
				kind = chroma.LiteralNumberHex
			}
			emit(start, i, kind)
		case mmixalWordByte(ch) || ch == '@':
			i = mmixalWordEnd(line, i)
			word := line[start:i]
			if kind, ok := mmixalOperandWords[strings.TrimPrefix(word, ":")]; ok {
				emit(start, i, kind)
			} else if mmixalDigit(ch) && strings.IndexFunc(word, func(r rune) bool { return r < '0' || r > '9' }) < 0 {
				emit(start, i, chroma.LiteralNumberInteger)
			}
		case strings.ContainsRune("+-|^*/%<>&~,()[]", rune(ch)):
			emit(start, i, chroma.Operator)
		}
	}
	return struct{}{}
}

func mmixalWordByte(ch byte) bool {
	return ch == ':' || ch == '_' || mmixalDigit(ch) || ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z'
}

func mmixalWordEnd(line string, start int) int {
	for start < len(line) && mmixalWordByte(line[start]) {
		start++
	}
	return start
}

func mmixalDigit(ch byte) bool     { return ch >= '0' && ch <= '9' }
func mmixalHexLetter(ch byte) bool { return ch >= 'a' && ch <= 'f' || ch >= 'A' && ch <= 'F' }
func mmixalSpace(ch byte) bool     { return ch == ' ' || ch == '\t' || ch == '\v' || ch == '\f' }

func mmixalSkipSpace(line string, start int) int {
	for start < len(line) && mmixalSpace(line[start]) {
		start++
	}
	return start
}
