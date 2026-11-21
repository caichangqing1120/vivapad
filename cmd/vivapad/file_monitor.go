package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

type fileMonitor struct {
	source    *fsnotify.Watcher
	path      string
	changed   atomic.Bool
	missing   atomic.Bool
	errors    chan error
	stop      chan struct{}
	done      chan struct{}
	wake      func()
	closeOnce sync.Once
	closeErr  error
}

func newFileMonitor(path string, wake func(), interval time.Duration) (*fileMonitor, error) {
	initial, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !initial.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	source, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := source.Add(filepath.Dir(path)); err != nil {
		_ = source.Close()
		return nil, err
	}
	monitor := &fileMonitor{source: source, path: filepath.Clean(path), errors: make(chan error, 1), stop: make(chan struct{}), done: make(chan struct{}), wake: wake}
	go monitor.run(initial, interval)
	return monitor, nil
}

func (monitor *fileMonitor) run(previous os.FileInfo, interval time.Duration) {
	defer close(monitor.done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	pending, missingTicks := false, 0
	for {
		select {
		case event, ok := <-monitor.source.Events:
			if !ok {
				return
			}
			name := filepath.Clean(event.Name)
			if name == monitor.path || runtime.GOOS == "windows" && strings.EqualFold(name, monitor.path) {
				pending = pending || event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename|fsnotify.Chmod) != 0
			}
		case err, ok := <-monitor.source.Errors:
			if !ok {
				return
			}
			select {
			case monitor.errors <- err:
			default:
			}
			monitor.wake()
		case <-ticker.C:
			info, err := os.Lstat(monitor.path)
			if errors.Is(err, os.ErrNotExist) {
				missingTicks++
				if missingTicks >= 2 && !monitor.missing.Swap(true) {
					monitor.changed.Store(true)
					monitor.wake()
				}
				continue
			}
			if err != nil || !info.Mode().IsRegular() {
				if err == nil {
					err = fmt.Errorf("%s is not a regular file", monitor.path)
				}
				select {
				case monitor.errors <- err:
				default:
				}
				monitor.wake()
				continue
			}
			if missingTicks > 0 {
				pending = true
				missingTicks = 0
			}
			if pending || info.Size() != previous.Size() || !info.ModTime().Equal(previous.ModTime()) || !os.SameFile(info, previous) {
				monitor.changed.Store(true)
				monitor.wake()
			}
			pending, previous = false, info
		case <-monitor.stop:
			return
		}
	}
}

func (monitor *fileMonitor) Consume() (changed, missing bool, err error) {
	changed, missing = monitor.changed.Swap(false), monitor.missing.Swap(false)
	select {
	case err = <-monitor.errors:
	default:
	}
	return changed, missing, err
}

func (monitor *fileMonitor) Close() error {
	monitor.closeOnce.Do(func() {
		close(monitor.stop)
		monitor.closeErr = monitor.source.Close()
		<-monitor.done
	})
	return monitor.closeErr
}
