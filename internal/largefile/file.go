package largefile

import (
	"bytes"
	"container/list"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	pageSize     = 128 << 10
	cachePages   = 32
	lineStride   = 1024
	maxWindow    = 400
	MaxLineBytes = 4096
)

var ErrChanged = errors.New("large file was replaced, truncated, or rewritten")

type Progress struct {
	Size, IndexedBytes int64
	Lines              int
	Revision           uint64
	Done               bool
	Err                error
}

type Line struct {
	Number        int
	Offset        int64
	DisplayOffset int64
	Text          string
	Clipped       bool
}

type checkpoint struct {
	line   int
	offset int64
}

type cachedPage struct {
	offset int64
	data   []byte
}

type File struct {
	handle *os.File
	path   string
	ctx    context.Context
	cancel context.CancelFunc
	work   sync.WaitGroup
	close  sync.Once
	err    error
	resume chan struct{}

	mu          sync.RWMutex
	info        os.FileInfo
	progress    Progress
	checkpoints []checkpoint
	lineStart   int64
	trailingCR  bool
	wake        func()

	cacheMu sync.Mutex
	pages   map[int64]*list.Element
	lru     list.List
}

func Open(path string) (*File, error) {
	handle, err := openShared(path)
	if err != nil {
		return nil, err
	}
	info, err := handle.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = handle.Close()
		if err == nil {
			err = fmt.Errorf("%s is not a regular file", path)
		}
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	file := &File{
		handle: handle, path: path, ctx: ctx, cancel: cancel, info: info,
		progress: Progress{Size: info.Size(), Lines: 1}, checkpoints: []checkpoint{{line: 1}},
		resume: make(chan struct{}, 1), pages: make(map[int64]*list.Element),
	}
	buffer := make([]byte, pageSize+1)
	if _, err := file.indexPage(buffer); err != nil {
		_ = file.Close()
		return nil, err
	}
	file.work.Go(func() { file.index(buffer) })
	return file, nil
}

func (file *File) begin() bool {
	file.mu.Lock()
	defer file.mu.Unlock()
	if file.ctx.Err() != nil {
		return false
	}
	file.work.Add(1)
	return true
}

func (file *File) Close() error {
	file.close.Do(func() {
		file.mu.Lock()
		file.cancel()
		file.wake = nil
		file.mu.Unlock()
		file.work.Wait()
		file.err = file.handle.Close()
		file.cacheMu.Lock()
		file.pages = nil
		file.lru.Init()
		file.cacheMu.Unlock()
		file.mu.Lock()
		file.checkpoints = nil
		file.mu.Unlock()
	})
	return file.err
}

func (file *File) SetWake(wake func()) {
	file.mu.Lock()
	defer file.mu.Unlock()
	if file.ctx.Err() == nil {
		file.wake = wake
	}
}

func (file *File) notify() {
	file.mu.RLock()
	wake := file.wake
	file.mu.RUnlock()
	if wake != nil {
		wake()
	}
}

func (file *File) Progress() Progress {
	file.mu.RLock()
	defer file.mu.RUnlock()
	return file.progress
}

func (file *File) index(buffer []byte) {
	lastWake := time.Now()
	for file.ctx.Err() == nil {
		done, err := file.indexPage(buffer)
		if err != nil {
			file.mu.Lock()
			file.progress.Err = err
			file.mu.Unlock()
			file.notify()
			return
		}
		if done || time.Since(lastWake) >= 100*time.Millisecond {
			file.notify()
			lastWake = time.Now()
		}
		if done {
			select {
			case <-file.ctx.Done():
				return
			case <-file.resume:
			}
		}
	}
}

func (file *File) indexPage(buffer []byte) (bool, error) {
	progress := file.Progress()
	offset, size := progress.IndexedBytes, progress.Size
	if offset >= size {
		file.mu.Lock()
		file.progress.Done = file.progress.IndexedBytes == file.progress.Size
		done := file.progress.Done
		file.mu.Unlock()
		return done, nil
	}
	length := int(min(int64(pageSize+1), size-offset))
	if _, err := file.handle.ReadAt(buffer[:length], offset); err != nil {
		return false, err
	}
	file.mu.Lock()
	defer file.mu.Unlock()
	limit := min(pageSize, length)
	start := 0
	if file.trailingCR && buffer[0] == '\n' {
		start = 1
		file.lineStart++
		last := &file.checkpoints[len(file.checkpoints)-1]
		if last.line == file.progress.Lines {
			last.offset++
		}
	}
	file.trailingCR = false
	for start < limit {
		end := bytes.IndexAny(buffer[start:limit], "\r\n")
		if end < 0 {
			break
		}
		end += start
		start = end + 1
		if buffer[end] == '\r' {
			if start < length && buffer[start] == '\n' {
				start++
			} else if start == length && offset+int64(start) == size {
				file.trailingCR = true
			}
		}
		next := offset + int64(start)
		last := file.checkpoints[len(file.checkpoints)-1]
		if next-file.lineStart >= pageSize && last.line != file.progress.Lines {
			file.checkpoints = append(file.checkpoints, checkpoint{file.progress.Lines, file.lineStart})
		}
		file.progress.Lines++
		file.lineStart = next
		if file.progress.Lines-last.line >= lineStride || next-last.offset >= pageSize {
			file.checkpoints = append(file.checkpoints, checkpoint{file.progress.Lines, next})
		}
	}
	file.progress.IndexedBytes = offset + int64(max(start, limit))
	file.progress.Done = file.progress.IndexedBytes == file.progress.Size
	return file.progress.Done, nil
}

func (file *File) Refresh() (bool, error) {
	if !file.begin() {
		return false, os.ErrClosed
	}
	defer file.work.Done()
	info, err := os.Stat(file.path)
	if err != nil {
		return false, err
	}
	file.cacheMu.Lock()
	defer file.cacheMu.Unlock()
	file.mu.Lock()
	defer file.mu.Unlock()
	if !os.SameFile(file.info, info) || info.Size() < file.info.Size() || file.progress.Err != nil {
		return false, ErrChanged
	}
	if info.Size() == file.info.Size() {
		if !file.info.ModTime().Equal(info.ModTime()) {
			return false, ErrChanged
		}
		return false, nil
	}
	lastPage := file.info.Size() / pageSize * pageSize
	if entry := file.pages[lastPage]; entry != nil {
		delete(file.pages, lastPage)
		file.lru.Remove(entry)
	}
	file.info = info
	file.progress.Size, file.progress.Done = info.Size(), false
	file.progress.Revision++
	select {
	case file.resume <- struct{}{}:
	default:
	}
	return true, nil
}

func (file *File) CacheBytes() int {
	file.cacheMu.Lock()
	defer file.cacheMu.Unlock()
	return len(file.pages) * pageSize
}

func (file *File) page(offset int64) ([]byte, error) {
	base := offset / pageSize * pageSize
	if hit := file.pages[base]; hit != nil {
		file.lru.MoveToFront(hit)
		return hit.Value.(cachedPage).data[int(offset-base):], nil
	}
	var data []byte
	if len(file.pages) == cachePages {
		entry := file.lru.Back()
		page := entry.Value.(cachedPage)
		delete(file.pages, page.offset)
		file.lru.Remove(entry)
		data = page.data[:cap(page.data)]
	} else {
		data = make([]byte, pageSize)
	}
	count, err := file.handle.ReadAt(data, base)
	if err != nil && err != io.EOF {
		return nil, err
	}
	data = data[:count]
	file.pages[base] = file.lru.PushFront(cachedPage{base, data})
	if offset-base > int64(count) {
		return nil, io.ErrUnexpectedEOF
	}
	return data[int(offset-base):], nil
}

func (file *File) Window(first, count, focusLine int, focusOffset int64) ([]Line, error) {
	if !file.begin() {
		return nil, os.ErrClosed
	}
	defer file.work.Done()
	file.cacheMu.Lock()
	defer file.cacheMu.Unlock()
	file.mu.RLock()
	progress, points := file.progress, file.checkpoints
	if count <= 0 || first > progress.Lines {
		file.mu.RUnlock()
		return nil, progress.Err
	}
	first = max(1, first)
	count = min(count, maxWindow, progress.Lines-first+1)
	from := sort.Search(len(points), func(index int) bool { return points[index].line > first }) - 1
	to := sort.Search(len(points), func(index int) bool { return points[index].line > first+count })
	var snapshot [maxWindow + 2]checkpoint
	points = snapshot[:copy(snapshot[:], points[from:to])]
	file.mu.RUnlock()
	start, index := points[0], 1
	lines := make([]Line, 0, count)
	for line, offset := start.line, start.offset; line < first+count; line++ {
		if err := file.ctx.Err(); err != nil {
			return nil, err
		}
		var end, next int64
		var err error
		switch {
		case line == progress.Lines:
			end, next = progress.IndexedBytes, progress.IndexedBytes
		case index < len(points) && points[index].line == line+1:
			next = points[index].offset
			end = next - 1
			if tail, err := file.page(end); err != nil {
				return nil, err
			} else if len(tail) == 0 {
				return nil, io.ErrUnexpectedEOF
			} else if tail[0] == '\n' && end > offset {
				if prev, err := file.page(end - 1); err != nil {
					return nil, err
				} else if len(prev) != 0 && prev[0] == '\r' {
					end--
				}
			}
			index++
		default:
			end, next, err = lineBounds(file.page, offset, progress.IndexedBytes)
			if err != nil {
				return nil, err
			}
		}
		if line >= first {
			shown := offset
			if line == focusLine && focusOffset-offset >= MaxLineBytes/2 {
				shown = min(max(offset, focusOffset-MaxLineBytes/4), max(offset, end-MaxLineBytes))
			}
			length := min(int64(MaxLineBytes), end-shown)
			var value []byte
			if length > 0 {
				value = make([]byte, length)
				if err := file.readCached(value, shown); err != nil {
					return nil, err
				}
			}
			for shown > offset && len(value) > 0 && !utf8.RuneStart(value[0]) {
				shown++
				value = value[1:]
			}
			if shown+int64(len(value)) < end {
				for tail := max(0, len(value)-utf8.UTFMax+1); tail < len(value); {
					if !utf8.FullRune(value[tail:]) {
						value = value[:tail]
						break
					}
					_, width := utf8.DecodeRune(value[tail:])
					tail += width
				}
			}
			lines = append(lines, Line{line, offset, shown, string(value), shown != offset || shown+int64(len(value)) < end})
		}
		offset = next
	}
	return lines, nil
}

func (file *File) readCached(value []byte, offset int64) error {
	for len(value) > 0 {
		page, err := file.page(offset)
		if err != nil {
			return err
		}
		count := copy(value, page)
		if count == 0 {
			return io.ErrUnexpectedEOF
		}
		value = value[count:]
		offset += int64(count)
	}
	return nil
}

func lineBounds(page func(int64) ([]byte, error), offset, size int64) (end, next int64, err error) {
	for offset < size {
		chunk, err := page(offset)
		if err != nil {
			return 0, 0, err
		}
		if len(chunk) == 0 {
			return 0, 0, io.ErrUnexpectedEOF
		}
		chunk = chunk[:min(int64(len(chunk)), size-offset)]
		index := bytes.IndexAny(chunk, "\r\n")
		if index < 0 {
			offset += int64(len(chunk))
			continue
		}
		end = offset + int64(index)
		next = end + 1
		if chunk[index] == '\r' && next < size {
			tail := chunk[index+1:]
			if len(tail) == 0 {
				tail, err = page(next)
				if err != nil {
					return 0, 0, err
				}
			}
			if len(tail) != 0 && tail[0] == '\n' {
				next++
			}
		}
		return end, next, nil
	}
	return size, size, nil
}
