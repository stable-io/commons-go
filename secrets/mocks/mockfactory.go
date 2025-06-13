package mocks

import "github.com/stable-io/commons-go/secrets"

// MockWatcherFactory implements FileWatcherFactory for testing
type MockWatcherFactory struct {
	watcher *MockFileWatcher
}

// NewMockWatcherFactory creates a new mock watcher factory
func NewMockWatcherFactory() *MockWatcherFactory {
	return &MockWatcherFactory{
		watcher: NewMockFileWatcher(),
	}
}

// NewFileWatcher implements FileWatcherFactory interface
func (m *MockWatcherFactory) NewFileWatcher() (secrets.FileWatcher, error) {
	return m.watcher, nil
}

// GetWatcher returns the underlying mock watcher for test control
func (m *MockWatcherFactory) GetWatcher() *MockFileWatcher {
	return m.watcher
}
