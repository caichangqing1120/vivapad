package document

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	ErrRenameConfirmationRequired = errors.New("destination replacement needs confirmation")
	ErrRenameSourceChanged        = errors.New("source changed on disk before rename")
	ErrRenameTargetChanged        = errors.New("destination changed before rename")
)

type RenamePlan struct {
	Target       string
	TargetExists bool

	doc          *Document
	source       string
	sourceDigest string
	targetDigest string
}

func (doc *Document) PrepareRename(path string) (*RenamePlan, error) {
	if doc.Path == "" {
		return nil, fmt.Errorf("untitled document has no disk file to rename")
	}
	target, err := CanonicalSavePath(path)
	if err != nil {
		return nil, err
	}
	source, err := os.ReadFile(doc.Path)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(source, doc.savedDisk) {
		return nil, ErrRenameSourceChanged
	}
	plan := &RenamePlan{Target: target, doc: doc, source: doc.Path, sourceDigest: digest(source)}
	if target == doc.Path {
		return plan, nil
	}
	data, err := os.ReadFile(target)
	if err == nil {
		plan.TargetExists, plan.targetDigest = true, digest(data)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return plan, nil
}

func (plan *RenamePlan) Apply(replace bool) error {
	if plan == nil || plan.doc == nil || plan.doc.Path != plan.source {
		return ErrRenameSourceChanged
	}
	source, err := os.ReadFile(plan.source)
	if err != nil || digest(source) != plan.sourceDigest {
		return ErrRenameSourceChanged
	}
	if plan.Target == plan.source {
		return nil
	}
	if plan.TargetExists && !replace {
		return ErrRenameConfirmationRequired
	}
	data, err := os.ReadFile(plan.Target)
	switch {
	case plan.TargetExists && (err != nil || digest(data) != plan.targetDigest):
		return ErrRenameTargetChanged
	case !plan.TargetExists && err == nil:
		return ErrRenameTargetChanged
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return err
	}
	if err := os.Rename(plan.source, plan.Target); err != nil {
		return err
	}
	plan.doc.Path, plan.doc.Name = plan.Target, filepath.Base(plan.Target)
	return nil
}
