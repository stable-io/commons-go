package secrets

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

const DefaultBasePath = "/mnt/secrets_store"

// Secret represents a watchable secret with change notifications
type Secret interface {
	Value() string
	// ListenChanges returns a new dedicated channel for receiving secret updates.
	// The returned channel will be closed when the secret will not be watched anymore, this could be due to an error.
	ListenChanges() (<-chan string, error) // Each call returns a new dedicated channel
}

// SecretLoader defines the interface for loading secrets (Port in Hexagonal Architecture)
type SecretLoader interface {
	GetSecret(secretKey string) (Secret, error)
	Close()
}

type fileSecretLoader struct {
	basePath       string
	ctx            context.Context
	cancelCtxFn    context.CancelFunc
	isClosed       ConcurrentValue[bool]
	closeOnce      sync.Once
	reader         FileReader
	watcherFactory FileWatcherFactory
	watcher        FileWatcher
	secrets        ConcurrentMap[string, *fileSecret]
	err            ConcurrentValue[error]
}

// subscriberInfo holds channel and failure tracking
type subscriberInfo struct {
	ch chan string
}

// Option defines a functional option for configuring the secret loader
type Option func(*fileSecretLoader)

// WithBasePath sets a custom base path for the secret loader
func WithBasePath(basePath string) Option {
	return func(fsl *fileSecretLoader) {
		fsl.basePath = basePath
	}
}

// WithFileReader sets a custom file reader for the secret loader
func WithFileReader(reader FileReader) Option {
	return func(fsl *fileSecretLoader) {
		fsl.reader = reader
	}
}

// WithWatcherFactory sets a custom watcher factory for the secret loader
func WithWatcherFactory(factory FileWatcherFactory) Option {
	return func(fsl *fileSecretLoader) {
		fsl.watcherFactory = factory
	}
}

// NewFileSecretLoader creates a new fileSecretLoader with optional configuration
func NewFileSecretLoader(ctx context.Context, opts ...Option) (SecretLoader, error) {
	childCtx, cancelFunc := context.WithCancel(ctx)
	fsl := &fileSecretLoader{
		ctx:            childCtx,
		cancelCtxFn:    cancelFunc,
		basePath:       DefaultBasePath,
		reader:         &osReadFile{},
		watcherFactory: &fsNotifyWatcherFactory{},
		secrets: ConcurrentMap[string, *fileSecret]{
			value: make(map[string]*fileSecret),
		},
	}

	// Apply all provided options
	for _, opt := range opts {
		opt(fsl)
	}

	watcher, err := fsl.watcherFactory.NewFileWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}
	fsl.watcher = watcher

	err = fsl.startWatching()

	return fsl, err
}

// GetSecret loads a secret and returns a Secret object that can be watched for changes
func (fsl *fileSecretLoader) GetSecret(secretKey string) (Secret, error) {

	if fsl.isClosed.Get() {
		return nil, fmt.Errorf("secret loader is closed")
	}

	if secretKey == "" {
		return nil, fmt.Errorf("secret key cannot be empty")
	}

	if secret, exists := fsl.secrets.Get(secretKey); exists {
		return secret, nil // Return existing secret if already loaded
	}

	secretPath := filepath.Join(fsl.basePath, secretKey)

	// Check if file exists and read initial value
	//if _, err := os.Stat(secretPath); os.IsNotExist(err) {
	if _, err := fsl.reader.Stat(secretPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("secret file not found: %s", secretPath)
	}

	content, err := fsl.reader.ReadFile(secretPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret file %s: %w", secretPath, err)
	}

	result := &fileSecret{
		ctx:            fsl.ctx,
		id:             secretKey,
		path:           secretPath,
		reader:         fsl.reader,
		watcherFactory: fsl.watcherFactory,
		value: ConcurrentValue[string]{
			value: string(content),
		},
		watcher: ConcurrentValue[FileWatcher]{
			value: fsl.watcher,
		},
		subscribers: ConcurrentList[subscriberInfo]{},
		closed:      ConcurrentValue[bool]{},
		err:         ConcurrentValue[error]{},
	}

	fsl.secrets.Set(secretKey, result)
	return result, nil
}

// GetBasePath returns the current base path (useful for testing/debugging)
func (fsl *fileSecretLoader) GetBasePath() string {
	return fsl.basePath
}

func (fsl *fileSecretLoader) Close() {
	fsl.closeOnce.Do(func() {
		fsl.isClosed.Set(true)
		// signal startWatching loop to exit
		fsl.cancelCtxFn()

		for k, v := range fsl.secrets.CopyMap() {
			v.Close()          // Close each secret to release resources
			fsl.secrets.Del(k) // Remove from the loader's map
		}

		if fsl.watcher != nil {
			if err := fsl.watcher.Close(); err != nil {
				fsl.setError(fmt.Errorf("failed to close watcher: %w", err))
			}
		}
	})
}

func (fsl *fileSecretLoader) startWatching() error {
	err := fsl.watcher.Add(fsl.basePath)
	if err != nil {
		return fmt.Errorf("failed to add base path to watcher: %w", err)
	}
	go func() {
		defer fsl.Close()
		for {
			select {
			case event, isOpen := <-fsl.watcher.Events():
				if !isOpen {
					return
				}

				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) {
					fsl.handleFileChange(event.String())
				}

			case errW, isOpen := <-fsl.watcher.Errors():
				if !isOpen {
					return
				}
				fsl.setError(errW)
				return

			case <-fsl.ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (fsl *fileSecretLoader) handleFileChange(filePath string) {
	for k, fs := range fsl.secrets.CopyMap() {
		if strings.Contains(filePath, k) {
			fs.handleFileChange()
		}
	}
}

func (fsl *fileSecretLoader) setError(err error) {
	fsl.err.Set(err)
}

func (fsl *fileSecretLoader) Err() error {
	return fsl.err.Get()
}
