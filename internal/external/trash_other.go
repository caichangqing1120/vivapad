//go:build !darwin && !windows && !linux

package external

func trashNative(string) error { return ErrTrashUnsupported }
