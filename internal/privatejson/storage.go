package privatejson

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func Load(path string, limit int64, dst any) error {
	if path == "" {
		return nil
	}
	bound, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !bound.Mode().IsRegular() || bound.Size() > limit {
		return fmt.Errorf("invalid configuration file size or type")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > limit || !os.SameFile(bound, info) {
		return fmt.Errorf("invalid configuration file size or type")
	}
	reader := &io.LimitedReader{R: file, N: limit + 1}
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF || reader.N == 0 {
		return fmt.Errorf("configuration has trailing data or exceeds its size limit")
	}
	return nil
}

func Save(path string, limit int64, value any) error {
	if path == "" {
		return errors.New("configuration storage unavailable")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if int64(len(data)) > limit {
		return fmt.Errorf("configuration exceeds its size limit")
	}
	return WriteAtomic(path, data)
}

func WriteAtomic(path string, data []byte) error {
	if path == "" {
		return errors.New("choose a path before saving")
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("configuration directory is not a regular directory")
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("configuration target is not a regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temp, err := os.CreateTemp(directory, ".vivapad-config-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if _, err := temp.Write(data); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return Replace(temp.Name(), path)
}
