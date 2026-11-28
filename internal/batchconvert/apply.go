package batchconvert

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"vivapad/internal/document"
)

var errChanged = errors.New("file changed since preview; preview again")

type revision struct {
	info   os.FileInfo
	digest [sha256.Size]byte
}

func sameRevision(before, after os.FileInfo) bool {
	return os.SameFile(before, after) && before.Mode() == after.Mode() &&
		before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}

func (preview *Preview) openRoot() (*os.Root, error) {
	if err := preview.checkRoot(); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(preview.root)
	if err != nil {
		return nil, err
	}
	info, err := root.Stat(".")
	if err != nil || !os.SameFile(preview.rootInfo, info) {
		root.Close()
		return nil, fmt.Errorf("directory no longer matches preview")
	}
	return root, nil
}

func (preview *Preview) checkRoot() error {
	resolved, err := filepath.EvalSymlinks(preview.root)
	if err != nil {
		return err
	}
	info, err := os.Lstat(preview.root)
	if err != nil {
		return err
	}
	if resolved != preview.root || !info.IsDir() || !os.SameFile(info, preview.rootInfo) {
		return errors.New("directory no longer matches preview")
	}
	return nil
}

func checkParents(root *os.Root, name string) error {
	if name == "." {
		return nil
	}
	var prefix string
	for part := range strings.SplitSeq(name, string(filepath.Separator)) {
		prefix = filepath.Join(prefix, part)
		info, err := root.Lstat(prefix)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return errors.New("parent is not a real directory")
		}
	}
	return nil
}

func readSnapshot(root *os.Root, name string, limit int64) ([]byte, revision, error) {
	info, err := root.Lstat(name)
	if err != nil {
		return nil, revision{}, err
	}
	if !info.Mode().IsRegular() {
		return nil, revision{}, errors.New(ReasonSpecial)
	}
	if info.Size() > limit {
		return nil, revision{}, errors.New(ReasonTooLarge)
	}
	file, err := openRegular(root, name)
	if err != nil {
		return nil, revision{}, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, revision{}, err
	}
	if !sameRevision(info, opened) {
		return nil, revision{}, errChanged
	}
	var reader io.Reader = file
	if limit < math.MaxInt64 {
		reader = io.LimitReader(file, limit+1)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, revision{}, err
	}
	current, err := root.Lstat(name)
	if err != nil {
		return nil, revision{}, err
	}
	if int64(len(data)) != info.Size() || !sameRevision(info, current) {
		return nil, revision{}, errChanged
	}
	return data, revision{info: info, digest: sha256.Sum256(data)}, nil
}

func Apply(ctx context.Context, preview *Preview) Result {
	if preview == nil || preview.rootInfo == nil {
		return Result{Err: errors.New("preview is required before conversion")}
	}
	var result Result
	var root *os.Root
	var rootErr error
	for _, file := range preview.files {
		if err := ctx.Err(); err != nil {
			result.Err = err
			break
		}
		switch file.entry.Action {
		case Skip:
			result.Skipped++
		case Unchanged:
			result.Unchanged++
		case Convert:
			if root == nil && rootErr == nil {
				root, rootErr = preview.openRoot()
				if root != nil {
					defer root.Close()
				}
			}
			err := rootErr
			if err == nil {
				err = preview.applyFile(ctx, root, file)
			}
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, FileError{Path: file.entry.Path, Relative: file.entry.Relative, Err: err})
			} else {
				result.Converted++
			}
		}
	}
	if result.Err == nil {
		result.Err = ctx.Err()
	}
	return result
}

func (preview *Preview) applyFile(ctx context.Context, root *os.Root, file plannedFile) error {
	parent, name := filepath.Split(file.entry.Relative)
	parent = filepath.Clean(parent)
	if err := checkParents(root, parent); err != nil {
		return err
	}
	directory, err := root.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer directory.Close()
	parentInfo, err := directory.Stat(".")
	if err != nil {
		return err
	}
	data, current, err := readSnapshot(directory, name, preview.limit)
	if err != nil {
		return err
	}
	if !sameRevision(file.revision.info, current.info) || file.revision.digest != current.digest {
		return errChanged
	}
	value, _, err := document.DetectTextEncoding(data)
	if err != nil {
		return err
	}
	output, err := convert(value, file.entry)
	if err != nil {
		return err
	}
	temp := ".vivapad-batch-" + rand.Text()
	outputFile, err := directory.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer directory.Remove(temp)
	if _, err = outputFile.Write(output); err == nil {
		err = outputFile.Chmod(current.info.Mode().Perm())
	}
	if err == nil {
		err = outputFile.Sync()
	}
	if closeErr := outputFile.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	_, latest, err := readSnapshot(directory, name, preview.limit)
	if err != nil {
		return err
	}
	if !sameRevision(current.info, latest.info) || current.digest != latest.digest {
		return errChanged
	}
	if err := preview.checkRoot(); err != nil {
		return err
	}
	if err := checkParents(root, parent); err != nil {
		return err
	}
	liveParent, err := root.Lstat(parent)
	if err != nil || !os.SameFile(parentInfo, liveParent) {
		return fmt.Errorf("parent directory changed since preview")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return directory.Rename(temp, name)
}
