package foldertree

import (
	"sync"
	"sync/atomic"

	"github.com/fsnotify/fsnotify"
)

type Watcher struct {
	source    *fsnotify.Watcher
	changed   atomic.Bool
	errors    chan error
	done      chan struct{}
	wake      func()
	closeOnce sync.Once
	closeErr  error
}

func NewWatcher(wake func()) (*Watcher, error) {
	source, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	watcher := &Watcher{source: source, errors: make(chan error, 1), done: make(chan struct{}), wake: wake}
	go watcher.run()
	return watcher, nil
}

func (watcher *Watcher) run() {
	defer close(watcher.done)
	for {
		select {
		case event, ok := <-watcher.source.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0 {
				watcher.changed.Store(true)
				if watcher.wake != nil {
					watcher.wake()
				}
			}
		case err, ok := <-watcher.source.Errors:
			if !ok {
				return
			}
			select {
			case watcher.errors <- err:
			default:
			}
			if watcher.wake != nil {
				watcher.wake()
			}
		}
	}
}

func (watcher *Watcher) Add(path string) error {
	return watcher.source.Add(path)
}

func (watcher *Watcher) Remove(path string) error {
	return watcher.source.Remove(path)
}

func (watcher *Watcher) Consume() (bool, error) {
	changed := watcher.changed.Swap(false)
	select {
	case err := <-watcher.errors:
		return changed, err
	default:
		return changed, nil
	}
}

func (watcher *Watcher) Close() error {
	watcher.closeOnce.Do(func() {
		watcher.closeErr = watcher.source.Close()
		<-watcher.done
	})
	return watcher.closeErr
}
