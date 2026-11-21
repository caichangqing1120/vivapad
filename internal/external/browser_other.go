//go:build !windows

package external

import "errors"

func legacyInternetExplorer() bool { return false }

func lookupWindowsBrowser(string) (string, error) { return "", errors.New("requires Windows") }
