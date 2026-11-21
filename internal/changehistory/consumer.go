package changehistory

// H02: consume the real Viva text history producer into the change-history
// metadata engine.
//
// The Consumer pins one editor identity, the producer's opaque history cursor,
// a Tracker and an explicit unavailable state. It is initialised only from
// TrackHistory's bootstrap byte length; it never rebuilds metadata by diffing
// text. Each Sync copies every new HistoryFrame chain, validates it completely,
// maps it onto Tracker events and applies it to a deep copy of the metadata.
// Cursor, byte length, action position and Tracker are committed together only
// after every frame validates and the final byte length matches the live
// document. Any refusal drops the metadata and makes every later query report
// the unavailable history instead of a guessed "no changes".
//
// The Consumer retains no text, no per-byte array, no frame ring and no replay
// log: it holds only the sparse metadata engine state (edition runs, sparse
// deletion editions, counted restoration spans and fixed headers).
//
// Contract and budget: docs/specs/001-vivapad/h02-consumer-contract.md.

import (
	"fmt"
	"unsafe"

	"github.com/vivaui/viva/text"
)

// maxConsumerMetadataBytes bounds the retained consumer metadata. It is charged
// like the producer budget (structure sizes, not text): 16 MiB is sixteen times
// the producer's own 256 Ki-rune (about 1 MiB) history budget. Measured live
// workloads stay several orders of magnitude below it; crossing it means the
// metadata can no longer be trusted to stay bounded, so the consumer releases
// it and reports the unavailable history.
const maxConsumerMetadataBytes = 16 << 20

// Consumer projects one editor's real history producer onto the metadata engine.
type Consumer struct {
	editor *text.Editor

	state   consumerState
	tracker *Tracker

	metadataBytes int
	revision      uint64
	budget        int

	unavailable bool
	reason      error

	// Diagnostic counters (consumed, not retained).
	frames uint64
	steps  uint64

	// read is a test-only injection point for frame validation; production
	// readers call the pinned editor's HistorySince.
	read consumerFrames
}

type consumerState struct {
	cursor  text.HistoryCursor
	bytes   int
	current int
}

type consumerFrames func(cursor text.HistoryCursor) ([]text.HistoryFrame, text.HistoryCursor, bool)

// NewConsumer pins a fresh history consumer to editor. It calls TrackHistory and
// initialises the metadata from that bootstrap byte length only. The returned
// consumer is always initially available; an error means the producer refused to
// track the editor (already enabled and expired, live Undo/Redo history or an
// active composition), so the caller must record the unavailable history.
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
		editor:        editor,
		tracker:       tracker,
		state:         consumerState{cursor: cursor, bytes: bootstrapBytes},
		metadataBytes: int(estimateTrackerBytes(tracker)),
		budget:        maxConsumerMetadataBytes,
	}, nil
}

// Sync consumes every frame published since the last successful Sync. Frames are
// validated as one complete chain and applied to a candidate deep copy of the
// metadata; the cursor and metadata are committed only after the whole chain and
// the live document length validate. On any failure the consumer releases its
// metadata and enters the unrecoverable unavailable state (the producer has no
// invalidation/checkpoint API, so a dropped cursor cannot be re-bootstrapped).
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
		// An empty read only proves "no new metadata events" if it also agrees
		// with the live document. Otherwise frames were suppressed/lost and the
		// committed metadata is stale, so the consumer must invalidate instead of
		// keeping a clean-looking history that can answer an empty mask.
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
	consumer.tracker = candidate
	consumer.state = next
	consumer.metadataBytes = size
	consumer.frames += uint64(len(frames))
	for _, frame := range frames {
		consumer.steps += uint64(len(frame.Steps))
	}
	if len(frames) > 0 {
		consumer.revision++
	}
	return nil
}

// LineMask answers the original Document::GetMark four-bit projection from the
// metadata committed by the last successful Sync. It refuses when the history is
// unavailable; it never returns an empty mask as a stand-in for "no changes".
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

// invalidate is the single failure exit: it enters the unavailable state, drops
// every retained metadata slot and returns the reported cause. The last committed
// Tracker is never mutated by a rejected chain; the candidate copy is discarded.
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

// applyFrames validates one complete frame chain against the expected cursor and
// byte/action state and applies the mapped events to candidate. It mutates only
// candidate: the caller commits the returned state and the candidate together.
func applyFrames(candidate *Tracker, frames []text.HistoryFrame, latest text.HistoryCursor, state consumerState) (consumerState, error) {
	if candidate == nil {
		return state, fmt.Errorf("missing metadata candidate")
	}
	// First pass: validate the whole chain and produce the mapped events without
	// touching the candidate, so no event of an earlier frame can reach metadata
	// before a later frame is rejected.
	expectedFrom, bytes, current := state.cursor, state.bytes, state.current
	seen := map[text.HistoryCursor]struct{}{expectedFrom: {}}
	pending := make([]Event, 0, len(frames))
	for frameIndex, frame := range frames {
		if frame.From != expectedFrom {
			return state, fmt.Errorf("frame %d does not continue the pinned cursor", frameIndex)
		}
		if frame.To == (text.HistoryCursor{}) {
			return state, fmt.Errorf("frame %d has a zero destination cursor", frameIndex)
		}
		if frame.To == frame.From {
			return state, fmt.Errorf("frame %d does not advance the producer cursor", frameIndex)
		}
		if _, repeated := seen[frame.To]; repeated {
			return state, fmt.Errorf("frame %d repeats an earlier cursor token", frameIndex)
		}
		seen[frame.To] = struct{}{}
		if frame.BeforeBytes != bytes {
			return state, fmt.Errorf("frame %d starts at %d bytes, metadata is at %d", frameIndex, frame.BeforeBytes, bytes)
		}
		if len(frame.Steps) == 0 {
			return state, fmt.Errorf("frame %d has no steps", frameIndex)
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
		bytes = running
		expectedFrom = frame.To
	}
	if expectedFrom != latest {
		return state, fmt.Errorf("delivered chain ends at an older cursor than the producer's latest")
	}
	// Second pass: the chain is fully valid, so apply the mapped events in order.
	for eventIndex, event := range pending {
		if err := candidate.Apply(event); err != nil {
			return state, fmt.Errorf("event %d: %w", eventIndex, err)
		}
	}
	if candidate.Length() != bytes {
		return state, fmt.Errorf("metadata length %d does not match the byte chain %d", candidate.Length(), bytes)
	}
	return consumerState{cursor: latest, bytes: bytes, current: current}, nil
}

// validateStepBytes enforces one primitive's actual source byte range against
// the running document length: delete ranges lie inside the document, insert
// positions are reachable and the running logical length changes exactly by the
// primitive length. Positions may move backwards between primitives (inverse and
// batch replay), but each primitive must be reachable in its current state.
func validateStepBytes(step text.HistoryStep, running int) error {
	if step.Position < 0 || step.Length < 0 {
		return fmt.Errorf("negative source byte range %d,%d", step.Position, step.Length)
	}
	if step.AfterBytes < 0 {
		return fmt.Errorf("negative resulting byte length %d", step.AfterBytes)
	}
	switch step.Primitive {
	case text.HistoryDelete:
		if step.Operation == text.HistorySave {
			return fmt.Errorf("save step carries a delete primitive")
		}
		if step.Length == 0 {
			return fmt.Errorf("delete primitive has zero length")
		}
		if step.Length > running || step.Position > running-step.Length {
			return fmt.Errorf("delete range [%d,%d) exceeds the running document length %d", step.Position, step.Position+step.Length, running)
		}
		if step.AfterBytes != running-step.Length {
			return fmt.Errorf("delete changes bytes %d -> %d, want %d", step.BeforeBytes, step.AfterBytes, running-step.Length)
		}
	case text.HistoryInsert:
		if step.Operation == text.HistorySave {
			return fmt.Errorf("save step carries an insert primitive")
		}
		if step.Length == 0 {
			return fmt.Errorf("insert primitive has zero length")
		}
		if step.Position > running {
			return fmt.Errorf("insert position %d is beyond the running document length %d", step.Position, running)
		}
		if step.Length > int(^uint(0)>>1)-running {
			return fmt.Errorf("insert length %d overflows the document length", step.Length)
		}
		if step.AfterBytes != running+step.Length {
			return fmt.Errorf("insert changes bytes %d -> %d, want %d", step.BeforeBytes, step.AfterBytes, running+step.Length)
		}
	case text.HistoryNoPrimitive:
		if step.Operation != text.HistorySave {
			return fmt.Errorf("primitive-less step has operation %d, want save", step.Operation)
		}
		if step.Position != 0 || step.Length != 0 || step.AfterBytes != running {
			return fmt.Errorf("save step changed a byte range %d,%d -> %d", step.Position, step.Length, step.AfterBytes)
		}
	default:
		return fmt.Errorf("unknown history primitive %d", step.Primitive)
	}
	return nil
}

// validateStepContext keeps the sampled source context inside the ranges the
// producer can publish. These are the same structural invariants the H01 producer
// contract test observes.
func validateStepContext(step text.HistoryStep) error {
	context := step.Context
	if context.Current < 0 {
		return fmt.Errorf("negative action position %d", context.Current)
	}
	if context.RetainedFrom < 0 || context.RetainedFrom > context.Current {
		return fmt.Errorf("retained-from %d is outside the action position %d", context.RetainedFrom, context.Current)
	}
	if context.RetainedTo < context.Current {
		return fmt.Errorf("retained-to %d is before the action position %d", context.RetainedTo, context.Current)
	}
	if context.Save < -1 || context.Save > context.RetainedTo {
		return fmt.Errorf("save position %d is outside the retained history", context.Save)
	}
	if context.Detach < -1 || context.Detach > context.RetainedTo {
		return fmt.Errorf("detach position %d is outside the retained history", context.Detach)
	}
	if step.Operation == text.HistorySave {
		// SetSavePoint publishes the sampled context after setting save = current
		// and clearing detach; any other sampled values are impossible.
		if context.Save != context.Current {
			return fmt.Errorf("save step sampled save position %d, want its current position %d", context.Save, context.Current)
		}
		if context.Detach != -1 {
			return fmt.Errorf("save step sampled detach position %d, want absent", context.Detach)
		}
	}
	return nil
}

// advanceOperation enforces the source action-position evolution: a collected
// forward append or redo advances one action, undo decrements one, save keeps the
// position. An uncollected forward edit is a real source edit that appended no
// action, so it must not move the position.
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
		if step.Context.Current != current {
			return current, fmt.Errorf("undo action position %d, want %d", step.Context.Current, current)
		}
		if current == 0 || step.Context.Current <= step.Context.RetainedFrom {
			return current, fmt.Errorf("undo completion %d would precede the retained history floor %d", current-1, step.Context.RetainedFrom)
		}
		return current - 1, nil
	case text.HistoryRedo:
		if step.Context.Current != current {
			return current, fmt.Errorf("redo action position %d, want %d", step.Context.Current, current)
		}
		if step.Context.Current >= step.Context.RetainedTo {
			return current, fmt.Errorf("redo completion %d would exceed the retained history tail %d", current+1, step.Context.RetainedTo)
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

// mapStep translates one producer step into the metadata engine events using the
// source sampling predicates. p/s/d are the step's sampled Current/Save/Detach.
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
			base.Kind = Insert
			base.CollectingUndo = step.Context.Collecting
			base.BeforeSave = beforeReachableSave
			return []Event{base}, nil
		case text.HistoryDelete:
			base.Kind = Delete
			base.BeforeSave = beforeReachableSave
			base.Detached = afterOrAtDetach
			return []Event{base}, nil
		default:
			return nil, fmt.Errorf("forward step has unsupported primitive %d", step.Primitive)
		}
	case text.HistoryUndo:
		events := make([]Event, 0, 2)
		if previousBeforeSave {
			events = append(events, Event{Kind: StartReversion})
		}
		switch step.Primitive {
		case text.HistoryDelete:
			// Physical delete while undoing: the forward operation was an insert.
			base.Kind = UndoInsert
			base.Reverting = previousBeforeSave && !afterDetach
		case text.HistoryInsert:
			// Physical insert while undoing: the forward operation was a delete.
			base.Kind = UndoDelete
			base.Detached = afterDetach
		default:
			return nil, fmt.Errorf("undo step has unsupported primitive %d", step.Primitive)
		}
		return append(events, base), nil
	case text.HistoryRedo:
		switch step.Primitive {
		case text.HistoryInsert:
			base.Kind = Insert
			base.CollectingUndo = step.Context.Collecting
			base.BeforeSave = beforeSave && !afterOrAtDetach
		case text.HistoryDelete:
			base.Kind = Delete
			base.BeforeSave = beforeReachableSave
			base.Detached = afterOrAtDetach
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

// estimateTrackerBytes measures the retained metadata slots: fixed headers, the
// edition-run backing, sparse deletion positions and edition counts, counted
// restoration spans and the independent reversion log. Backing capacities are
// counted, not just used lengths. There is no term proportional to document
// bytes or to delivered frames/steps (those live only during one Sync).
func estimateTrackerBytes(tracker *Tracker) int64 {
	if tracker == nil {
		return 0
	}
	// Tracker already contains the by-value changeLog header; only its backing
	// arrays are additional allocations.
	total := int64(unsafe.Sizeof(*tracker))
	total += estimateChangeLogBacking(&tracker.log)
	if tracker.reversions != nil {
		total += int64(unsafe.Sizeof(*tracker.reversions))
		total += estimateChangeLogBacking(tracker.reversions)
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

// Revision increments each time Sync consumes at least one frame. Callers that
// cache derived metadata use it together with the text: an equal-text edit or a
// zero-net transaction changes editions without changing the bytes.
func (consumer *Consumer) Revision() uint64 {
	if consumer == nil {
		return 0
	}
	return consumer.revision
}

// Margin reports one logical line's change margin: the edition bits
// (1 << (edition-1)) and whether a deletion point sits at the line's content
// start (above) or end (below). LineMask merges editions and deletions; this
// keeps them apart so a margin can paint the bar and its points differently.
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
			// A deletion inside the line is not a line boundary; mark the top.
			deletedAbove = true
		}
	}
	return editions, deletedAbove, deletedBelow, nil
}
