package changehistory

import (
	"fmt"
	"unsafe"

	"github.com/vivaui/viva/text"
)

const maxConsumerMetadataBytes = 16 << 20

// Consumer binds one editor's published history to sparse change metadata.
type Consumer struct {
	editor *text.Editor

	state   consumerState
	tracker *Tracker

	metadataBytes int
	revision      uint64
	budget        int

	unavailable bool
	reason      error
	frames      uint64
	steps       uint64

	read consumerFrames
}

type consumerState struct {
	cursor  text.HistoryCursor
	bytes   int
	current int
}

type consumerFrames func(text.HistoryCursor) ([]text.HistoryFrame, text.HistoryCursor, bool)

func NewConsumer(editor *text.Editor) (*Consumer, error) {
	if editor == nil {
		return nil, fmt.Errorf("change-history consumer requires an editor")
	}
	cursor, bootstrapBytes, ok := editor.TrackHistory()
	if !ok {
		return nil, fmt.Errorf("change-history producer refused to track the editor")
	}
	tracker, err := New(bootstrapBytes)
	if err != nil {
		return nil, err
	}
	return &Consumer{
		editor: editor, tracker: tracker,
		state:         consumerState{cursor: cursor, bytes: bootstrapBytes},
		metadataBytes: int(estimateTrackerBytes(tracker)),
		budget:        maxConsumerMetadataBytes,
	}, nil
}

// Sync validates a whole producer delivery and publishes the candidate atomically.
func (consumer *Consumer) Sync() error {
	if consumer == nil {
		return fmt.Errorf("nil change-history consumer")
	}
	if consumer.unavailable {
		return consumer.reason
	}
	if consumer.editor == nil || consumer.tracker == nil {
		return consumer.invalidate(fmt.Errorf("consumer lost its pinned editor or metadata"))
	}
	frames, latest, ok := consumer.readFrames(consumer.state.cursor)
	if !ok {
		return consumer.invalidate(fmt.Errorf("history producer refused the pinned cursor (lost checkpoint or changed source/generation)"))
	}
	if len(frames) == 0 {
		if latest != consumer.state.cursor {
			return consumer.invalidate(fmt.Errorf("history producer advanced without publishing a frame"))
		}
		if live := len(consumer.editor.String()); consumer.state.bytes != live {
			return consumer.invalidate(fmt.Errorf("no frame was published but the metadata is at %d bytes while the live document has %d", consumer.state.bytes, live))
		}
		return nil
	}
	candidate := consumer.tracker.clone()
	next, err := applyFrames(candidate, frames, latest, consumer.state)
	if err != nil {
		return consumer.invalidate(err)
	}
	if live := len(consumer.editor.String()); next.bytes != live {
		return consumer.invalidate(fmt.Errorf("frame chain ends at %d bytes but the live document has %d", next.bytes, live))
	}
	size := int(estimateTrackerBytes(candidate))
	if size > consumer.budget {
		return consumer.invalidate(fmt.Errorf("consumer metadata budget exceeded: %d > %d bytes", size, consumer.budget))
	}
	consumer.tracker, consumer.state, consumer.metadataBytes = candidate, next, size
	consumer.frames += uint64(len(frames))
	for _, frame := range frames {
		consumer.steps += uint64(len(frame.Steps))
	}
	consumer.revision++
	return nil
}

// LineMask reads only the metadata committed by the last successful Sync.
func (consumer *Consumer) LineMask(start, end, next int) (uint8, error) {
	if consumer == nil {
		return 0, fmt.Errorf("nil change-history consumer")
	}
	if consumer.unavailable {
		return 0, consumer.reason
	}
	if consumer.tracker == nil {
		return 0, fmt.Errorf("change-history consumer has no metadata")
	}
	return consumer.tracker.LineMask(start, end, next)
}

func (consumer *Consumer) readFrames(cursor text.HistoryCursor) ([]text.HistoryFrame, text.HistoryCursor, bool) {
	if consumer.read != nil {
		return consumer.read(cursor)
	}
	return consumer.editor.HistorySince(cursor)
}

func (consumer *Consumer) invalidate(cause error) error {
	if !consumer.unavailable {
		consumer.unavailable = true
		consumer.reason = fmt.Errorf("change history unavailable: %w", cause)
	}
	consumer.tracker = nil
	consumer.state = consumerState{}
	consumer.metadataBytes = 0
	return consumer.reason
}

func applyFrames(candidate *Tracker, frames []text.HistoryFrame, latest text.HistoryCursor, state consumerState) (consumerState, error) {
	if candidate == nil {
		return state, fmt.Errorf("missing metadata candidate")
	}
	expectedFrom, bytes, current := state.cursor, state.bytes, state.current
	seen := map[text.HistoryCursor]struct{}{expectedFrom: {}}
	pending := make([]Event, 0, len(frames))
	for frameIndex, frame := range frames {
		if frame.From != expectedFrom {
			return state, fmt.Errorf("frame %d does not continue the pinned cursor", frameIndex)
		}
		if frame.To == (text.HistoryCursor{}) || frame.To == frame.From {
			return state, fmt.Errorf("frame %d does not advance the producer cursor", frameIndex)
		}
		if _, repeated := seen[frame.To]; repeated {
			return state, fmt.Errorf("frame %d repeats an earlier cursor token", frameIndex)
		}
		seen[frame.To] = struct{}{}
		if frame.BeforeBytes != bytes || len(frame.Steps) == 0 {
			return state, fmt.Errorf("frame %d starts at an inconsistent byte length or has no steps", frameIndex)
		}
		running := frame.BeforeBytes
		for stepIndex, step := range frame.Steps {
			if step.BeforeBytes != running {
				return state, fmt.Errorf("frame %d step %d starts at %d bytes, running is %d", frameIndex, stepIndex, step.BeforeBytes, running)
			}
			if err := validateStepBytes(step, running); err != nil {
				return state, fmt.Errorf("frame %d step %d: %w", frameIndex, stepIndex, err)
			}
			if err := validateStepContext(step); err != nil {
				return state, fmt.Errorf("frame %d step %d: %w", frameIndex, stepIndex, err)
			}
			nextCurrent, err := advanceOperation(step, current)
			if err != nil {
				return state, fmt.Errorf("frame %d step %d: %w", frameIndex, stepIndex, err)
			}
			current = nextCurrent
			events, err := mapStep(step)
			if err != nil {
				return state, fmt.Errorf("frame %d step %d: %w", frameIndex, stepIndex, err)
			}
			pending = append(pending, events...)
			running = step.AfterBytes
		}
		if running != frame.AfterBytes {
			return state, fmt.Errorf("frame %d ends at %d bytes, steps end at %d", frameIndex, frame.AfterBytes, running)
		}
		bytes, expectedFrom = running, frame.To
	}
	if expectedFrom != latest {
		return state, fmt.Errorf("delivered chain ends at an older cursor than the producer's latest")
	}
	for index, event := range pending {
		if err := candidate.Apply(event); err != nil {
			return state, fmt.Errorf("event %d: %w", index, err)
		}
	}
	if candidate.Length() != bytes {
		return state, fmt.Errorf("metadata length %d does not match the byte chain %d", candidate.Length(), bytes)
	}
	return consumerState{cursor: latest, bytes: bytes, current: current}, nil
}

func validateStepBytes(step text.HistoryStep, running int) error {
	if step.Position < 0 || step.Length < 0 || step.AfterBytes < 0 {
		return fmt.Errorf("negative history byte range or resulting length")
	}
	switch step.Primitive {
	case text.HistoryDelete:
		if step.Operation == text.HistorySave || step.Length == 0 || step.Length > running || step.Position > running-step.Length || step.AfterBytes != running-step.Length {
			return fmt.Errorf("invalid delete range or byte transition at %d,%d", step.Position, step.Length)
		}
	case text.HistoryInsert:
		if step.Operation == text.HistorySave || step.Length == 0 || step.Position > running || step.Length > int(^uint(0)>>1)-running || step.AfterBytes != running+step.Length {
			return fmt.Errorf("invalid insert range or byte transition at %d,%d", step.Position, step.Length)
		}
	case text.HistoryNoPrimitive:
		if step.Operation != text.HistorySave || step.Position != 0 || step.Length != 0 || step.AfterBytes != running {
			return fmt.Errorf("invalid primitive-less history step")
		}
	default:
		return fmt.Errorf("unknown history primitive %d", step.Primitive)
	}
	return nil
}

func validateStepContext(step text.HistoryStep) error {
	context := step.Context
	if context.Current < 0 || context.RetainedFrom < 0 || context.RetainedFrom > context.Current || context.RetainedTo < context.Current ||
		context.Save < -1 || context.Save > context.RetainedTo || context.Detach < -1 || context.Detach > context.RetainedTo {
		return fmt.Errorf("history action context is outside retained bounds")
	}
	if step.Operation == text.HistorySave && (context.Save != context.Current || context.Detach != -1) {
		return fmt.Errorf("save step sampled an invalid save or detach position")
	}
	return nil
}

func advanceOperation(step text.HistoryStep, current int) (int, error) {
	switch step.Operation {
	case text.HistoryForward:
		want := current
		if step.Context.Collecting {
			want++
		}
		if step.Context.Current != want {
			return current, fmt.Errorf("forward action position %d, want %d", step.Context.Current, want)
		}
		return want, nil
	case text.HistoryUndo:
		if step.Context.Current != current || current == 0 || step.Context.Current <= step.Context.RetainedFrom {
			return current, fmt.Errorf("undo completion precedes retained history")
		}
		return current - 1, nil
	case text.HistoryRedo:
		if step.Context.Current != current || step.Context.Current >= step.Context.RetainedTo {
			return current, fmt.Errorf("redo completion exceeds retained history")
		}
		return current + 1, nil
	case text.HistorySave:
		if step.Context.Current != current {
			return current, fmt.Errorf("save action position %d, want %d", step.Context.Current, current)
		}
		return current, nil
	default:
		return current, fmt.Errorf("unknown history operation %d", step.Operation)
	}
}

func mapStep(step text.HistoryStep) ([]Event, error) {
	position, length := step.Position, step.Length
	p, s, d := step.Context.Current, step.Context.Save, step.Context.Detach
	beforeReachableSave := s > 0 && s > p
	previousBeforeSave := s < 0 || s >= p
	beforeSave := s < 0 || s > p
	afterSave := s >= 0 && s <= p
	afterDetach := d >= 0 && d < p
	afterOrAtDetach := d >= 0 && d <= p
	base := Event{Position: position, Length: length}
	switch step.Operation {
	case text.HistoryForward:
		switch step.Primitive {
		case text.HistoryInsert:
			base.Kind, base.CollectingUndo, base.BeforeSave = Insert, step.Context.Collecting, beforeReachableSave
		case text.HistoryDelete:
			base.Kind, base.BeforeSave, base.Detached = Delete, beforeReachableSave, afterOrAtDetach
		default:
			return nil, fmt.Errorf("forward step has unsupported primitive %d", step.Primitive)
		}
		return []Event{base}, nil
	case text.HistoryUndo:
		events := make([]Event, 0, 2)
		if previousBeforeSave {
			events = append(events, Event{Kind: StartReversion})
		}
		switch step.Primitive {
		case text.HistoryDelete:
			base.Kind, base.Reverting = UndoInsert, previousBeforeSave && !afterDetach
		case text.HistoryInsert:
			base.Kind, base.Detached = UndoDelete, afterDetach
		default:
			return nil, fmt.Errorf("undo step has unsupported primitive %d", step.Primitive)
		}
		return append(events, base), nil
	case text.HistoryRedo:
		switch step.Primitive {
		case text.HistoryInsert:
			base.Kind, base.CollectingUndo, base.BeforeSave = Insert, step.Context.Collecting, beforeSave && !afterOrAtDetach
		case text.HistoryDelete:
			base.Kind, base.BeforeSave, base.Detached = Delete, beforeReachableSave, afterOrAtDetach
		default:
			return nil, fmt.Errorf("redo step has unsupported primitive %d", step.Primitive)
		}
		events := []Event{base}
		if afterSave {
			events = append(events, Event{Kind: EndReversion})
		}
		return events, nil
	case text.HistorySave:
		if step.Primitive != text.HistoryNoPrimitive {
			return nil, fmt.Errorf("save step has primitive %d", step.Primitive)
		}
		return []Event{{Kind: Save}}, nil
	default:
		return nil, fmt.Errorf("unknown history operation %d", step.Operation)
	}
}

func estimateTrackerBytes(tracker *Tracker) int64 {
	if tracker == nil {
		return 0
	}
	total := int64(unsafe.Sizeof(*tracker)) + estimateChangeLogBacking(&tracker.log)
	if tracker.reversions != nil {
		total += int64(unsafe.Sizeof(*tracker.reversions)) + estimateChangeLogBacking(tracker.reversions)
	}
	return total
}

func estimateChangeLogBacking(log *changeLog) int64 {
	total := int64(cap(log.insertions.runs)) * int64(unsafe.Sizeof(editionRun{}))
	total += int64(cap(log.deletions.positions)) * int64(unsafe.Sizeof(int(0)))
	total += int64(cap(log.deletions.values)) * int64(unsafe.Sizeof([]editionCount(nil)))
	for _, value := range log.deletions.values {
		total += int64(cap(value)) * int64(unsafe.Sizeof(editionCount{}))
	}
	total += int64(cap(log.stack.steps)) * int64(unsafe.Sizeof(changeStep{}))
	total += int64(cap(log.stack.changes)) * int64(unsafe.Sizeof(changeSpan{}))
	return total
}

// Revision advances only when Sync commits at least one producer frame.
func (consumer *Consumer) Revision() uint64 {
	if consumer == nil {
		return 0
	}
	return consumer.revision
}

// Margin keeps the edition bits separate from line-boundary deletion points.
func (consumer *Consumer) Margin(start, end, next int) (editions uint8, deletedAbove, deletedBelow bool, err error) {
	if consumer == nil {
		return 0, false, false, fmt.Errorf("nil change-history consumer")
	}
	if consumer.unavailable {
		return 0, false, false, consumer.reason
	}
	if consumer.tracker == nil {
		return 0, false, false, fmt.Errorf("change-history consumer has no metadata")
	}
	tracker := consumer.tracker
	if start < 0 || end < start || next < end || next > tracker.Length() {
		return 0, false, false, fmt.Errorf("invalid change-history line bounds %d,%d,%d for length %d", start, end, next, tracker.Length())
	}
	for position := start; position < next; position = tracker.EndRun(position) {
		if edition := tracker.EditionAt(position); edition != 0 {
			editions |= 1 << (edition - 1)
		}
	}
	for position := start; position <= end; position = tracker.NextDelete(position) {
		if tracker.DeletesAt(position) == 0 {
			continue
		}
		switch {
		case position == start:
			deletedAbove = true
		case position >= end:
			deletedBelow = true
		default:
			deletedAbove = true
		}
	}
	return editions, deletedAbove, deletedBelow, nil
}
