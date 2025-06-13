package mocks

import (
	"fmt"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// MockFileWatcher provides a deterministic file watcher for testing
type MockFileWatcher struct {
	events    chan fsnotify.Event
	errors    chan error
	watched   map[string]bool
	mu        sync.RWMutex
	closed    bool
	closeOnce sync.Once
}

// NewMockFileWatcher creates a new mock file watcher
func NewMockFileWatcher() *MockFileWatcher {
	return &MockFileWatcher{
		events:  make(chan fsnotify.Event, 100),
		errors:  make(chan error, 100),
		watched: make(map[string]bool),
	}
}

// Add adds a path to watch
func (m *MockFileWatcher) Add(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("watcher is closed")
	}

	m.watched[path] = true
	return nil
}

// Remove removes a path from watching
func (m *MockFileWatcher) Remove(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("watcher is closed")
	}

	delete(m.watched, path)
	return nil
}

// Events returns the events channel
func (m *MockFileWatcher) Events() <-chan fsnotify.Event {
	return m.events
}

// Errors returns the errors channel
func (m *MockFileWatcher) Errors() <-chan error {
	return m.errors
}

// Close closes the watcher
func (m *MockFileWatcher) Close() error {
	m.closeOnce.Do(func() {
		m.mu.Lock()
		m.closed = true
		close(m.events)
		close(m.errors)
		m.mu.Unlock()
	})
	return nil
}

// SimulateWrite simulates a file write event
func (m *MockFileWatcher) SimulateWrite(path string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return
	}

	if m.watched[path] {
		m.events <- fsnotify.Event{
			Name: path,
			Op:   fsnotify.Write,
		}
	}
}

// SimulateError simulates a watcher error
func (m *MockFileWatcher) SimulateError(err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return
	}

	m.errors <- err
}
