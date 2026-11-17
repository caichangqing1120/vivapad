package changehistory

import (
	"slices"
	"sort"
)

type editionRun struct {
	start, edition int
}

type editionRuns struct {
	length int
	runs   []editionRun
}

func newEditionRuns(length int) editionRuns {
	return editionRuns{length: length, runs: []editionRun{{}}}
}

func (runs *editionRuns) partition(position int) int {
	if position >= runs.length {
		return len(runs.runs) - 1
	}
	return max(0, sort.Search(len(runs.runs), func(index int) bool { return runs.runs[index].start > position })-1)
}

func (runs *editionRuns) from(position int) int {
	index := runs.partition(position)
	for index > 0 && runs.runs[index-1].start == position {
		index--
	}
	return index
}

func (runs *editionRuns) value(position int) int {
	return runs.runs[runs.partition(position)].edition
}

func (runs *editionRuns) end(position int) int {
	index := runs.partition(position) + 1
	if index == len(runs.runs) {
		return runs.length
	}
	return runs.runs[index].start
}

func (runs *editionRuns) split(position int) int {
	index := runs.from(position)
	if runs.runs[index].start < position {
		edition := runs.value(position)
		index++
		runs.runs = slices.Insert(runs.runs, index, editionRun{start: position, edition: edition})
	}
	return index
}

func (runs *editionRuns) shift(index, delta int) {
	for next := index + 1; next < len(runs.runs); next++ {
		runs.runs[next].start += delta
	}
	runs.length += delta
}

func (runs *editionRuns) removeEmpty(index int) {
	if index < len(runs.runs) && len(runs.runs) > 1 {
		end := runs.length
		if index+1 < len(runs.runs) {
			end = runs.runs[index+1].start
		}
		if runs.runs[index].start == end {
			runs.runs = slices.Delete(runs.runs, index, index+1)
		}
	}
}

func (runs *editionRuns) removeSame(index int) {
	if index > 0 && index < len(runs.runs) && runs.runs[index-1].edition == runs.runs[index].edition {
		runs.runs = slices.Delete(runs.runs, index, index+1)
	}
}

func (runs *editionRuns) insertSpace(position, length int) {
	index := runs.from(position)
	if runs.runs[index].start == position {
		edition := runs.value(position)
		if index == 0 {
			if edition != 0 {
				runs.runs[0].edition = 0
				runs.runs = slices.Insert(runs.runs, 1, editionRun{edition: edition})
			}
			runs.shift(0, length)
		} else if edition != 0 {
			runs.shift(index-1, length)
		} else {
			runs.shift(index, length)
		}
	} else {
		runs.shift(index, length)
	}
}

func (runs *editionRuns) deleteRange(position, length int) {
	end := position + length
	first, last := runs.from(position), runs.from(end)
	if first == last {
		runs.shift(first, -length)
		runs.removeEmpty(first)
		return
	}
	first, last = runs.split(position), runs.split(end)
	runs.shift(first, -length)
	runs.runs = slices.Delete(runs.runs, first, last)
	runs.removeEmpty(first)
	runs.removeSame(first)
}

func (runs *editionRuns) fill(position, length, edition int) {
	if length <= 0 {
		return
	}
	end := position + length
	last := runs.from(end)
	if runs.runs[last].edition == edition {
		end = runs.runs[last].start
		if position >= end {
			return
		}
	} else {
		last = runs.split(end)
	}
	first := runs.from(position)
	if runs.runs[first].edition == edition {
		first++
	} else if runs.runs[first].start < position {
		first = runs.split(position)
		last++
	}
	if first < last {
		runs.runs[first].edition = edition
		runs.runs = slices.Delete(runs.runs, first+1, last)
		runs.removeSame(runs.from(end))
		runs.removeSame(first)
		runs.removeEmpty(runs.from(end))
	}
}

type editionCount struct {
	edition, count int
}

type sparseEditions struct {
	positions []int
	values    [][]editionCount
}

func newSparseEditions(length int) sparseEditions {
	return sparseEditions{positions: []int{0, length}, values: make([][]editionCount, 2)}
}

func (editions *sparseEditions) length() int {
	return editions.positions[len(editions.positions)-1]
}

func (editions *sparseEditions) partition(position int) int {
	if position >= editions.length() {
		return len(editions.positions) - 2
	}
	return max(0, sort.Search(len(editions.positions)-1, func(index int) bool { return editions.positions[index] > position })-1)
}

func (editions *sparseEditions) element(position int) int {
	if position >= editions.length() {
		return len(editions.positions) - 1
	}
	return editions.partition(position)
}

func (editions *sparseEditions) value(position int) []editionCount {
	index := editions.element(position)
	if editions.positions[index] == position {
		return editions.values[index]
	}
	return nil
}

func (editions *sparseEditions) set(position int, value []editionCount) {
	index := editions.element(position)
	atStart := editions.positions[index] == position
	if value == nil {
		if position == 0 || position == editions.length() {
			editions.values[index] = nil
		} else if atStart {
			editions.positions = slices.Delete(editions.positions, index, index+1)
			editions.values = slices.Delete(editions.values, index, index+1)
		}
	} else if atStart {
		editions.values[index] = value
	} else {
		editions.positions = slices.Insert(editions.positions, index+1, position)
		editions.values = slices.Insert(editions.values, index+1, value)
	}
}

func (editions *sparseEditions) extract(position int) []editionCount {
	index := editions.element(position)
	value := editions.values[index]
	if index > 0 && index < len(editions.positions)-1 {
		editions.positions = slices.Delete(editions.positions, index, index+1)
		editions.values = slices.Delete(editions.values, index, index+1)
	} else {
		editions.values[index] = nil
	}
	return value
}

func (editions *sparseEditions) next(position int) int {
	index := editions.element(position) + 1
	if index < len(editions.positions) {
		return editions.positions[index]
	}
	return editions.length() + 1
}

func (editions *sparseEditions) shift(index, delta int) {
	for next := index + 1; next < len(editions.positions); next++ {
		editions.positions[next] += delta
	}
}

func (editions *sparseEditions) insertSpace(position, length int) {
	index := editions.partition(position)
	if editions.positions[index] == position {
		occupied := editions.values[index] != nil
		if index == 0 {
			if occupied {
				editions.positions = slices.Insert(editions.positions, 1, 0)
				editions.values = slices.Insert(editions.values, 0, []editionCount(nil))
			}
			editions.shift(0, length)
		} else if occupied {
			editions.shift(index-1, length)
		} else {
			editions.shift(index, length)
		}
	} else {
		editions.shift(index, length)
	}
}

func (editions *sparseEditions) deleteRange(position, length int) {
	end := position + length
	if position == 0 {
		for len(editions.positions) > 2 && editions.positions[1] <= length {
			editions.positions = slices.Delete(editions.positions, 1, 2)
			editions.values = slices.Delete(editions.values, 0, 1)
		}
		editions.shift(0, -length)
		if editions.length() == 0 {
			editions.values[0] = nil
		}
		return
	}
	index := editions.partition(position)
	remove := index
	if editions.positions[index] == position {
		index--
	} else {
		remove++
	}
	for editions.positions[remove] < end {
		editions.positions = slices.Delete(editions.positions, remove, remove+1)
		editions.values = slices.Delete(editions.values, remove, remove+1)
	}
	editions.shift(index, -length)
}

func (runs editionRuns) clone() editionRuns {
	return editionRuns{length: runs.length, runs: slices.Clone(runs.runs)}
}

func (editions sparseEditions) clone() sparseEditions {
	cloned := sparseEditions{
		positions: slices.Clone(editions.positions),
		values:    make([][]editionCount, len(editions.values)),
	}
	for index, value := range editions.values {
		if value != nil {
			cloned.values[index] = slices.Clone(value)
		}
	}
	return cloned
}
