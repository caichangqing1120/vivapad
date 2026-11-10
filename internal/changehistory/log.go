package changehistory

import "slices"

type changeSpan struct {
	position, length, edition, count int
	deletion                         bool
}

type changeStack struct {
	steps   []changeStep
	changes []changeSpan
}

type changeStep struct {
	count, position, length int
}

func (stack *changeStack) pushDeletion(position int, edition editionCount) {
	stack.steps[len(stack.steps)-1].count += edition.count
	if len(stack.changes) != 0 {
		last := &stack.changes[len(stack.changes)-1]
		if last.deletion && last.position == position && last.length == 0 && last.edition == edition.edition {
			last.count += edition.count
			return
		}
	}
	stack.changes = append(stack.changes, changeSpan{position: position, edition: edition.edition, count: edition.count, deletion: true})
}

func (stack *changeStack) pushInsertion(position, length, edition int) {
	stack.steps[len(stack.steps)-1].count++
	stack.changes = append(stack.changes, changeSpan{position: position, length: length, edition: edition, count: 1})
}

func (stack *changeStack) popStep() int {
	last := len(stack.steps) - 1
	count := stack.steps[last].count
	stack.steps = stack.steps[:last]
	return count
}

func (stack *changeStack) popSpan(maximum int) changeSpan {
	last := len(stack.changes) - 1
	span := stack.changes[last]
	remove := min(maximum, span.count)
	if remove == span.count {
		stack.changes = stack.changes[:last]
	} else {
		stack.changes[last].count -= remove
		span.count = remove
	}
	return span
}

func pushEdition(editions []editionCount, edition editionCount) []editionCount {
	if len(editions) != 0 && editions[len(editions)-1].edition == edition.edition {
		editions[len(editions)-1].count += edition.count
		return editions
	}
	return append(editions, edition)
}

func popEdition(editions []editionCount) []editionCount {
	last := len(editions) - 1
	if editions[last].count == 1 {
		return editions[:last]
	}
	editions[last].count--
	return editions
}

type changeLog struct {
	stack      changeStack
	insertions editionRuns
	deletions  sparseEditions
}

func newChangeLog(length int) changeLog {
	return changeLog{insertions: newEditionRuns(length), deletions: newSparseEditions(length)}
}

func (log *changeLog) insertSpace(position, length int) {
	log.insertions.insertSpace(position, length)
	log.deletions.insertSpace(position, length)
}

func (log *changeLog) deleteRange(position, length int) {
	log.insertions.deleteRange(position, length)
	saved := slices.Clone(log.deletions.value(position))
	log.deletions.deleteRange(position, length)
	if saved != nil {
		log.deletions.set(position, saved)
	}
}

func (log *changeLog) pushDeletion(position int, edition editionCount) {
	log.deletions.set(position, pushEdition(log.deletions.value(position), edition))
}

func (log *changeLog) saveRange(position, length int) {
	log.stack.steps = append(log.stack.steps, changeStep{position: position, length: length})
	end := position + length
	start := position
	if log.insertions.value(start) == 0 {
		start = log.insertions.end(start)
	}
	for start < end {
		last := log.insertions.end(start)
		log.stack.pushInsertion(start, min(last, end)-start, log.insertions.value(start))
		start = log.insertions.end(last)
	}
	for at := position + 1; at <= end; at = log.deletions.next(at) {
		for _, edition := range log.deletions.value(at) {
			log.stack.pushDeletion(at, edition)
		}
	}
}

func (log *changeLog) saveHistory(position, length int) {
	log.saveRange(position, length)
	for at, end := position+1, position+length; at <= end; at = log.deletions.next(at) {
		for _, edition := range log.deletions.value(at) {
			log.pushDeletion(position, edition)
		}
		log.deletions.set(at, nil)
	}
}

func (log *changeLog) deleteSavingHistory(position, length int) {
	log.saveHistory(position, length)
	log.deleteRange(position, length)
}

func (log *changeLog) canPop(position, length int) bool {
	if len(log.stack.steps) == 0 {
		return false
	}
	step := log.stack.steps[len(log.stack.steps)-1]
	if step.position != position || step.length != length {
		return false
	}
	editions := log.deletions.value(position)
	index, available := len(editions)-1, 0
	consume := func(count, edition int) bool {
		for count > 0 {
			if available == 0 {
				if index < 0 {
					return false
				}
				available = editions[index].count
			}
			if available <= 0 || edition != 0 && editions[index].edition != edition {
				return false
			}
			removed := min(count, available)
			count, available = count-removed, available-removed
			if available == 0 {
				index--
			}
		}
		return true
	}
	if !consume(1, 0) {
		return false
	}
	// Collapsed older deletions are not necessarily enough to restore this step.
	// Validate its compressed spans and editions before either log is mutated.
	remaining := step.count
	for index := len(log.stack.changes) - 1; remaining > 0; index-- {
		if index < 0 {
			return false
		}
		span := log.stack.changes[index]
		if span.count <= 0 || span.position < position || span.position > position+length || span.length < 0 || span.length > position+length-span.position {
			return false
		}
		count := min(remaining, span.count)
		if span.deletion {
			if !consume(count, span.edition) {
				return false
			}
		} else if span.count != 1 || span.length == 0 {
			return false
		}
		remaining -= count
	}
	return true
}

func (log *changeLog) popDeletion(position, length int) {
	editions := log.deletions.extract(position + length)
	log.deletions.set(position, editions)
	editions = popEdition(editions)
	count := log.stack.popStep()
	for restored := 0; restored < count; {
		span := log.stack.popSpan(count - restored)
		if !span.deletion {
			log.insertions.fill(span.position, span.length, span.edition)
			restored++
		} else {
			for range span.count {
				editions = popEdition(editions)
			}
			at := log.deletions.value(span.position)
			at = slices.Insert(at, 0, editionCount{edition: span.edition, count: span.count})
			log.deletions.set(span.position, at)
			restored += span.count
		}
	}
	if len(editions) == 0 {
		log.deletions.set(position, nil)
	} else {
		log.deletions.set(position, editions)
	}
}

func (log *changeLog) save() {
	for index := range log.stack.changes {
		if log.stack.changes[index].edition == 3 {
			log.stack.changes[index].edition = 2
		}
	}
	for start := 0; start < log.insertions.length; {
		end := log.insertions.end(start)
		if log.insertions.value(start) == 3 {
			log.insertions.fill(start, end-start, 2)
		}
		start = end
	}
	for _, editions := range log.deletions.values {
		for index := range editions {
			if editions[index].edition == 3 {
				editions[index].edition = 2
			}
		}
	}
}

func (log *changeLog) deletionCount(position, length int) int {
	count := 0
	for at, end := position, position+length; at <= end; at = log.deletions.next(at) {
		for _, edition := range log.deletions.value(at) {
			count += edition.count
		}
	}
	return count
}
