package document

import "github.com/vivaui/viva/text"

// SelectedSearchTarget returns the current selection or the word at the caret
// without changing the editor selection.
func (doc *Document) SelectedSearchTarget(expand bool) (string, text.Range) {
	selection := doc.Editor.Selection()
	if selection.Start == selection.End && expand {
		if word, ok := doc.MultiSelectionWordAtCaret(); ok {
			selection = word
		}
	}
	if selection.Start == selection.End {
		return "", selection
	}
	value := doc.Editor.String()
	return value[doc.Editor.ByteOffset(selection.Start):doc.Editor.ByteOffset(selection.End)], selection
}
