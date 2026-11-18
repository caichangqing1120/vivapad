package searchfiles

import (
	"context"
	"fmt"
	"os"

	"vivapad/internal/document"
)

type FileChange struct {
	Path     string
	Count    int
	Preview  string
	Encoding document.Encoding
	digest   string
	doc      *document.Document
}

type ReplacePlan struct {
	Changes       []FileChange
	Replacements  int
	FilesSearched int
	FilesSkipped  int
	applied       bool
}

type ApplyResult struct {
	FilesChanged int
	Replacements int
	Paths        []string
}

func PreviewReplace(ctx context.Context, root, query, replacement string, search document.SearchOptions, options Options) (*ReplacePlan, error) {
	found, err := Find(ctx, root, query, search, options)
	if err != nil {
		return nil, err
	}
	if found.Truncated {
		return nil, fmt.Errorf("match limit reached; narrow the search before replacing")
	}
	plan := &ReplacePlan{FilesSearched: found.FilesSearched, FilesSkipped: found.FilesSkipped}
	seen := make(map[string]bool)
	for _, match := range found.Matches {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if seen[match.Path] {
			continue
		}
		seen[match.Path] = true
		doc, err := openWithEncoding(match.Path, match.Encoding)
		if err != nil {
			return nil, fmt.Errorf("%s changed during preview: %w", match.Path, err)
		}
		if doc.Path != match.Path || doc.DiskDigest() != match.DiskDigest {
			return nil, fmt.Errorf("%s changed during preview", match.Path)
		}
		count, err := doc.ReplaceAllWith(query, replacement, search)
		if err != nil {
			return nil, err
		}
		if count == 0 || !doc.Dirty() {
			continue
		}
		if err := doc.ValidateEncoding(); err != nil {
			return nil, fmt.Errorf("%s cannot be replaced: %w", match.Path, err)
		}
		plan.Changes = append(plan.Changes, FileChange{Path: match.Path, Count: count, Preview: match.Preview, Encoding: match.Encoding, digest: match.DiskDigest, doc: doc})
		plan.Replacements += count
	}
	return plan, nil
}

func (plan *ReplacePlan) Apply(ctx context.Context) (ApplyResult, error) {
	result := ApplyResult{}
	if plan == nil || plan.applied {
		return result, fmt.Errorf("replacement plan is unavailable or already applied")
	}
	for _, change := range plan.Changes {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		info, err := os.Lstat(change.Path)
		if err != nil || !info.Mode().IsRegular() {
			return result, fmt.Errorf("%s is no longer a regular file", change.Path)
		}
		current, err := openWithEncoding(change.Path, change.Encoding)
		if err != nil || current.Path != change.Path || current.DiskDigest() != change.digest {
			return result, fmt.Errorf("file changed after preview; no files written: %s", change.Path)
		}
	}
	for _, change := range plan.Changes {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("replace stopped after %d files: %w", result.FilesChanged, err)
		}
		if err := change.doc.Save(""); err != nil {
			return result, fmt.Errorf("replace failed after %d files at %s: %w", result.FilesChanged, change.Path, err)
		}
		result.FilesChanged++
		result.Replacements += change.Count
		result.Paths = append(result.Paths, change.Path)
	}
	plan.applied = true
	return result, nil
}
