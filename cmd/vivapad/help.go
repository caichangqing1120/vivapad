package main

import (
	"encoding/hex"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

const (
	noHelp uint8 = iota
	aboutHelp
	debugHelp
)

const (
	aboutHelpCommand       = "About VivaPad"
	debugHelpCommand       = "Debug Info..."
	projectPageHelpCommand = "Project Page"
)

var buildVersionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:[-+][a-zA-Z0-9.+-]{1,64})?$`)
var goVersionPattern = regexp.MustCompile(`^go[0-9]+\.[0-9]+(?:\.[0-9]+|(?:beta|rc)[0-9]+)?$`)

type buildDetails struct {
	build, revision, platform, goVersion, viva string
}

func helpBuildDetails(info *debug.BuildInfo, goos, goarch string) buildDetails {
	details := buildDetails{"Development build", "Unavailable", goos + "/" + goarch, "Unavailable", "Unavailable"}
	if info == nil {
		return details
	}
	if buildVersionPattern.MatchString(info.Main.Version) {
		details.build = info.Main.Version
	}
	if goVersionPattern.MatchString(info.GoVersion) {
		details.goVersion = info.GoVersion
	}
	modified := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			if len(setting.Value) == 40 || len(setting.Value) == 64 {
				if _, err := hex.DecodeString(setting.Value); err == nil {
					details.revision = setting.Value
				}
			}
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	if modified && details.revision != "Unavailable" {
		details.revision += " (modified)"
	}
	for _, dep := range info.Deps {
		if dep == nil || dep.Path != "github.com/vivaui/viva" {
			continue
		}
		version := dep.Version
		if dep.Replace != nil {
			version = dep.Replace.Version
			if version == "" || version == "(devel)" {
				details.viva = "Local build"
			}
		}
		if buildVersionPattern.MatchString(version) {
			details.viva = version
		}
		break
	}
	return details
}

func (details buildDetails) rows(mode uint8) []string {
	rows := []string{"Build: " + details.build, "Revision: " + details.revision, "Platform: " + details.platform}
	if mode == debugHelp {
		rows = append(rows, "Go: "+details.goVersion, "Viva: "+details.viva)
	}
	return rows
}

func (details buildDetails) report() string {
	return "VivaPad\n" + strings.Join(details.rows(debugHelp), "\n")
}

func (state *model) openHelp(mode uint8) {
	info, _ := debug.ReadBuildInfo()
	state.helpDetails = helpBuildDetails(info, runtime.GOOS, runtime.GOARCH)
	state.helpMode, state.helpCopyStatus = mode, ""
}

func (state *model) closeHelp() {
	state.helpMode, state.helpCopyStatus = noHelp, ""
	state.requestEditorCommandFocus()
}

func (state *model) openHelpProjectPage(openURL func(string) error) {
	if err := openURL("https://github.com/caichangqing1120/vivapad"); err != nil {
		state.notice = "Could not open the project page"
	}
}

func (state *model) paintHelp(canvas *gfx.Canvas, size gfx.Size) {
	title := aboutHelpCommand
	if state.helpMode == debugHelp {
		title = "Debug Info"
	}
	rows := state.helpDetails.rows(state.helpMode)
	width := min(float32(536), size.Width-24)
	height := min(float32(114+len(rows)*25), size.Height-24)
	x, y := (size.Width-width)/2, (size.Height-height)/2
	canvas.PushFocusScope(helpDialogID)
	canvas.TrapFocus(helpDialogID)
	canvas.SetInitialFocus(helpCloseID)
	canvas.SetEscapeTarget(helpCloseID)
	canvas.PushLayer(4)
	canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, gfx.RGBA(0, 0, 0, 0.25))
	bounds := gfx.Rect{X: x, Y: y, Width: width, Height: height}
	canvas.RoundedBorder(bounds, 4, 1, palette.border, palette.white)
	canvas.HitWithOptions(helpDialogID, bounds, title, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	smallLabel(canvas, title, gfx.Rect{X: x + 18, Y: y + 12, Width: width - 36, Height: 26}, palette.text, 15)
	canvas.Fill(gfx.Rect{X: x + 18, Y: y + 43, Width: width - 36, Height: 1}, palette.border)
	rowHeight := min(float32(25), (height-104)/float32(len(rows)))
	for index, row := range rows {
		field := gfx.Rect{X: x + 18, Y: y + 51 + float32(index)*rowHeight, Width: width - 36, Height: rowHeight}
		smallLabel(canvas, canvas.FitLine(row, 12, gfx.WeightRegular, field.Width), field, palette.text, 12)
		canvas.HitWithOptions(helpRowScope.At(index), field, row, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	}
	buttonY := y + height - 44
	button := func(id gfx.ID, label string, left float32) bool {
		field := gfx.Rect{X: left, Y: buttonY, Width: 88, Height: 28}
		border := palette.border
		if canvas.FocusVisible() == id {
			border = palette.blue
		}
		canvas.RoundedBorder(field, 4, 1, border, palette.toolbar)
		canvas.LabelCenter(label, field, gfx.TextStyle{Size: 12, Color: palette.text})
		return hitButton(canvas, id, field, label)
	}
	if state.helpMode == debugHelp {
		if button(helpCopyID, "Copy", x+width-202) {
			state.helpCopyStatus = "Copied"
			if err := app.SetClipboardText(state.helpDetails.report()); err != nil {
				state.helpCopyStatus = "Could not copy debug info"
			}
		}
		smallLabel(canvas, state.helpCopyStatus, gfx.Rect{X: x + 18, Y: buttonY, Width: width - 228, Height: 28}, palette.muted, 11)
	}
	if button(helpCloseID, "Close", x+width-106) {
		state.closeHelp()
	}
	canvas.PopLayer()
	canvas.PopFocusScope()
}
