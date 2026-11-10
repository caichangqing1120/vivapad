package document

import (
	"bytes"
	"errors"
	"fmt"
	"os"
)

var (
	ErrTrashSourceChanged = errors.New("file changed on disk before trash")
	ErrTrashSourceRemains = errors.New("trash operation left the file in place")
)

type TrashPlan struct {
	doc    *Document
	path   string
	source os.FileInfo
}

func (doc *Document) PrepareTrash() (*TrashPlan, error) {
	if doc.Path == "" {
		return nil, fmt.Errorf("untitled document has no disk file to trash")
	}
	info, err := doc.verifyTrashSource(nil)
	if err != nil {
		return nil, err
	}
	return &TrashPlan{doc: doc, path: doc.Path, source: info}, nil
}

func (plan *TrashPlan) Apply(move func(string) error) error {
	if plan == nil || plan.doc == nil || plan.doc.Path != plan.path {
		return ErrTrashSourceChanged
	}
	if _, err := plan.doc.verifyTrashSource(plan.source); err != nil {
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

func (doc *Document) verifyTrashSource(original os.FileInfo) (os.FileInfo, error) {
	info, err := os.Lstat(doc.Path)
	if err != nil || !info.Mode().IsRegular() || (original != nil && !os.SameFile(original, info)) {
		return nil, ErrTrashSourceChanged
	}
	data, err := os.ReadFile(doc.Path)
	if err != nil || !bytes.Equal(data, doc.savedDisk) {
		return nil, ErrTrashSourceChanged
	}
	return info, nil
}
