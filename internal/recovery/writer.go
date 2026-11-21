package recovery

import (
	"errors"
	"slices"
	"sync"
)

type Writer struct {
	path   string
	onDone func()

	mutex      sync.Mutex
	latest     State
	generation uint64
	running    bool
	done       chan struct{}
	lastError  error
}

func NewWriter(path string, onDone func()) *Writer {
	return &Writer{path: path, onDone: onDone}
}

// Submit queues a snapshot. Validation and write errors are delivered through
// Wait and LastError for the same generation.
func (writer *Writer) Submit(state State) {
	state.Tabs = slices.Clone(state.Tabs)
	for i := range state.Tabs {
		state.Tabs[i].FoldedHeaders = slices.Clone(state.Tabs[i].FoldedHeaders)
	}
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	writer.latest = state
	writer.generation++
	if !writer.running {
		writer.startLocked()
	}
}

// RetryFailed retries the latest snapshot only after a completed failure.
func (writer *Writer) RetryFailed() {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	if !writer.running && writer.lastError != nil {
		writer.generation++
		writer.startLocked()
	}
}

func (writer *Writer) startLocked() {
	writer.running = true
	writer.done = make(chan struct{})
	go writer.flush()
}

func (writer *Writer) flush() {
	for {
		writer.mutex.Lock()
		state, generation := writer.latest, writer.generation
		writer.mutex.Unlock()
		err := Save(writer.path, state)
		writer.mutex.Lock()
		if generation == writer.generation {
			writer.lastError = err
			writer.running = false
			close(writer.done)
			writer.mutex.Unlock()
			if writer.onDone != nil {
				writer.onDone()
			}
			return
		}
		writer.mutex.Unlock()
	}
}

func (writer *Writer) Wait() error {
	writer.mutex.Lock()
	done := writer.done
	writer.mutex.Unlock()
	if done != nil {
		<-done
	}
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	return writer.lastError
}

// Status returns the last write error and whether the latest write finished.
func (writer *Writer) Status() (err error, finished bool) {
	writer.mutex.Lock()
	defer writer.mutex.Unlock()
	return writer.lastError, writer.done != nil && !writer.running
}

func (writer *Writer) Clear() error {
	writeErr := writer.Wait()
	return errors.Join(writeErr, Clear(writer.path))
}
