package document

import (
	"errors"
	"fmt"
	"io"
	"os"
)

var (
	ErrTrashSourceChanged = errors.New("file changed on disk before trash")
	ErrTrashSourceRemains = errors.New("trash operation left the file in place")
)

// TrashPlan is a prepared Move to Recycle Bin. PrepareTrash keeps the source
// file open until the plan is applied or closed, so the kernel cannot hand the
// source inode number to a file that replaces it while the decision is pending:
// an open descriptor keeps the original inode alive, which is what makes the
// device+inode identity comparison (os.SameFile between the held descriptor's
// Stat and a fresh Lstat of the path) a reliable replacement detector. Size and
// modification time are deliberately not used as identity: a same-content
// replacement has the same size, and a copy can preserve the timestamp.
//
// Lifetime: a prepared plan owns exactly one descriptor. Apply is one-shot and
// releases it on every return path (success, refusal, backend error). A caller
// that abandons a prepared plan without applying it must call Close; the
// application does that when the confirmation dialog is cancelled or the
// document disappears. Close is idempotent and safe on a nil plan. If a plan is
// dropped without Close the descriptor is still released by the *os.File
// finalizer once the plan becomes unreachable, but explicit Close is the
// supported path.
type TrashPlan struct {
	doc    *Document
	path   string
	file   *os.File
	source os.FileInfo
}

// PrepareTrash captures the source identity and keeps the file open. The caller
// must Close (or Apply, which closes) on every path that abandons the plan.
func (doc *Document) PrepareTrash() (*TrashPlan, error) {
	if doc.Path == "" {
		return nil, fmt.Errorf("untitled document has no disk file to trash")
	}
	path := doc.Path
	if err := doc.CheckDiskPath(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTrashSourceChanged, err)
	}
	file, err := openTrashSource(path)
	if err != nil {
		return nil, ErrTrashSourceChanged
	}
	source, err := file.Stat()
	if err != nil || !source.Mode().IsRegular() {
		file.Close()
		return nil, ErrTrashSourceChanged
	}
	plan := &TrashPlan{doc: doc, path: path, file: file, source: source}
	if err := plan.verify(); err != nil {
		plan.Close()
		return nil, err
	}
	return plan, nil
}

// verify checks that path still names the file the plan holds open and that its
// bytes are still the saved disk content. The held descriptor keeps the original
// inode alive, so a replacement file cannot share its device+inode identity and
// a recycled inode number cannot masquerade as the original.
func (plan *TrashPlan) verify() error {
	if plan == nil || plan.doc == nil || plan.file == nil || plan.source == nil {
		return ErrTrashSourceChanged
	}
	held, err := plan.file.Stat()
	if err != nil || !held.Mode().IsRegular() || !os.SameFile(plan.source, held) {
		return ErrTrashSourceChanged
	}
	current, err := os.Lstat(plan.path)
	if err != nil || !current.Mode().IsRegular() || !os.SameFile(held, current) {
		return ErrTrashSourceChanged
	}
	if _, err := plan.file.Seek(0, io.SeekStart); err != nil {
		return ErrTrashSourceChanged
	}
	matches, err := fileBytesMatch(plan.file, held.Size(), plan.doc.savedDisk)
	if err != nil || !matches {
		return ErrTrashSourceChanged
	}
	return nil
}

// Apply runs the prepared move. It is one-shot: the descriptor is released on
// every return path, and a later call reports ErrTrashSourceChanged without
// calling move.
func (plan *TrashPlan) Apply(move func(string) error) error {
	if plan == nil {
		return ErrTrashSourceChanged
	}
	defer plan.Close()
	if plan.doc == nil || plan.doc.Path != plan.path || plan.file == nil {
		return ErrTrashSourceChanged
	}
	if err := plan.doc.CheckDiskPath(); err != nil {
		return fmt.Errorf("%w: %w", ErrTrashSourceChanged, err)
	}
	if err := plan.verify(); err != nil {
		return err
	}
	if err := move(plan.path); err != nil {
		return err
	}
	if _, err := os.Lstat(plan.path); err == nil {
		return ErrTrashSourceRemains
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Close releases the descriptor held since PrepareTrash. It is idempotent and
// safe on a nil plan.
func (plan *TrashPlan) Close() error {
	if plan == nil || plan.file == nil {
		return nil
	}
	err := plan.file.Close()
	plan.file = nil
	return err
}
