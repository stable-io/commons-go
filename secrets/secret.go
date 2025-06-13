package secrets

import (
	"context"
	"fmt"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// fileSecret implements Secret interface with file watching capabilities
type fileSecret struct {
	id             string
	path           string
	value          ConcurrentValue[string]
	subscribers    concurrentList[subscriberInfo]
	watcher        ConcurrentValue[FileWatcher]
	watchOnce      sync.Once
	closed         ConcurrentValue[bool]
	closeOnce      sync.Once
	ctx            context.Context
	err            ConcurrentValue[error]
	reader         FileReader
	watcherFactory FileWatcherFactory
}

func (fs *fileSecret) Value() string {
	return fs.value.Get()
}

// ListenChanges Changes returns a new dedicated channel for receiving secret updates
// File watching starts lazily on first call to Changes()
func (fs *fileSecret) ListenChanges() (<-chan string, error) {

	if fs.closed.Get() {
		return nil, fmt.Errorf("secret %s is closed", fs.id)
	}

	var err error
	// Start watching on first subscriber (lazy initialization)
	fs.watchOnce.Do(func() {
		w := fs.watcher.Get()
		if w == nil {
			err = fmt.Errorf("file watcher is not initialized")
		} else {
			err = w.Add(fs.path)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start watching secret %s: %w", fs.id, err)
	}

	// Create buffered channel for this subscriber
	ch := make(chan string, 1)
	fs.subscribers.Add(subscriberInfo{
		ch: ch,
	})

	return ch, nil
}

func (fs *fileSecret) getFileEvents() <-chan fsnotify.Event {
	w := fs.watcher.Get()
	if w == nil {
		c := make(chan fsnotify.Event)
		close(c)
		return c
	}
	return w.Events()
}

func (fs *fileSecret) watchingErrorEvents() <-chan error {
	w := fs.watcher.Get()
	if w == nil {
		c := make(chan error)
		close(c)
		return c
	}
	return w.Errors()
}

// addWatchPath initializes file watching and handles file change events
func (fs *fileSecret) addWatchPath() error {
	watcher := fs.watcher.Get()
	if watcher == nil {
		return fmt.Errorf("missing file watcher")
	}
	return watcher.Add(fs.path)
}

// handleFileChange reads the new file content and broadcasts to subscribers
func (fs *fileSecret) handleFileChange() {
	// Read new content
	content, err := fs.reader.ReadFile(fs.path)
	if err != nil {
		fs.Close()
		return
	}

	newValue := string(content)

	if newValue == fs.value.Get() {
		return // No change, skip broadcasting
	}

	// Update cached value
	fs.value.Set(newValue)

	// Broadcast to all subscribers with failure tracking
	subscribers := fs.subscribers.Get()
	activeSubscribers := make([]subscriberInfo, 0, len(subscribers))
	for _, sub := range subscribers {
		select {
		case sub.ch <- newValue:
			activeSubscribers = append(activeSubscribers, sub)
		default:
			// Channel buffer is full, close and remove channel
			close(sub.ch)
		}
	}

	// Update subscribers list (filtering out closed channels)
	fs.subscribers.Set(activeSubscribers)
}

// Close stops watching and closes all subscriber channels
func (fs *fileSecret) Close() {

	// Already closed
	if fs.closed.Get() {
		return
	}

	// Ensure close logic runs only once
	fs.closeOnce.Do(func() {
		// first avoid close the door for new subscribers
		fs.closed.Set(true)

		// signal all subscribers that the secret is closed
		for _, sub := range fs.subscribers.Get() {
			close(sub.ch)
		}
		fs.subscribers.Set([]subscriberInfo{})
	})
}

func (fs *fileSecret) setError(err error) {
	fs.err.Set(err)
}

func (fs *fileSecret) Err() error {
	return fs.err.Get()
}
