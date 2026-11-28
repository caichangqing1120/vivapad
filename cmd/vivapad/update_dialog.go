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

func (state *model) paintUpdatePreferences(canvas *gfx.Canvas, body gfx.Rect) {
	panel, palette := &state.updates, state.palette()
	if panel.buildRows == nil {
		panel.buildRows = diagnosticRows()
	}
	cardHeight := float32(184)
	build := state.settingsCard(canvas, gfx.Rect{X: body.X, Y: body.Y, Width: (body.Width - settingsCardGap) * .44}, cardHeight)
	state.settingsLabel(canvas, state.tr("Build Information", "构建信息"), build.row(canvas, 0, 34), palette.text, 13, gfx.WeightSemibold)
	buildY := build.bounds.Y + 36
	buildStyle := gfx.TextStyle{Color: palette.muted, Size: state.interfaceSize(11)}
	for index, value := range panel.buildRows {
		if state.chineseUI {
			value = strings.NewReplacer("Version:", "版本：", "Platform:", "平台：", "Build Commit:", "构建提交：").Replace(value)
		}
		bounds := gfx.Rect{X: build.bounds.X + 12, Y: buildY, Width: build.bounds.Width - 24, Height: 18}
		id := updateBuildScope.At(index)
		if index == 0 {
			lineBounds := bounds
			display := value
			if state.chineseUI {
				display = strings.ReplaceAll(display, "(development)", "（开发版）")
			}
			lines := canvas.WrapLines(nil, display, buildStyle.Size, buildStyle.Weight, bounds.Width, 2)
			for _, line := range lines {
				canvas.Label(line, lineBounds, buildStyle)
				lineBounds.Y += lineBounds.Height
			}
			bounds.Height *= float32(len(lines))
			canvas.HoverHint(page, bounds, value, id)
		} else {
			canvas.LabelEllipsis(value, bounds, buildStyle, id)
		}
		canvas.HitWithOptions(id, bounds, value, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
		buildY += bounds.Height
	}
	const actionsTop = 146
	copyLabel := state.tr("Copy Diagnostics", "复制诊断信息")
	if time.Now().Before(panel.copiedUntil) {
		copyLabel = state.tr("Copied", "已复制")
	}
	copyBounds := settingsControl(build.row(canvas, actionsTop, cardHeight-actionsTop), state.settingsButtonWidth(canvas, state.tr("Copy Diagnostics", "复制诊断信息")), 26)
	if state.settingsButton(canvas, copyDiagnosticsID, copyBounds, copyLabel, false) {
		if err := app.SetClipboardText("VivaPad\n" + strings.Join(panel.buildRows, "\n")); err != nil {
			state.preferencesNotice = state.tr("Could not copy diagnostics", "无法复制诊断信息")
		} else {
			state.preferencesNotice = ""
			panel.copiedUntil = time.Now().Add(1500 * time.Millisecond)
			time.AfterFunc(1500*time.Millisecond, state.invalidator.Invalidate)
		}
	}
	updateBounds := body
	updateBounds.X += build.bounds.Width + settingsCardGap
	updateBounds.Width -= build.bounds.Width + settingsCardGap
	state.paintUpdateControls(canvas, state.settingsCard(canvas, updateBounds, cardHeight), actionsTop)
	body.Y += cardHeight + settingsCardGap
	links := state.settingsCard(canvas, body, 42).row(canvas, 0, 42)
	homeLabel, issuesLabel := state.tr("Project Home", "项目主页"), state.tr("Issues", "问题反馈")
	issues := settingsControl(links, state.settingsButtonWidth(canvas, issuesLabel), 26)
	linkSpace := links
	linkSpace.Width -= issues.Width + 6
	home := settingsControl(linkSpace, state.settingsButtonWidth(canvas, homeLabel), 26)
	state.settingsRowLabel(canvas, state.tr("Project Links", "项目链接"), links, home)
	if state.settingsButton(canvas, updateProjectID, home, homeLabel, false) {
		state.openUpdateLink(projectPageURL)
	}
	if state.settingsButton(canvas, updateIssuesID, issues, issuesLabel, false) {
		state.openUpdateLink(projectPageURL + "/issues")
	}
}

func (state *model) openUpdateLink(target string) {
	state.preferencesNotice = ""
	if err := app.OpenURL(target); err != nil {
		state.preferencesNotice = state.tr("Could not open the project link", "无法打开项目链接")
	}
}

func (state *model) paintUpdateControls(canvas *gfx.Canvas, card settingsCard, actionsTop float32) {
	panel, palette := &state.updates, state.palette()
	row := func(top, height float32) gfx.Rect {
		return gfx.Rect{X: card.bounds.X + 12, Y: card.bounds.Y + top, Width: card.bounds.Width - 24, Height: height}
	}
	state.settingsLabel(canvas, state.tr("Software Update", "软件更新"), row(0, 34), palette.text, 13, gfx.WeightSemibold)
	if state.paintPreferenceToggle(canvas, updateAutoCheckID, card.row(canvas, 34, 36), state.tr("Check on startup", "启动时检查更新"),
		state.tr("Check for updates on startup", "启动时检查更新"), state.autoCheckUpdates) {
		state.autoCheckUpdates = !state.autoCheckUpdates
		state.saveGeneralPreferences()
	}
	status := state.updateStatus()
	statusBounds := row(76, 38)
	style := gfx.TextStyle{Color: palette.text, Size: state.interfaceSize(11)}
	lines := canvas.WrapLines(nil, status, style.Size, style.Weight, statusBounds.Width, 2)
	for index, line := range lines {
		canvas.Label(line, row(76+float32(index)*17, 17), style)
	}
	canvas.HoverHint(page, statusBounds, status, updateStatusID)
	canvas.HitWithOptions(updateStatusID, statusBounds, status, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	job := panel.operation
	if job != nil && job.download {
		job.mutex.Lock()
		progress := job.progress
		job.mutex.Unlock()
		bar := row(133, 5)
		canvas.Rounded(bar, 2.5, palette.border)
		if progress.Total > 0 {
			bar.Width *= max(0, min(1, float32(progress.Done)/float32(progress.Total)))
			canvas.Rounded(bar, 2.5, palette.accent)
		}
		state.interfaceLabel(canvas, fmt.Sprintf("%s / %s", updateSize(progress.Done), updateSize(progress.Total)), row(113, 18), palette.muted, 10)
		canvas.HitWithOptions(updateProgressID, row(113, 28), fmt.Sprintf("%d / %d", progress.Done, progress.Total), gfx.HitOptions{Role: gfx.RoleProgressBar, SkipFocus: true})
	} else if candidate := panel.candidate; candidate != nil {
		text := fmt.Sprintf(state.tr("Full package %s", "全量包 %s"), updateSize(candidate.FullSize()))
		if candidate.PatchFrom() != "" {
			text = fmt.Sprintf(state.tr("Delta %s vs Full %s", "增量 %s · 全量 %s"), updateSize(candidate.PatchSize()), updateSize(candidate.FullSize()))
		}
		badge := row(114, 24)
		badge.Width = min(badge.Width, canvas.TextWidth(text, state.interfaceSize(10))+16)
		canvas.Rounded(badge, 12, palette.selectedRow)
		state.interfaceLabel(canvas, text, badge.Inset(gfx.Insets{Left: 8, Right: 8}), palette.accent, 10)
	}
	actions := card.row(canvas, actionsTop, card.bounds.Height-actionsTop)
	buttonID, buttonLabel, emphasis := checkUpdateID, state.tr("Check for Updates", "立即检查更新"), true
	managed := errors.Is(panel.err, update.ErrManagedExternally)
	switch {
	case job != nil:
		buttonID, buttonLabel, emphasis = cancelUpdateID, state.tr("Cancel", "取消"), false
	case panel.prepared != nil && !managed:
		buttonID, buttonLabel = installUpdateID, state.tr("Install & Restart", "安装并重启")
	case panel.candidate != nil && !managed:
		buttonID, buttonLabel = downloadUpdateID, state.tr("Download Update", "下载更新")
	}
	button := settingsControl(actions, state.settingsButtonWidth(canvas, buttonLabel), 26)
	if job == nil && panel.candidate != nil && !managed {
		secondaryLabel := state.tr("Check Again", "重新检查")
		secondaryRow := actions
		secondaryRow.Width -= button.Width + 6
		secondary := settingsControl(secondaryRow, state.settingsButtonWidth(canvas, secondaryLabel), 26)
		if state.settingsButton(canvas, checkUpdateID, secondary, secondaryLabel, false) {
			state.startUpdate(false)
			return
		}
	}
	if !state.settingsButton(canvas, buttonID, button, buttonLabel, emphasis) {
		return
	}
	switch buttonID {
	case cancelUpdateID:
		job.canceled = true
		job.cancel()
	case installUpdateID:
		state.requestUpdateInstall()
	case downloadUpdateID:
		state.startUpdate(true)
	case checkUpdateID:
		state.startUpdate(false)
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
