package main

import (
	"os"
	"os/exec"

	"vivapad/internal/document"
)

const newWindowCommand = "Open in New Window"
const newWindowFlag = "--new-window"

func launchVivaPadWindow(args ...string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	child := exec.Command(executable, args...)
	if err := child.Start(); err != nil {
		return err
	}
	go func() { _ = child.Wait() }()
	return nil
}

func (state *model) openNewWindow(doc *document.Document) {
	args := []string{newWindowFlag, "--"}
	if doc.Path != "" {
		if err := doc.CheckDiskPath(); err != nil {
			state.notice = "New window: " + err.Error()
			return
		}
		args = append(args, doc.Path)
	}
	if err := state.launchWindow(args...); err != nil {
		state.notice = "New window: " + err.Error()
		return
	}
	state.notice = "Opened a new VivaPad window"
}
