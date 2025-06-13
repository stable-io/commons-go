package secrets

import (
	"io/fs"
	"os"

	"github.com/fsnotify/fsnotify"
)

// FileReader defines the interface for reading file contents
type FileReader interface {
	ReadFile(path string) ([]byte, error)
	Stat(name string) (fs.FileInfo, error)
}

// FileWatcher defines the interface for watching file changes
type FileWatcher interface {
	Add(path string) error
	Remove(path string) error
	Events() <-chan fsnotify.Event
	Errors() <-chan error
	Close() error
}

// FileWatcherFactory defines the interface for creating new file watchers
type FileWatcherFactory interface {
	NewFileWatcher() (FileWatcher, error)
}

// osReadFile implements FileReader using the real file system
type osReadFile struct{}

func (r *osReadFile) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (r *osReadFile) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

// fsNotifyWatcherFactory implements FileWatcherFactory using fsnotify
type fsNotifyWatcherFactory struct{}

func (f *fsNotifyWatcherFactory) NewFileWatcher() (FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &fsNotifyWatcher{watcher: watcher}, nil
}

// fsNotifyWatcher wraps fsnotify.Watcher to implement FileWatcher
type fsNotifyWatcher struct {
	watcher *fsnotify.Watcher
}

func (w *fsNotifyWatcher) Add(path string) error {
	return w.watcher.Add(path)
}

func (w *fsNotifyWatcher) Remove(path string) error {
	return w.watcher.Remove(path)
}

func (w *fsNotifyWatcher) Events() <-chan fsnotify.Event {
	return w.watcher.Events
}

func (w *fsNotifyWatcher) Errors() <-chan error {
	return w.watcher.Errors
}

func (w *fsNotifyWatcher) Close() error {
	return w.watcher.Close()
}
