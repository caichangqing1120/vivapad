package main

import (
	"log"
	"os"
	"runtime"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"vivapad/internal/printing"
)

func init() {
	// Go runs init on the startup thread; Cocoa needs main to stay on it.
	if runtime.GOOS == "darwin" {
		runtime.LockOSThread()
	}
}

func main() {
	paths, help := parseLaunchArguments(os.Args[1:])
	if help {
		if err := writeLaunchUsage(); err != nil {
			log.Fatal(err)
		}
		return
	}
	if runtime.GOOS != "darwin" {
		runtime.LockOSThread()
	}
	var canvas gfx.Canvas
	sessionPath, err := defaultSessionPath()
	if err != nil {
		log.Printf("session storage unavailable: %v", err)
	}
	state := newStartupModel(paths, sessionPath)
	config := newNativeConfig(state)
	if err := app.RunApplication(func(application *app.Application) error {
		window, err := application.OpenWindow(config, &canvas)
		if err != nil {
			return err
		}
		state.closeWindow = func() { config.OnCloseRequest(window.Close) }
		state.requestFullscreen = window.SetFullscreen
		state.requestAlwaysOnTop = window.SetAlwaysOnTop
		state.postBrowserResult = application.Post
		state.saveSessionPicker = app.SaveFileWithCheckbox
		state.printJob = func(job printing.Job) (int, error) {
			doc := state.paneDocument(state.activePane)
			now := state.clock()
			variables := printing.PageVariables{
				ShortDate: now.Format("1/2/06"), LongDate: now.Format("Monday, January 2, 2006"), Time: now.Format("3:04 PM"),
				NPP: state.runVariables(doc),
			}
			return printing.Execute(job, float64(state.paneFontSize(state.activePane)), state.tabWidth, variables, state.syntax.Highlight)
		}
		application.OnOpenFile(state.queueSystemDocument)
		return nil
	}); err != nil {
		log.Fatal(err)
	}
}

const launchUsage = `Usage: vivapad [--] [file ...]
       vivapad -h | --help

Open files in VivaPad. With no files, reopen the previous session if available.
Use -- before a filename such as --help to open it literally.
`

func parseLaunchArguments(args []string) (paths []string, help bool) {
	if len(args) != 0 {
		switch args[0] {
		case "-h", "--help":
			return nil, true
		case "--":
			return args[1:], false
		}
	}
	return args, false
}

func newNativeConfig(state *model) app.Config {
	state.requestLinuxComposition = runtime.GOOS == "linux"
	return newConfig(state)
}
