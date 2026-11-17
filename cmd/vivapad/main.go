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
	if runtime.GOOS != "darwin" {
		runtime.LockOSThread()
	}
	var canvas gfx.Canvas
	sessionPath, err := defaultSessionPath()
	if err != nil {
		log.Printf("session storage unavailable: %v", err)
	}
	state := newStartupModel(os.Args[1:], sessionPath)
	config := newConfig(state)
	if err := app.RunApplication(func(application *app.Application) error {
		window, err := application.OpenWindow(config, &canvas)
		if err != nil {
			return err
		}
		state.closeWindow = func() { config.OnCloseRequest(window.Close) }
		state.requestFullscreen = window.SetFullscreen
		state.requestAlwaysOnTop = window.SetAlwaysOnTop
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
		return nil
	}); err != nil {
		log.Fatal(err)
	}
}
