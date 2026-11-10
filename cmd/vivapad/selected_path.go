package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"unicode"

	"vivapad/internal/completion"
)

var selectedEnvironmentToken = regexp.MustCompile(`%[A-Za-z_][A-Za-z0-9_]*%|\$\{[A-Za-z_][A-Za-z0-9_]*\}|\$[A-Za-z_][A-Za-z0-9_]*`)

func (state *model) resolveSelectedPath(allowDirectory bool) (string, error) {
	doc := state.paneDocument(state.activePane)
	value := doc.Editor.String()
	if selected := state.multiSelections[state.activePane]; selected.active && selected.doc == doc && selected.value == value && len(selected.ranges) > 1 {
		return "", fmt.Errorf("a single selected path is required")
	}
	if selected := state.rectangles[state.activePane]; selected.active && selected.doc == doc && selected.value == value {
		return "", fmt.Errorf("a single selected path is required")
	}
	selection := doc.Editor.Selection()
	raw := ""
	if selection.Start < selection.End {
		raw = string([]rune(value)[selection.Start:selection.End])
	} else {
		raw, _, _ = completion.PathTokenAtCaret(value, doc.Editor.Caret())
	}
	raw = strings.TrimSpace(raw)
	if len(raw) >= 2 && (raw[0] == '"' && raw[len(raw)-1] == '"' || raw[0] == '\'' && raw[len(raw)-1] == '\'') {
		raw = raw[1 : len(raw)-1]
	}
	if raw == "" || len(raw) > 4096 || strings.ContainsAny(raw, "\x00\r\n") {
		return "", fmt.Errorf("select one valid file path")
	}
	missing := ""
	raw = selectedEnvironmentToken.ReplaceAllStringFunc(raw, func(token string) string {
		key := token[1:]
		if token[0] == '%' || strings.HasPrefix(token, "${") {
			key = token[1 : len(token)-1]
			if token[0] == '$' {
				key = token[2 : len(token)-1]
			}
		}
		if value, ok := os.LookupEnv(key); ok {
			return value
		}
		missing = token
		return token
	})
	if missing != "" || strings.ContainsAny(raw, "\x00\r\n") {
		return "", fmt.Errorf("unknown or invalid path variable %s", missing)
	}
	if raw == "~" || strings.HasPrefix(raw, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		raw = filepath.Join(home, strings.TrimPrefix(raw, "~/"))
	}
	if runtime.GOOS != "windows" && len(raw) >= 3 && unicode.IsLetter(rune(raw[0])) && raw[1] == ':' && (raw[2] == '\\' || raw[2] == '/') {
		return "", fmt.Errorf("Windows paths require Windows")
	}
	if !filepath.IsAbs(raw) {
		base := filepath.Dir(doc.Path)
		if doc.Path == "" {
			var err error
			base, err = os.Getwd()
			if err != nil {
				return "", err
			}
		}
		raw = filepath.Join(base, raw)
	}
	path := filepath.Clean(raw)
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() && (!allowDirectory || !info.IsDir()) {
		return "", fmt.Errorf("selected path is not a regular file: %s", path)
	}
	return path, nil
}

func (state *model) openSelectedPath(reveal bool) {
	path, err := state.resolveSelectedPath(reveal)
	if err != nil {
		state.notice = "Open selected path: " + err.Error()
		return
	}
	if reveal {
		err = state.revealSelectedPath(path)
	} else {
		err = state.launchSelectedFile(path)
	}
	if err != nil {
		state.notice = "Open selected path: " + err.Error()
		return
	}
	if reveal {
		state.notice = "Opened selected path in file manager"
	} else {
		state.notice = "Opened selected file in new instance"
	}
}
