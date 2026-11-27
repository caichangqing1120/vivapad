package updater

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/vivaui/viva/update"
)

func stageWindowsZIP(ctx context.Context, path, dir string) (string, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer archive.Close()
	if len(archive.File) != 1 || archive.File[0].Name != "vivapad.exe" || !archive.File[0].Mode().IsRegular() {
		return "", fmt.Errorf("%w: expected only vivapad.exe in the portable archive", update.ErrUnsafeArchive)
	}
	program := archive.File[0]
	if program.UncompressedSize64 > uint64(update.DefaultMaxAssetBytes) {
		return "", update.ErrTooLarge
	}
	source, err := program.Open()
	if err != nil {
		return "", err
	}
	defer source.Close()
	staging, err := os.MkdirTemp(dir, "staging-")
	if err != nil {
		return "", err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(staging)
		}
	}()
	dest, err := os.OpenFile(filepath.Join(staging, "vivapad.exe"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0700)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(dest, updateContextReader{ctx, source})
	if err := errors.Join(copyErr, dest.Close(), ctx.Err()); err != nil {
		return "", err
	}
	complete = true
	return staging, nil
}

type updateContextReader struct {
	ctx context.Context
	io.Reader
}

func (reader updateContextReader) Read(buffer []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.Reader.Read(buffer)
}
