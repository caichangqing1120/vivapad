package main

import (
	"math/rand/v2"

	"vivapad/internal/document"
)

func (state *model) editLineTargetsWithBookmarks(doc *document.Document, _ string, edit func()) {
	state.editWithBookmarkAnchor(doc, edit)
}

// randomizeLineOrder applies the Randomize Line Order command for one seed.
// The source whole-buffer marker policy applies even when the shuffled order
// happens to equal the source order: the command still replaces the block, so
// bookmarks are dropped instead of surviving a no-op. Production passes the
// wall-clock seed; tests pass a fixed seed to exercise the identity permutation.
func (state *model) randomizeLineOrder(doc *document.Document, seed uint64) (bool, error) {
	// Capture the replaceable range before the operation: an identity shuffle can
	// normalize the selection, and recomputing afterwards could shrink the range.
	start, end, targeted := doc.RandomizeLineTarget()
	changed, applicable, err := false, false, error(nil)
	state.editLineTargetsWithBookmarks(doc, "replace", func() {
		changed, applicable, err = doc.RandomizeLinesOutcome(rand.New(rand.NewPCG(seed, 1)))
	})
	if !changed && applicable && err == nil && targeted {
		// A shuffle that reproduces the source order records no edit, so the
		// edit-driven marker mapping never runs. Apply the policy only to the
		// replaceable block: ineligible operations (single line, refused edit)
		// must leave every bookmark untouched.
		state.dropBookmarksInLines(doc, start, end)
	}
	return changed, err
}

// dropBookmarksInLines removes bookmarks on the inclusive line range and leaves
// every bookmark outside it intact.
func (state *model) dropBookmarksInLines(doc *document.Document, start, end int) {
	lines := state.bookmarkLines(doc)
	kept := make([]int, 0, len(lines))
	for _, line := range lines {
		if line < start || line > end {
			kept = append(kept, line)
		}
	}
	state.setBookmarkLines(doc, kept)
}
