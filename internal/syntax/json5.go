package syntax

import (
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// JSON5 adds identifier keys, quoted strings, comments, and signed decimal,
// hexadecimal, and nonfinite numbers beyond Chroma's stock JSON rules.
const (
	json5Space  = `[\t\v\f \p{Zs}\uFEFF]`
	json5Key    = `(?=` + json5Space + `*:)`
	json5Ident  = `(?:[$_\p{L}\p{Nl}]|\\u[0-9a-fA-F]{4})(?:[$_\p{L}\p{Nl}\p{Mn}\p{Mc}\p{Nd}\p{Pc}\u200C\u200D]|\\u[0-9a-fA-F]{4})*`
	json5SQBody = `'(?:\\(?:\r\n|[\s\S])|[^'\\\r\n])*`
	json5DQBody = `"(?:\\(?:\r\n|[\s\S])|[^"\\\r\n])*`
	json5SQ     = json5SQBody + `'`
	json5DQ     = json5DQBody + `"`
	json5End    = `(?![$_\p{L}\p{Nl}\p{Mn}\p{Mc}\p{Nd}\p{Pc}\u200C\u200D])`
)

var _ = lexers.Register(chroma.MustNewLexer(&chroma.Config{Name: "JSON5"}, func() chroma.Rules {
	return chroma.Rules{
		"root": {
			{Pattern: `(?:` + json5Space + `|[\r\n\u2028\u2029])+`, Type: chroma.Text},
			{Pattern: `//[^\r\n\u2028\u2029]*`, Type: chroma.CommentSingle},
			{Pattern: `(?s:/\*.*?(?:\*/|\z))`, Type: chroma.CommentMultiline},
			{Pattern: `(?:` + json5Ident + `|` + json5SQ + `|` + json5DQ + `)` + json5Key, Type: chroma.NameTag},
			{Pattern: json5SQ, Type: chroma.LiteralStringSingle},
			{Pattern: json5DQ, Type: chroma.LiteralStringDouble},
			{Pattern: json5SQBody + `\\?\z`, Type: chroma.LiteralStringSingle},
			{Pattern: json5DQBody + `\\?\z`, Type: chroma.LiteralStringDouble},
			{Pattern: `[+-]?(?:Infinity|NaN)` + json5End, Type: chroma.LiteralNumber},
			{Pattern: `[+-]?0[xX][0-9a-fA-F]+` + json5End, Type: chroma.LiteralNumberHex},
			{Pattern: `[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?` + json5End, Type: chroma.LiteralNumber},
			{Pattern: `(?:true|false|null)` + json5End, Type: chroma.KeywordConstant},
			{Pattern: json5Ident, Type: chroma.NameOther},
			{Pattern: `[{}\[\],:]`, Type: chroma.Punctuation},
		},
	}
}))
