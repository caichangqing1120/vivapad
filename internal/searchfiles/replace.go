package searchfiles

import (
	"context"
	"errors"
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
}

type ReplacePlan struct {
	Changes       []FileChange
	Replacements  int
	FilesSearched int
	FilesSkipped  int
	maxFileBytes  int64
	query         string
	replacement   string
	search        document.SearchOptions
	applied       bool
}

type ApplyResult struct {
	FilesChanged int
	Replacements int
	Paths        []string
}

func PreviewReplace(ctx context.Context, root, query, replacement string, search document.SearchOptions, options Options) (*ReplacePlan, error) {
	found, err := find(ctx, root, query, search, options, false)
	if err != nil {
		return nil, err
	}
	if found.Truncated {
		return nil, fmt.Errorf("match limit reached; narrow the search before replacing")
	}
	plan := &ReplacePlan{
		FilesSearched: found.FilesSearched, FilesSkipped: found.FilesSkipped, maxFileBytes: options.fileByteLimit(),
		query: query, replacement: replacement, search: search,
	}
	seen := make(map[string]bool)
	for _, match := range found.Matches {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if seen[match.Path] {
			continue
		}
		seen[match.Path] = true
		doc, err := document.OpenSearchDocument(match.Path, match.Encoding, plan.maxFileBytes)
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
		if count == 0 {
			continue
		}
		changed, err := doc.SaveDiffersFromSnapshot()
		if err != nil {
			return nil, fmt.Errorf("%w (%s cannot be replaced)", err, match.Path)
		}
		if !changed {
			continue
		}
		plan.Changes = append(plan.Changes, FileChange{Path: match.Path, Count: count, Preview: match.Preview, Encoding: match.Encoding, digest: match.DiskDigest})
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
			return result, fmt.Errorf("target not a regular file; no files written: %s", change.Path)
		}
		if info.Mode().Perm()&0222 == 0 {
			return result, fmt.Errorf("read-only file; no files written: %s: %w", change.Path, os.ErrPermission)
		}
		current, err := document.ReadSearchSource(change.Path, change.Encoding, plan.maxFileBytes)
		if errors.Is(err, document.ErrSearchFileTooLarge) {
			return result, fmt.Errorf("%w; no files written: %s", err, change.Path)
		}
		if err != nil || current.Path != change.Path || current.DiskDigest != change.digest {
			return result, fmt.Errorf("file changed after preview; no files written: %s", change.Path)
		}
	}
	for _, change := range plan.Changes {
		if err := ctx.Err(); err != nil {
			return result, fmt.Errorf("replace stopped after %d files: %w", result.FilesChanged, err)
		}
		if err := plan.writeChange(change); err != nil {
			return result, fmt.Errorf("replace failed after %d files at %s: %w", result.FilesChanged, change.Path, err)
		}
		result.FilesChanged++
		result.Replacements += change.Count
		result.Paths = append(result.Paths, change.Path)
	}
	plan.applied = true
	return result, nil
}

func (plan *ReplacePlan) writeChange(change FileChange) error {
	// Rebuild one draft at a time and reject changes made since batch preflight.
	doc, err := document.OpenSearchDocument(change.Path, change.Encoding, plan.maxFileBytes)
	if err != nil {
		return err
	}
	if doc.Path != change.Path || doc.DiskDigest() != change.digest {
		return fmt.Errorf("file changed after preview")
	}
	count, err := doc.ReplaceAllWith(plan.query, plan.replacement, plan.search)
	if err != nil {
		return err
	}
	if count != change.Count {
		return fmt.Errorf("replacement count changed after preview")
	}
	return doc.Save("")
}
