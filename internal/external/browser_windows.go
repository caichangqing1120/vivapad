package external

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func legacyInternetExplorer() bool { return windows.RtlGetVersion().BuildNumber < 22000 }

func lookupWindowsBrowser(executable string) (string, error) {
	const appPaths = `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\`
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		for _, view := range []uint32{0, registry.WOW64_64KEY, registry.WOW64_32KEY} {
			key, err := registry.OpenKey(root, appPaths+executable, registry.QUERY_VALUE|view)
			if err != nil {
				continue
			}
			path, kind, err := key.GetStringValue("")
			_ = key.Close()
			if err == nil && kind == registry.EXPAND_SZ {
				path, err = registry.ExpandString(path)
			}
			path = strings.Trim(strings.TrimSpace(path), `"`)
			if err != nil || !filepath.IsAbs(path) || !strings.EqualFold(filepath.Base(path), executable) {
				continue
			}
			if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
				return path, nil
			}
		}
	}
	return "", errors.New("browser executable is not registered")
}
