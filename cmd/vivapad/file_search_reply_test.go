package main

import (
	"context"
	"testing"

	"vivapad/internal/searchfiles"
)

func TestCanceledFileJobsDoNotEvictCurrentReply(t *testing.T) {
	t.Run("find", func(t *testing.T) {
		state := newModel()
		canceled, current := state.cancelFileSearch(), state.cancelFileSearch()
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		state.searchCancel, state.filesSearching, state.filesDialogOpen = cancel, true, true
		state.sendFileReply(fileSearchResponse{generation: current, query: "current", result: searchfiles.Result{
			Matches: []searchfiles.Match{{Text: "current"}}, FilesSearched: 1,
		}})
		for range 32 {
			state.sendFileReply(fileSearchResponse{generation: canceled, err: context.Canceled})
		}
		state.consumeFileSearch()
		if state.filesSearching || state.filesDialogOpen || !state.resultsOpen || state.resultQuery != "current" ||
			len(state.fileResults.Matches) != 1 || state.fileResults.Matches[0].Text != "current" || ctx.Err() != context.Canceled {
			t.Fatalf("current find result lost: searching=%v opened=%v query=%q notice=%q", state.filesSearching, state.resultsOpen, state.resultQuery, state.notice)
		}
	})
	t.Run("apply", func(t *testing.T) {
		state := newModel()
		canceled, current := state.cancelFileSearch(), state.searchReplies.advance()
		state.filesApplying, state.replaceConfirmOpen = true, true
		state.sendFileReply(fileSearchResponse{generation: current, kind: fileJobReplaceApply,
			applied: searchfiles.ApplyResult{FilesChanged: 1, Replacements: 2},
		})
		for range 32 {
			state.sendFileReply(fileSearchResponse{generation: canceled, err: context.Canceled})
		}
		state.consumeFileSearch()
		if state.filesApplying || state.replaceConfirmOpen || state.notice != "Replaced 2 matches in 1 files" {
			t.Fatalf("current apply result lost: applying=%v confirmation=%v notice=%q", state.filesApplying, state.replaceConfirmOpen, state.notice)
		}
	})
}
