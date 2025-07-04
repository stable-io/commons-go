package mocks

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// mockFileInfo implements fs.FileInfo for testing
type mockFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() fs.FileMode  { return m.mode }
func (m *mockFileInfo) ModTime() time.Time { return m.modTime }
func (m *mockFileInfo) IsDir() bool        { return m.isDir }
func (m *mockFileInfo) Sys() interface{}   { return nil }

// MockFileSystem provides a deterministic in-memory file system for testing
type MockFileSystem struct {
	files     map[string][]byte
	dirs      map[string]bool
	mu        sync.RWMutex
	writeChan chan string
}

// NewMockFileSystem creates a new mock file system
func NewMockFileSystem() *MockFileSystem {
	return &MockFileSystem{
		files:     make(map[string][]byte),
		dirs:      make(map[string]bool),
		writeChan: make(chan string, 100),
	}
}

// WriteFile writes content to a file
func (m *MockFileSystem) WriteFile(path string, content []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Create a copy of the content to prevent external modifications
	contentCopy := make([]byte, len(content))
	copy(contentCopy, content)

	m.files[path] = contentCopy

	// Ensure the directory exists
	dir := filepath.Dir(path)
	m.dirs[dir] = true

	m.writeChan <- path
}

// CreateDir creates a directory entry
func (m *MockFileSystem) CreateDir(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dirs[path] = true
}

// ReadFile reads content from a file
func (m *MockFileSystem) ReadFile(path string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	content, exists := m.files[path]
	if !exists {
		return nil, &os.PathError{Op: "read", Path: path, Err: os.ErrNotExist}
	}

	// Return a copy to prevent external modifications
	contentCopy := make([]byte, len(content))
	copy(contentCopy, content)
	return contentCopy, nil
}

// Stat returns file info for the given path
func (m *MockFileSystem) Stat(name string) (fs.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	content, exists := m.files[name]
	if !exists {
		return nil, &os.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
	}

	return &mockFileInfo{
		name:    name,
		size:    int64(len(content)),
		mode:    0644,
		modTime: time.Now(),
		isDir:   false,
	}, nil
}

// FileExists checks if a file exists
func (m *MockFileSystem) FileExists(path string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.files[path]
	return exists
}

// GetWriteEvents returns a channel that receives file paths when they are written
func (m *MockFileSystem) GetWriteEvents() <-chan string {
	return m.writeChan
}

// Close cleans up resources
func (m *MockFileSystem) Close() {
	close(m.writeChan)
}

// ReadDir reads directory entries
func (m *MockFileSystem) ReadDir(dirname string) ([]fs.DirEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if directory exists (either as a directory or has files in it)
	dirExists := m.dirs[dirname]
	hasFiles := false
	for path := range m.files {
		if filepath.Dir(path) == dirname {
			hasFiles = true
			break
		}
	}

	if !dirExists && !hasFiles {
		return nil, &os.PathError{Op: "readdir", Path: dirname, Err: os.ErrNotExist}
	}

	var entries []fs.DirEntry

	// Add file entries
	for path := range m.files {
		dir := filepath.Dir(path)
		if dir == dirname {
			filename := filepath.Base(path)
			entries = append(entries, &mockDirEntry{
				name:  filename,
				isDir: false,
			})
		}
	}

	// Add directory entries
	for path := range m.dirs {
		dir := filepath.Dir(path)
		if dir == dirname {
			dirname := filepath.Base(path)
			entries = append(entries, &mockDirEntry{
				name:  dirname,
				isDir: true,
			})
		}
	}

	return entries, nil
}

// mockDirEntry implements fs.DirEntry for testing
type mockDirEntry struct {
	name  string
	isDir bool
}

func (m *mockDirEntry) Name() string { return m.name }
func (m *mockDirEntry) IsDir() bool  { return m.isDir }
func (m *mockDirEntry) Type() fs.FileMode {
	if m.isDir {
		return fs.ModeDir
	}
	return 0
}
func (m *mockDirEntry) Info() (fs.FileInfo, error) {
	return &mockFileInfo{
		name:  m.name,
		isDir: m.isDir,
	}, nil
}
