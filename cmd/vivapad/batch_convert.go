package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"vivapad/internal/batchconvert"
	"vivapad/internal/document"
)

const batchConvertCommand = "Batch Convert..."

var batchEncodings = [...]document.Encoding{document.UTF8, document.UTF8BOM, document.UTF16LE, document.UTF16BE, document.GB18030}

type batchInputs struct {
	directory, recursive bool
	root, glob           string
	encoding             int
	eol                  batchconvert.EOL
}

func (inputs batchInputs) targetEncoding() *document.Encoding {
	if inputs.encoding == 0 {
		return nil
	}
	return &batchEncodings[inputs.encoding-1]
}

func (inputs batchInputs) targetLineEnding() *document.LineEnding {
	switch inputs.eol {
	case batchconvert.LF:
		return new(document.LF)
	case batchconvert.CRLF:
		return new(document.CRLF)
	case batchconvert.CR:
		return new(document.CR)
	default:
		return nil
	}
}

type batchConvertDialog struct {
	inputs          batchInputs
	root, glob      *text.Editor
	preview         *batchconvert.Preview
	entries         []batchconvert.Entry
	tabs            []batchTab
	result          *batchconvert.Result
	err             error
	job             *batchJob
	offset, visible int
	selected        int
	focus           bool
}

type batchTab struct {
	doc      *document.Document
	editor   *text.Editor
	format   document.FileFormat
	revision uint64
}

type batchJob struct {
	cancel   context.CancelFunc
	reply    chan batchReply
	applying bool
}

type batchReply struct {
	preview *batchconvert.Preview
	result  *batchconvert.Result
	err     error
}

func (state *model) openBatchConvertDialog() {
	state.batchConvert = &batchConvertDialog{
		root: text.NewEditor(""), glob: text.NewEditor(""), selected: -1, focus: true,
	}
	state.menuOpen, state.menuOverflowOpen = -1, false
	state.calltip = nil
	state.invalidator.Invalidate()
}

func (state *model) closeBatchConvertDialog() bool {
	dialog := state.batchConvert
	if job := dialog.job; job != nil {
		if job.applying {
			return false
		}
		job.cancel()
	}
	state.batchConvert = nil
	state.requestNativeChooserFocus()
	return true
}

func (dialog *batchConvertDialog) resetPreview() {
	dialog.preview, dialog.entries, dialog.tabs, dialog.result, dialog.err = nil, nil, nil, nil, nil
	dialog.offset, dialog.selected = 0, -1
}

func (dialog *batchConvertDialog) readFields() {
	if root, glob := dialog.root.String(), dialog.glob.String(); dialog.inputs.root != root || dialog.inputs.glob != glob {
		dialog.inputs.root, dialog.inputs.glob = root, glob
		dialog.resetPreview()
	}
}

func (dialog *batchConvertDialog) textInput(id gfx.ID) *app.TextInput {
	if dialog.job != nil || !dialog.inputs.directory {
		return nil
	}
	switch id {
	case batchRootID:
		return &app.TextInput{Editor: dialog.root}
	case batchGlobID:
		return &app.TextInput{Editor: dialog.glob}
	default:
		return nil
	}
}

func (state *model) browseBatchDirectory() {
	dialog := state.batchConvert
	path, err := app.OpenDirectory(app.OpenDirectoryOptions{
		Title: state.tr("Batch Convert: Select Directory", "批量转换：选择目录"), InitialDirectory: dialog.inputs.root,
	})
	if err == nil {
		dialog.root = text.NewEditor(path)
		dialog.readFields()
	} else if !errors.Is(err, app.ErrCanceled) {
		dialog.err = err
	}
}

func (state *model) startBatchJob(applying bool, run func(context.Context) batchReply) {
	ctx, cancel := context.WithCancel(context.Background())
	job := &batchJob{cancel: cancel, applying: applying, reply: make(chan batchReply, 1)}
	state.batchConvert.job = job
	wake := state.invalidator.Invalidate
	go func() {
		job.reply <- run(ctx)
		wake()
	}()
}

func (state *model) consumeBatchJob() {
	dialog := state.batchConvert
	if dialog == nil || dialog.job == nil {
		return
	}
	select {
	case reply := <-dialog.job.reply:
		dialog.job.cancel()
		dialog.job = nil
		dialog.preview, dialog.result, dialog.err = reply.preview, reply.result, reply.err
		if reply.preview != nil {
			dialog.entries = reply.preview.Entries()
		}
		if reply.err == nil && reply.result != nil && reply.result.Err == nil {
			state.showToast(state.tr("Batch convert completed · ", "批量转换完成 · ") + state.batchCounts(dialog))
		}
	default:
	}
}

func (state *model) previewBatchConversion() {
	dialog := state.batchConvert
	dialog.readFields()
	dialog.resetPreview()
	inputs := dialog.inputs
	if inputs.directory {
		state.startBatchJob(false, func(ctx context.Context) batchReply {
			preview, err := batchconvert.Plan(ctx, batchconvert.Options{
				Root: inputs.root, Glob: inputs.glob, Recursive: inputs.recursive,
				TargetEncoding: inputs.targetEncoding(), TargetEOL: inputs.eol,
			})
			return batchReply{preview: preview, err: err}
		})
		return
	}
	for _, doc := range state.space.Documents {
		entry, tab := state.previewBatchTab(doc, inputs)
		dialog.entries = append(dialog.entries, entry)
		dialog.tabs = append(dialog.tabs, tab)
	}
}

func (state *model) previewBatchTab(doc *document.Document, inputs batchInputs) (batchconvert.Entry, batchTab) {
	entry := batchconvert.Entry{Path: doc.Path, Relative: doc.Name, Action: batchconvert.Skip}
	if state.isReadOnly(doc) {
		entry.Reason = state.tr("Read-only tab", "只读标签页")
		return entry, batchTab{}
	}
	format := doc.FileFormat()
	entry.HasFormat = true
	entry.SourceEncoding, entry.TargetEncoding = format.Encoding, format.Encoding
	entry.SourceEOL = batchconvert.EOL(format.EOL) + batchconvert.LF
	if doc.MixedLineEndings() {
		entry.SourceEOL = batchconvert.Mixed
	}
	entry.TargetEOL = entry.SourceEOL
	if target := inputs.targetEncoding(); target != nil {
		entry.TargetEncoding = *target
	}
	if inputs.eol != batchconvert.Keep {
		entry.TargetEOL = inputs.eol
	}
	if entry.SourceEncoding == entry.TargetEncoding && entry.SourceEOL == entry.TargetEOL {
		entry.Action = batchconvert.Unchanged
		return entry, batchTab{}
	}
	if err := doc.ValidateFormatConversion(entry.TargetEncoding, inputs.targetLineEnding()); err != nil {
		entry.Reason = err.Error()
		return entry, batchTab{}
	}
	entry.Action = batchconvert.Convert
	return entry, batchTab{doc: doc, editor: doc.Editor, format: format, revision: doc.Editor.TrackEdits()}
}

func (state *model) applyBatchConversion() {
	dialog := state.batchConvert
	dialog.readFields()
	if !dialog.canApply() {
		return
	}
	dialog.offset, dialog.selected = 0, -1
	if dialog.inputs.directory {
		preview := dialog.preview
		state.startBatchJob(true, func(ctx context.Context) batchReply {
			result := batchconvert.Apply(ctx, preview)
			return batchReply{result: &result, err: result.Err}
		})
		return
	}
	result := &batchconvert.Result{}
	for index, entry := range dialog.entries {
		switch entry.Action {
		case batchconvert.Skip:
			result.Skipped++
		case batchconvert.Unchanged:
			result.Unchanged++
		case batchconvert.Convert:
			tab := dialog.tabs[index]
			doc := tab.doc
			var err error
			if !slices.Contains(state.space.Documents, doc) || doc.Editor != tab.editor || doc.Path != entry.Path ||
				state.isReadOnly(doc) || doc.Editor.TrackEdits() != tab.revision || doc.FileFormat() != tab.format {
				err = errors.New(state.tr("Tab changed since preview; preview again", "标签页在预览后已变化，请重新预览"))
			} else {
				err = doc.ConvertFormat(entry.TargetEncoding, dialog.inputs.targetLineEnding())
			}
			if err != nil {
				result.Failed++
				result.Errors = append(result.Errors, batchconvert.FileError{Path: entry.Path, Relative: entry.Relative, Err: err})
			} else {
				result.Converted++
			}
		}
	}
	dialog.result = result
}

func (dialog *batchConvertDialog) canApply() bool {
	return dialog.job == nil && dialog.result == nil && len(dialog.entries) != 0
}

func (state *model) batchCounts(dialog *batchConvertDialog) string {
	if dialog.job != nil {
		if dialog.job.applying {
			return state.tr("Applying conversion…", "正在应用转换…")
		}
		return state.tr("Scanning…", "正在扫描…")
	}
	if dialog.result != nil {
		result := dialog.result
		return fmt.Sprintf(state.tr("%d converted  ·  %d unchanged  ·  %d skipped  ·  %d failed", "已转换 %d  ·  未变化 %d  ·  已跳过 %d  ·  失败 %d"), result.Converted, result.Unchanged, result.Skipped, result.Failed)
	}
	if dialog.entries == nil {
		return ""
	}
	var convert, unchanged, skip int
	for _, entry := range dialog.entries {
		switch entry.Action {
		case batchconvert.Convert:
			convert++
		case batchconvert.Unchanged:
			unchanged++
		case batchconvert.Skip:
			skip++
		}
	}
	return fmt.Sprintf(state.tr("%d will convert  ·  %d unchanged  ·  %d skipped", "待转换 %d  ·  无需转换 %d  ·  跳过 %d"), convert, unchanged, skip)
}

func (dialog *batchConvertDialog) rowCount() int {
	if dialog.result != nil {
		return len(dialog.result.Errors)
	}
	return len(dialog.entries)
}

func (dialog *batchConvertDialog) canPreview() bool {
	return dialog.job == nil && (!dialog.inputs.directory || strings.TrimSpace(dialog.inputs.root) != "")
}
