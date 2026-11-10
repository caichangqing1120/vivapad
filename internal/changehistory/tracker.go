package changehistory

import "fmt"

type Kind uint8

const (
	Insert Kind = iota + 1
	Delete
	UndoInsert
	UndoDelete
	Save
	StartReversion
	EndReversion
	Reset
)

type Event struct {
	Kind                                            Kind
	Position, Length                                int
	CollectingUndo, BeforeSave, Detached, Reverting bool
}

type Tracker struct {
	log        changeLog
	reversions *changeLog
}

func New(length int) (*Tracker, error) {
	if length < 0 || length >= int(^uint(0)>>1) {
		return nil, fmt.Errorf("invalid change-history length %d", length)
	}
	return &Tracker{log: newChangeLog(length)}, nil
}

func (tracker *Tracker) Length() int { return tracker.log.insertions.length }

func (tracker *Tracker) Apply(event Event) error {
	if err := tracker.validate(event); err != nil {
		return err
	}
	position, length := event.Position, event.Length
	switch event.Kind {
	case Reset:
		tracker.log, tracker.reversions = newChangeLog(length), nil
	case Save:
		tracker.log.save()
		tracker.reversions = nil
	case StartReversion:
		if tracker.reversions == nil {
			reversions := newChangeLog(tracker.Length())
			tracker.reversions = &reversions
		}
	case EndReversion:
		tracker.reversions = nil
	case Insert:
		tracker.log.insertSpace(position, length)
		edition := 0
		if event.CollectingUndo {
			edition = 3
			if event.BeforeSave {
				edition = 2
			}
		}
		tracker.log.insertions.fill(position, length, edition)
		if tracker.reversions != nil {
			tracker.reversions.insertSpace(position, length)
			if event.BeforeSave {
				tracker.reversions.popDeletion(position, length)
			}
		}
	case Delete:
		tracker.log.deleteSavingHistory(position, length)
		edition := 3
		if event.BeforeSave {
			edition = 2
		}
		tracker.log.pushDeletion(position, editionCount{edition: edition, count: 1})
		if tracker.reversions != nil {
			if event.Detached {
				tracker.reversions.saveHistory(position, length)
			}
			tracker.reversions.deleteRange(position, length)
		}
	case UndoInsert:
		tracker.log.deleteRange(position, length)
		if tracker.reversions != nil {
			tracker.reversions.deleteSavingHistory(position, length)
			if event.Reverting {
				tracker.reversions.pushDeletion(position, editionCount{edition: 1, count: 1})
			}
		}
	case UndoDelete:
		tracker.log.insertSpace(position, length)
		tracker.log.popDeletion(position, length)
		if tracker.reversions != nil {
			tracker.reversions.insertSpace(position, length)
			if !event.Detached {
				tracker.reversions.insertions.fill(position, length, 1)
			}
		}
	}
	return nil
}

func (tracker *Tracker) validate(event Event) error {
	invalid := false
	flags := event.CollectingUndo || event.BeforeSave || event.Detached || event.Reverting
	maximum := int(^uint(0) >> 1)
	switch event.Kind {
	case Reset:
		invalid = event.Position != 0 || event.Length < 0 || event.Length >= maximum || flags
	case Save, StartReversion, EndReversion:
		invalid = event.Position != 0 || event.Length != 0 || flags
	case Insert, UndoDelete:
		invalid = event.Position < 0 || event.Position > tracker.Length() || event.Length <= 0 || event.Length >= maximum-tracker.Length()
		if event.Kind == Insert {
			invalid = invalid || event.Detached || event.Reverting
			if !invalid && event.BeforeSave && tracker.reversions != nil {
				invalid = !tracker.reversions.canPop(event.Position, event.Length)
			}
		} else {
			invalid = invalid || event.CollectingUndo || event.BeforeSave || event.Reverting
			if !invalid {
				invalid = !tracker.log.canPop(event.Position, event.Length)
			}
		}
	case Delete, UndoInsert:
		invalid = event.Position < 0 || event.Length <= 0 || event.Length > tracker.Length() || event.Position > tracker.Length()-event.Length || event.CollectingUndo
		if event.Kind == Delete {
			invalid = invalid || event.Reverting
		} else {
			invalid = invalid || event.BeforeSave || event.Detached
			if !invalid {
				invalid = tracker.log.deletionCount(event.Position, event.Length-1) != 0
			}
		}
	default:
		invalid = true
	}
	if invalid {
		return fmt.Errorf("invalid change-history event %+v", event)
	}
	return nil
}

func (tracker *Tracker) EditionAt(position int) int {
	if position < 0 || position > tracker.Length() {
		return 0
	}
	edition := tracker.log.insertions.value(position)
	if tracker.reversions != nil && tracker.reversions.insertions.value(position) != 0 {
		if edition > 0 {
			return 4
		}
		return 1
	}
	return edition
}

func (tracker *Tracker) EndRun(position int) int {
	if position < 0 || position > tracker.Length() {
		return tracker.Length()
	}
	end := tracker.log.insertions.end(position)
	if tracker.reversions != nil {
		end = min(end, tracker.reversions.insertions.end(position))
	}
	return end
}

func (tracker *Tracker) DeletesAt(position int) uint32 {
	if position < 0 || position > tracker.Length() {
		return 0
	}
	var flags uint32
	for _, edition := range tracker.log.deletions.value(position) {
		flags |= 1 << (edition.edition - 1)
	}
	if tracker.reversions != nil && tracker.reversions.deletions.value(position) != nil {
		if flags&6 == 0 {
			flags |= 1
		} else {
			flags |= 8
		}
	}
	return flags
}

func (tracker *Tracker) NextDelete(position int) int {
	if position < 0 || position > tracker.Length() {
		return tracker.Length() + 1
	}
	next := tracker.log.deletions.next(position)
	if tracker.reversions != nil {
		next = min(next, tracker.reversions.deletions.next(position))
	}
	return next
}
