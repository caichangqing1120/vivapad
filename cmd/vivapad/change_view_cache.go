package main

import (
	"github.com/vivaui/viva/components"

	"vivapad/internal/changehistory"
	"vivapad/internal/document"
)

// changeViewCache keeps the change margin decorations for one document between
// paints. The key is the text, the consumer identity and the saved truth: text
// edits change value, a proven Clear replaces the consumer, and Save changes the
// editions without changing the text.
type changeViewCache struct {
	value                   string
	consumer                *changehistory.Consumer
	atSave                  bool
	revision                uint64
	margin, indicator, both components.CodeEditorDecorations
}

func newChangeViewCache() map[*document.Document]*changeViewCache {
	return make(map[*document.Document]*changeViewCache)
}
