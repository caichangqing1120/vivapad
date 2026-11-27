package main

import (
	"context"
	"fmt"

	"vivapad/internal/document"
	"vivapad/internal/largefile"
	"vivapad/internal/searchfiles"
)

type largeSearchJob struct {
	doc      *document.Document
	query    string
	options  document.SearchOptions
	revision uint64
	all      bool
	cancel   context.CancelFunc
	reply    chan largeSearchReply
}

type largeSearchReply struct {
	result largefile.SearchResult
	hit    largefile.Match
	found  bool
	err    error
}

func (state *model) cancelLargeSearch() {
	if job := state.largeSearch; job != nil {
		job.cancel()
		<-job.reply
		state.largeSearch = nil
	}
}

func (state *model) startLargeSearch(doc *document.Document, all, backward bool) {
	query := state.query.String()
	if query == "" {
		state.notice = "Enter text to find"
		return
	}
	state.cancelLargeSearch()
	state.cancelFileSearch()
	file := doc.LargeFile()
	ctx, cancel := context.WithCancel(context.Background())
	job := &largeSearchJob{doc: doc, query: query, options: state.searchOptions, all: all, cancel: cancel,
		revision: file.Progress().Revision, reply: make(chan largeSearchReply, 1)}
	state.largeSearch = job
	view := state.largeView(doc, state.activePane)
	_, _, boundary := view.position()
	if view.anchor != view.caret && backward {
		caret := view.caret
		view.caret = min(view.anchor, view.caret)
		_, _, boundary = view.position()
		view.caret = caret
	} else if view.hit.Line != 0 && view.hit.Offset == view.hit.End && boundary == view.hit.Offset && !backward {
		boundary++
	}
	if all {
		state.discardFileResults()
	}
	state.notice = "Searching large file..."
	wake := state.invalidator.Invalidate
	go func() {
		options := largefile.SearchOptions(job.options)
		var reply largeSearchReply
		if all {
			reply.result, reply.err = file.Search(ctx, query, options, largefile.MaxMatches)
		} else {
			reply.hit, reply.found, reply.err = file.Find(ctx, query, options, boundary, backward)
		}
		job.reply <- reply
		wake()
	}()
}

func (state *model) consumeLargeSearch() {
	job := state.largeSearch
	if job == nil {
		return
	}
	select {
	case reply := <-job.reply:
		state.largeSearch = nil
		job.cancel()
		if state.space.Current() != job.doc || state.query.String() != job.query || state.searchOptions != job.options {
			return
		}
		if reply.err != nil {
			state.notice = reply.err.Error()
			return
		}
		if job.doc.LargeFile().Progress().Revision != job.revision {
			state.notice = staleSearchResultNotice
			return
		}
		if !job.all {
			if reply.found {
				state.jumpLarge(job.doc, reply.hit.Line, reply.hit)
				state.notice = ""
			} else {
				state.notice = "Text not found"
			}
			return
		}
		result := searchfiles.Result{FilesSearched: 1, Truncated: reply.result.Truncated}
		for _, match := range reply.result.Matches {
			result.Matches = append(result.Matches, searchfiles.Match{Path: job.doc.Path, Line: match.Line, Column: match.Column,
				Start: int(match.Offset), End: int(match.End), Preview: match.Preview})
		}
		state.replaceFileResults(result)
		state.resultDocument, state.resultQuery, state.largeResultRevision = job.doc, job.query, job.revision
		state.resultOffset, state.resultSelected = 0, -1
		state.resultsOpen, state.searchOpen, state.goToOpen = true, false, false
		state.notice = fmt.Sprintf("%d matches in current document", len(result.Matches))
		if result.Truncated {
			state.notice = "Showing first 2000 matches in current document"
		}
	default:
	}
}

func (state *model) openLargeResult(doc *document.Document, match searchfiles.Match, index int) bool {
	if doc.LargeFile().Progress().Revision != state.largeResultRevision {
		state.notice = staleSearchResultNotice
		return false
	}
	state.cancelLiveComposition()
	state.space.Select(state.documentIndex(doc))
	state.activateOpenedDocument(doc)
	state.jumpLarge(doc, match.Line, largefile.Match{Line: match.Line, Column: match.Column, Offset: int64(match.Start), End: int64(match.End)})
	state.selectFileResult(index)
	state.notice = ""
	return true
}
