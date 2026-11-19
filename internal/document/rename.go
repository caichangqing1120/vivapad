package document

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
	sourceInfo   os.FileInfo
	targetInfo   os.FileInfo
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
	sourceInfo, source, err := readRenameFile(doc.Path)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(source, doc.savedDisk) {
		return nil, ErrRenameSourceChanged
	}
	plan := &RenamePlan{Target: target, doc: doc, source: doc.Path, sourceInfo: sourceInfo, sourceDigest: digest(source)}
	if target == doc.Path {
		return plan, nil
	}
	targetInfo, data, err := readRenameFile(target)
	if err == nil {
		plan.TargetExists, plan.targetInfo, plan.targetDigest = true, targetInfo, digest(data)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return plan, nil
}

func readRenameFile(path string) (os.FileInfo, []byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%s is not a regular file", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !os.SameFile(info, opened) {
		return nil, nil, fmt.Errorf("%s changed while opening", path)
	}
	data, err := io.ReadAll(file)
	return opened, data, err
}

func (plan *RenamePlan) Apply(replace bool) error {
	if plan == nil || plan.doc == nil || plan.doc.Path != plan.source {
		return ErrRenameSourceChanged
	}
	sourceInfo, source, err := readRenameFile(plan.source)
	if err != nil || !os.SameFile(plan.sourceInfo, sourceInfo) || digest(source) != plan.sourceDigest {
		return ErrRenameSourceChanged
	}
	if plan.Target == plan.source {
		return nil
	}
	if plan.TargetExists && !replace {
		return ErrRenameConfirmationRequired
	}
	targetInfo, data, err := readRenameFile(plan.Target)
	switch {
	case plan.TargetExists && (err != nil || !os.SameFile(plan.targetInfo, targetInfo) || digest(data) != plan.targetDigest):
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
