//go:build !darwin && !windows && !linux

package printing

import "github.com/vivaui/viva/components"

func Execute(Job, float64, int, PageVariables,
	func(string, string, []components.CodeEditorRun) []components.CodeEditorRun) (int, error) {
	return 0, ErrUnsupported
}
