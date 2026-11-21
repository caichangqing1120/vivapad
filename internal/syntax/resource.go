package syntax

import (
	"slices"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Resource scripts use the C-family lexer with a separate keyword list in
// Notepad++ v8.9.8.1 (PowerEditor/src/langs.model.xml, language rc).
const resourceWords = `ACCELERATORS ALT AUTO3STATE AUTOCHECKBOX AUTORADIOBUTTON BEGIN BITMAP BLOCK BUTTON CAPTION CHARACTERISTICS CHECKBOX CLASS COMBOBOX CONTROL CTEXT CURSOR DEFPUSHBUTTON DIALOG DIALOGEX DISCARDABLE EDITTEXT END EXSTYLE FONT GROUPBOX ICON LANGUAGE LISTBOX LTEXT MENU MENUEX MENUITEM MESSAGETABLE POPUP PUSHBUTTON RADIOBUTTON RCDATA RTEXT SCROLLBAR SEPARATOR SHIFT STATE3 STRINGTABLE STYLE TEXTINCLUDE VALUE VERSION VERSIONINFO VIRTKEY`

var resourceKeywords = strings.Fields(resourceWords)

var resourceConfig = chroma.Config{Name: "Windows Resource"}

type resourceLexer struct{ chroma.Lexer }

func (resourceLexer) Config() *chroma.Config { return &resourceConfig }

func (lexer resourceLexer) Tokenise(options *chroma.TokeniseOptions, value string) (chroma.Iterator, error) {
	next, err := lexer.Lexer.Tokenise(options, value)
	if err != nil {
		return nil, err
	}
	return func() chroma.Token {
		token := next()
		if token.Type.InCategory(chroma.Name) || token.Type.InCategory(chroma.Keyword) {
			token.Type = chroma.Text
			if _, found := slices.BinarySearch(resourceKeywords, token.Value); found {
				token.Type = chroma.Keyword
			}
		}
		return token
	}, nil
}

var _ = lexers.Register(resourceLexer{Lexer: lexers.Get("C++")})
