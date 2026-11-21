package document

import (
	"crypto/sha256"
	"encoding/hex"
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
	sourceInfo, sourceDigest, err := snapshotRenameFile(doc.Path)
	if err != nil {
		return nil, err
	}
	if sourceInfo.Size() != int64(len(doc.savedDisk)) || sourceDigest != doc.savedDigest {
		return nil, ErrRenameSourceChanged
	}
	plan := &RenamePlan{Target: target, doc: doc, source: doc.Path, sourceInfo: sourceInfo, sourceDigest: sourceDigest}
	if target == doc.Path {
		return plan, nil
	}
	targetInfo, targetDigest, err := snapshotRenameFile(target)
	if err == nil {
		plan.TargetExists, plan.targetInfo, plan.targetDigest = true, targetInfo, targetDigest
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return plan, nil
}

func snapshotRenameFile(path string) (os.FileInfo, string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, "", err
	}
	if !info.Mode().IsRegular() {
		return nil, "", fmt.Errorf("%s is not a regular file", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, "", err
	}
	if !os.SameFile(info, opened) {
		return nil, "", fmt.Errorf("%s changed while opening", path)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, "", err
	}
	return opened, hex.EncodeToString(hash.Sum(nil)), nil
}

func (plan *RenamePlan) Apply(replace bool) error {
	if plan == nil || plan.doc == nil || plan.doc.Path != plan.source {
		return ErrRenameSourceChanged
	}
	sourceInfo, sourceDigest, err := snapshotRenameFile(plan.source)
	if err != nil || !os.SameFile(plan.sourceInfo, sourceInfo) || sourceDigest != plan.sourceDigest {
		return ErrRenameSourceChanged
	}
	if plan.Target == plan.source {
		return nil
	}
	if plan.TargetExists && !replace {
		return ErrRenameConfirmationRequired
	}
	targetInfo, targetDigest, err := snapshotRenameFile(plan.Target)
	switch {
	case plan.TargetExists && (err != nil || !os.SameFile(plan.targetInfo, targetInfo) || targetDigest != plan.targetDigest):
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
