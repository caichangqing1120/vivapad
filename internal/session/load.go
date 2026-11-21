package session

import (
	"errors"
	"os"
)

func openRegularSession(path string) (*os.File, int64, error) {
	file, err := openSessionFile(path)
	if err != nil {
		return nil, 0, err
	}
	info, err := file.Stat()
	if err == nil && !info.Mode().IsRegular() {
		err = errors.New("session source is not a regular file")
	}
	if err != nil {
		file.Close()
		return nil, 0, err
	}
	return file, info.Size(), nil
}
