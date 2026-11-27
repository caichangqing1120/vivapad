package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/update"
	"vivapad/internal/buildinfo"
	"vivapad/internal/document"
	"vivapad/internal/updater"
)

const checkUpdatesCommand = "Check for Updates..."

var (
	copyDiagnosticsID = tree.ID(page, "copy-diagnostics")
	checkUpdateID     = tree.ID(page, "check-update")
	downloadUpdateID  = tree.ID(page, "download-update")
	cancelUpdateID    = tree.ID(page, "cancel-update")
	installUpdateID   = tree.ID(page, "install-update")
	updateProgressID  = tree.ID(page, "update-progress")
	updateProjectID   = tree.ID(page, "update-project")
	updateIssuesID    = tree.ID(page, "update-issues")
	updateStatusID    = tree.ID(page, "update-status")
	updateAutoCheckID = tree.ID(page, "update-auto-check")
	updateBuildScope  = tree.Sub(page, tree.Name("update-build-info"))
)

type updateService interface {
	Check(context.Context) (*updater.Candidate, error)
	Prepare(context.Context, *updater.Candidate, func(updater.Progress)) (*updater.Prepared, error)
	Install(*updater.Prepared, func()) error
}

type updateUI struct {
	engine      updateService
	candidate   *updater.Candidate
	prepared    *updater.Prepared
	operation   *updateOperation
	err         error
	checked     bool
	copiedUntil time.Time
	buildRows   []string
	restarting  bool
	approved    map[*document.Document]bool
}

type updateResult struct {
	candidate *updater.Candidate
	prepared  *updater.Prepared
	err       error
}

type updateOperation struct {
	cancel          context.CancelFunc
	download, quiet bool
	canceled        bool
	mutex           sync.Mutex
	progress        updater.Progress
	result          *updateResult
	abandoned       bool
}

func (state *model) openUpdatePreferences() {
	state.preferencesOpen, state.preferencesPage = true, preferencesAboutPage
	state.preferencesNotice = ""
	state.openPrintPreferenceFields()
	state.invalidator.Invalidate()
}

func (state *model) startUpdateAtLaunch() {
	state.beginUpdate(false, true)
}

func (state *model) startUpdate(download bool) {
	state.beginUpdate(download, false)
}

func (state *model) beginUpdate(download, quiet bool) {
	panel := &state.updates
	if panel.operation != nil || panel.restarting || (download && (panel.candidate == nil || panel.prepared != nil)) {
		return
	}
	if panel.engine == nil {
		configDir := ""
		if state.preferencesPath != "" {
			configDir = filepath.Dir(state.preferencesPath)
		}
		engine, err := updater.New(configDir, buildinfo.Version, buildinfo.Channel, buildinfo.FeedURL, buildinfo.PublicKey, nil)
		if err != nil {
			if !quiet {
				panel.err = err
			}
			return
		}
		panel.engine = engine
	}
	panel.err = nil
	if !download {
		panel.prepared.Close()
		panel.candidate, panel.prepared, panel.checked = nil, nil, false
	}
	ctx, cancel := context.WithCancel(context.Background())
	job := &updateOperation{cancel: cancel, download: download, quiet: quiet}
	panel.operation = job
	engine, candidate := panel.engine, panel.candidate
	go func() {
		var result updateResult
		if download {
			result.prepared, result.err = engine.Prepare(ctx, candidate, func(progress updater.Progress) {
				job.mutex.Lock()
				job.progress = progress
				job.mutex.Unlock()
				state.invalidator.Invalidate()
			})
		} else {
			result.candidate, result.err = engine.Check(ctx)
		}
		if ctx.Err() != nil {
			result.prepared.Close()
			result = updateResult{err: ctx.Err()}
		}
		job.mutex.Lock()
		if job.abandoned {
			result.prepared.Close()
		} else {
			job.result = &result
		}
		job.mutex.Unlock()
		state.invalidator.Invalidate()
	}()
}

func (state *model) consumeUpdate() {
	panel := &state.updates
	job := panel.operation
	if job == nil {
		return
	}
	job.mutex.Lock()
	result := job.result
	job.mutex.Unlock()
	if result == nil {
		return
	}
	job.cancel()
	panel.operation = nil
	if job.canceled {
		result.prepared.Close()
		panel.err = context.Canceled
		return
	}
	panel.err = result.err
	if job.download {
		panel.prepared = result.prepared
		return
	}
	panel.candidate, panel.checked = result.candidate, result.err == nil
	if job.quiet && result.candidate != nil {
		state.notice = fmt.Sprintf(state.tr("VivaPad %s is available (Help > Check for Updates)", "VivaPad %s 可用（帮助 > 检查更新）"), result.candidate.Version())
	}
}

func (state *model) closeUpdates() {
	panel := &state.updates
	if job := panel.operation; job != nil {
		job.cancel()
		job.mutex.Lock()
		job.abandoned = true
		if job.result != nil {
			job.result.prepared.Close()
		}
		job.mutex.Unlock()
		panel.operation = nil
	}
	panel.prepared.Close()
	panel.prepared = nil
}

func (state *model) requestUpdateInstall() {
	panel := &state.updates
	if panel.prepared == nil || panel.engine == nil || state.closeWindow == nil || state.pendingClose >= 0 || panel.restarting {
		return
	}
	panel.restarting = true
	panel.approved = make(map[*document.Document]bool)
	state.closePreferences()
	state.closeWindow()
	if panel.restarting && state.allowClose == nil && !state.closingAccepted {
		panel.restarting, panel.approved = false, nil
		state.openUpdatePreferences()
	}
}

func (state *model) finishUpdateInstall(allow func()) {
	panel := &state.updates
	err := panel.engine.Install(panel.prepared, func() {
		state.persistSession()
		allow()
	})
	panel.restarting, panel.approved = false, nil
	if err != nil {
		state.clearCloseSnapshot()
		panel.err = err
		state.openUpdatePreferences()
	}
}

func diagnosticRows() []string {
	info, _ := debug.ReadBuildInfo()
	details := helpBuildDetails(info, runtime.GOOS, runtime.GOARCH)
	commit := buildinfo.Commit
	if commit == "unknown" {
		commit = details.revision
	}
	return []string{"Version: " + buildinfo.Version + " (" + buildinfo.Channel + ")", "Platform: " + details.platform, "Build Commit: " + commit, "Go: " + runtime.Version(), "Viva: " + details.viva}
}

func (state *model) paintUpdatePreferences(canvas *gfx.Canvas, x, y, width float32) {
	panel, palette := &state.updates, state.palette()
	if panel.buildRows == nil {
		panel.buildRows = diagnosticRows()
	}
	left := (width - 9) * .43
	card := func(bounds gfx.Rect) { canvas.RoundedBorder(bounds, 7, 1, palette.border, palette.dock) }
	card(gfx.Rect{X: x, Y: y + 59, Width: left, Height: 171})
	line := func(value string, top float32, index int) {
		bounds := gfx.Rect{X: x + 10, Y: y + top, Width: left - 20, Height: 19}
		state.interfaceLabel(canvas, value, bounds, palette.muted, 11)
		canvas.HitWithOptions(updateBuildScope.At(index), bounds, value, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	}
	state.interfaceLabel(canvas, state.tr("Build Information", "构建信息"), gfx.Rect{X: x + 10, Y: y + 64, Width: left - 20, Height: 23}, palette.text, 14)
	for index, value := range panel.buildRows {
		if state.chineseUI {
			value = strings.NewReplacer("Version:", "版本：", "Platform:", "平台：", "Build Commit:", "构建提交：").Replace(value)
		}
		line(value, 89+float32(index)*18, index)
	}
	copyLabel := state.tr("Copy Diagnostics", "复制诊断信息")
	if time.Now().Before(panel.copiedUntil) {
		copyLabel = state.tr("Copied", "已复制")
	}
	if state.settingsButton(canvas, copyDiagnosticsID, gfx.Rect{X: x + 10, Y: y + 190, Width: left - 20, Height: 28}, copyLabel, false) {
		if err := app.SetClipboardText("VivaPad\n" + strings.Join(panel.buildRows, "\n")); err != nil {
			state.preferencesNotice = state.tr("Could not copy diagnostics", "无法复制诊断信息")
		} else {
			state.preferencesNotice = ""
			panel.copiedUntil = time.Now().Add(1500 * time.Millisecond)
			time.AfterFunc(1500*time.Millisecond, state.invalidator.Invalidate)
		}
	}
	card(gfx.Rect{X: x + left + 9, Y: y + 59, Width: width - left - 9, Height: 171})
	state.paintUpdateControls(canvas, x+left+19, y, width-left-29)
	card(gfx.Rect{X: x, Y: y + 238, Width: width, Height: 41})
	state.interfaceLabel(canvas, state.tr("Project Links", "项目链接"), gfx.Rect{X: x + 10, Y: y + 242, Width: 100, Height: 32}, palette.text, 12)
	if state.settingsButton(canvas, updateProjectID, gfx.Rect{X: x + 116, Y: y + 245, Width: 121, Height: 26}, state.tr("Project Home", "项目主页"), false) {
		state.openUpdateLink(projectPageURL)
	}
	if state.settingsButton(canvas, updateIssuesID, gfx.Rect{X: x + 244, Y: y + 245, Width: 70, Height: 26}, state.tr("Issues", "问题反馈"), false) {
		state.openUpdateLink(projectPageURL + "/issues")
	}
}

func (state *model) openUpdateLink(target string) {
	state.preferencesNotice = ""
	if err := app.OpenURL(target); err != nil {
		state.preferencesNotice = state.tr("Could not open the project link", "无法打开项目链接")
	}
}

func (state *model) paintUpdateControls(canvas *gfx.Canvas, x, y, width float32) {
	panel, palette := &state.updates, state.palette()
	row := func(top, height float32) gfx.Rect { return gfx.Rect{X: x, Y: y + top, Width: width, Height: height} }
	state.interfaceLabel(canvas, state.tr("Software Update", "软件更新"), row(64, 23), palette.text, 14)
	state.generalCheckbox(canvas, updateAutoCheckID, row(88, 25), state.tr("Check for updates on startup", "启动时检查更新"), &state.autoCheckUpdates)
	status := state.updateStatus()
	state.interfaceLabel(canvas, status, row(116, 22), palette.text, 11)
	canvas.HitWithOptions(updateStatusID, row(116, 22), status, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	job := panel.operation
	if job != nil && job.download {
		job.mutex.Lock()
		progress := job.progress
		job.mutex.Unlock()
		bar := row(161, 5)
		canvas.Fill(bar, palette.border)
		if progress.Total > 0 {
			bar.Width *= float32(progress.Done) / float32(progress.Total)
			canvas.Fill(bar, palette.accent)
		}
		state.interfaceLabel(canvas, fmt.Sprintf("%s / %s", updateSize(progress.Done), updateSize(progress.Total)), row(138, 21), palette.muted, 10)
		canvas.HitWithOptions(updateProgressID, row(138, 29), fmt.Sprintf("%d / %d", progress.Done, progress.Total), gfx.HitOptions{Role: gfx.RoleProgressBar, SkipFocus: true})
	} else if candidate := panel.candidate; candidate != nil {
		badge := row(143, 25)
		canvas.RoundedBorder(badge, 5, 1, palette.border, palette.selectedRow)
		text := fmt.Sprintf(state.tr("Full package %s", "全量包 %s"), updateSize(candidate.FullSize()))
		if candidate.PatchFrom() != "" {
			text = fmt.Sprintf(state.tr("Delta %s vs Full %s", "增量 %s · 全量 %s"), updateSize(candidate.PatchSize()), updateSize(candidate.FullSize()))
		}
		state.interfaceLabel(canvas, text, badge.Inset(gfx.Insets{Left: 6, Right: 6}), palette.accent, 10)
	}
	button := row(190, 28)
	if job == nil && panel.candidate != nil && !errors.Is(panel.err, update.ErrManagedExternally) {
		secondary := button
		secondary.Width = min(112, (width-6)*.44)
		secondary.X += width - secondary.Width
		button.Width -= secondary.Width + 6
		if state.settingsButton(canvas, checkUpdateID, secondary, state.tr("Check Again", "重新检查"), false) {
			state.startUpdate(false)
			return
		}
	}
	switch {
	case job != nil:
		if state.settingsButton(canvas, cancelUpdateID, button, state.tr("Cancel", "取消"), false) {
			job.canceled = true
			job.cancel()
		}
	case panel.prepared != nil && !errors.Is(panel.err, update.ErrManagedExternally):
		if state.settingsButton(canvas, installUpdateID, button, state.tr("Install & Restart", "安装并重启"), true) {
			state.requestUpdateInstall()
		}
	case panel.candidate != nil && !errors.Is(panel.err, update.ErrManagedExternally):
		if state.settingsButton(canvas, downloadUpdateID, button, state.tr("Download Update", "下载更新"), true) {
			state.startUpdate(true)
		}
	default:
		if state.settingsButton(canvas, checkUpdateID, button, state.tr("Check for Updates", "立即检查更新"), true) {
			state.startUpdate(false)
		}
	}
}

func (state *model) updateStatus() string {
	panel := &state.updates
	if job := panel.operation; job != nil {
		if !job.download {
			return state.tr("Checking…", "正在检查…")
		}
		job.mutex.Lock()
		progress := job.progress
		job.mutex.Unlock()
		if progress.Applying {
			return state.tr("Applying delta patch…", "正在应用增量补丁…")
		}
		if progress.Full {
			return state.tr("Downloading full package…", "正在下载全量包…")
		}
		return state.tr("Downloading update…", "正在下载更新…")
	}
	switch {
	case errors.Is(panel.err, context.Canceled):
		return state.tr("Update canceled", "已取消更新")
	case errors.Is(panel.err, updater.ErrNotConfigured):
		return state.tr("No signed feed for this build", "此构建未配置签名更新源")
	case errors.Is(panel.err, update.ErrSignature):
		return state.tr("Update signature rejected", "更新签名验证失败")
	case errors.Is(panel.err, update.ErrManagedExternally):
		return state.tr("Use your system package manager", "请使用系统包管理器更新")
	case panel.err != nil:
		return state.tr("Update failed: ", "更新失败：") + panel.err.Error()
	case panel.prepared != nil:
		if panel.prepared.FellBack() {
			return state.tr("Full package ready to install", "全量包已准备就绪")
		}
		return state.tr("Ready to install", "已准备就绪")
	case panel.candidate != nil:
		return fmt.Sprintf(state.tr("Version %s is available", "版本 %s 可用"), panel.candidate.Version())
	case panel.checked:
		return state.tr("You’re up to date", "已是最新版本")
	default:
		return state.tr("Not checked yet", "尚未检查更新")
	}
}

func updateSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1<<20 {
		return fmt.Sprintf("%.1f KiB", float64(size)/1024)
	}
	return fmt.Sprintf("%.1f MiB", float64(size)/(1<<20))
}
