package secrets_test

import (
	"context"
	"testing"
	"time"

	"github.com/stable-io/commons-go/secrets"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stable-io/commons-go/secrets/mocks"
)

func TestSecretLoader_GetSecret(t *testing.T) {
	tests := []struct {
		name          string
		secretKey     string
		initialValue  string
		setupMock     func(*mocks.MockFileSystem)
		expectedError bool
	}{
		{
			name:         "successful secret load",
			secretKey:    "test-secret",
			initialValue: "secret-value",
			setupMock: func(mfs *mocks.MockFileSystem) {
				mfs.WriteFile("/mnt/secrets_store/test-secret", []byte("secret-value"))
			},
			expectedError: false,
		},
		{
			name:      "non-existent secret",
			secretKey: "non-existent",
			setupMock: func(mfs *mocks.MockFileSystem) {
				// No file created
			},
			expectedError: true,
		},
		{
			name:      "empty secret key",
			secretKey: "",
			setupMock: func(mfs *mocks.MockFileSystem) {
				// No setup needed
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mfs := mocks.NewMockFileSystem()
			defer mfs.Close()

			tt.setupMock(mfs)

			mockWatcherFactory := mocks.NewMockWatcherFactory()

			loader, err := secrets.NewFileSecretLoader(
				context.Background(),
				secrets.WithBasePath("/mnt/secrets_store"),
				secrets.WithWatcherFactory(mockWatcherFactory),
				secrets.WithFileReader(mfs),
			)
			require.NoError(t, err)
			defer loader.Close()

			// Test
			secret, err := loader.GetSecret(tt.secretKey)

			// Assert
			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, secret)
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
					return
				}
				assert.NotNil(t, secret)
				assert.Equal(t, tt.initialValue, secret.Value())
			}
		})
	}
}

func TestSecret_ListenChanges(t *testing.T) {
	// Setup
	mfs := mocks.NewMockFileSystem()
	defer mfs.Close()

	mwf := mocks.NewMockWatcherFactory()

	loader, err := secrets.NewFileSecretLoader(
		context.Background(),
		secrets.WithBasePath("/mnt/secrets_store"),
		secrets.WithFileReader(mfs),
		secrets.WithWatcherFactory(mwf),
	)
	require.NoError(t, err)
	defer loader.Close()

	// Create initial secret
	mfs.WriteFile("/mnt/secrets_store/test-secret", []byte("initial-value"))
	secret, err := loader.GetSecret("test-secret")
	require.NoError(t, err)

	// Test change notification
	changes, err := secret.ListenChanges()
	require.NoError(t, err)

	// Update secret value
	mfs.WriteFile("/mnt/secrets_store/test-secret", []byte("new-value"))
	mwf.GetWatcher().SimulateWrite("/mnt/secrets_store/test-secret")

	// Wait for change notification with timeout
	select {
	case newValue := <-changes:
		assert.Equal(t, "new-value", newValue)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for secret change notification")
	}
}

func TestSecretLoader_Close(t *testing.T) {
	// Setup
	mfs := mocks.NewMockFileSystem()
	defer mfs.Close()

	mwf := mocks.NewMockWatcherFactory()

	loader, err := secrets.NewFileSecretLoader(
		context.Background(),
		secrets.WithBasePath("/mnt/secrets_store"),
		secrets.WithFileReader(mfs),
		secrets.WithWatcherFactory(mwf),
	)
	require.NoError(t, err)

	// Create secret
	mfs.WriteFile("/mnt/secrets_store/test-secret", []byte("test-value"))
	secret, err := loader.GetSecret("test-secret")
	require.NoError(t, err)

	// Get change channel
	changes, err := secret.ListenChanges()
	require.NoError(t, err)

	// Close loader
	loader.Close()

	// Verify channel is closed
	select {
	case _, ok := <-changes:
		assert.False(t, ok, "channel should be closed")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for channel close")
	}

	// Verify subsequent GetSecret fails
	_, err = loader.GetSecret("test-secret")
	assert.Error(t, err)
}

func TestSecretLoader_ListSecretKeys(t *testing.T) {
	tests := []struct {
		name           string
		setupFiles     func(*mocks.MockFileSystem)
		expectedKeys   []string
		expectedLength int
		shouldClose    bool
		expectError    bool
	}{
		{
			name: "empty directory returns empty list",
			setupFiles: func(mfs *mocks.MockFileSystem) {
				// Create the base directory but no files
				mfs.CreateDir("/mnt/secrets_store")
			},
			expectedKeys:   []string{},
			expectedLength: 0,
			shouldClose:    false,
			expectError:    false,
		},
		{
			name: "directory with regular files",
			setupFiles: func(mfs *mocks.MockFileSystem) {
				mfs.WriteFile("/mnt/secrets_store/api-key", []byte("secret-api-key"))
				mfs.WriteFile("/mnt/secrets_store/database-password", []byte("db-password"))
				mfs.WriteFile("/mnt/secrets_store/jwt-token", []byte("jwt-secret"))
			},
			expectedKeys:   []string{"api-key", "database-password", "jwt-token"},
			expectedLength: 3,
			shouldClose:    false,
			expectError:    false,
		},
		{
			name: "directory with hidden files excludes them",
			setupFiles: func(mfs *mocks.MockFileSystem) {
				mfs.WriteFile("/mnt/secrets_store/api-key", []byte("secret-api-key"))
				mfs.WriteFile("/mnt/secrets_store/.hidden-file", []byte("hidden-content"))
				mfs.WriteFile("/mnt/secrets_store/..double-hidden", []byte("double-hidden"))
			},
			expectedKeys:   []string{"api-key"},
			expectedLength: 1,
			shouldClose:    false,
			expectError:    false,
		},
		{
			name: "directory with only hidden files returns empty",
			setupFiles: func(mfs *mocks.MockFileSystem) {
				mfs.WriteFile("/mnt/secrets_store/.hidden-file", []byte("hidden-content"))
				mfs.WriteFile("/mnt/secrets_store/.another-hidden", []byte("another-hidden"))
			},
			expectedKeys:   []string{},
			expectedLength: 0,
			shouldClose:    false,
			expectError:    false,
		},
		{
			name: "mixed file types filters correctly",
			setupFiles: func(mfs *mocks.MockFileSystem) {
				// Regular files (should be included)
				mfs.WriteFile("/mnt/secrets_store/api-key", []byte("secret-api-key"))
				mfs.WriteFile("/mnt/secrets_store/database-password", []byte("db-password"))
				// Hidden files (should be excluded)
				mfs.WriteFile("/mnt/secrets_store/.hidden-file", []byte("hidden-content"))
				mfs.WriteFile("/mnt/secrets_store/.gitignore", []byte("*.log"))
				// Subdirectories (should be excluded)
				mfs.CreateDir("/mnt/secrets_store/subdirectory")
				mfs.CreateDir("/mnt/secrets_store/another-dir")
			},
			expectedKeys:   []string{"api-key", "database-password"},
			expectedLength: 2,
			shouldClose:    false,
			expectError:    false,
		},
		{
			name: "closed loader returns error",
			setupFiles: func(mfs *mocks.MockFileSystem) {
				mfs.WriteFile("/mnt/secrets_store/test-secret", []byte("test-value"))
			},
			expectedKeys:   []string{},
			expectedLength: 0,
			shouldClose:    true,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mfs := mocks.NewMockFileSystem()
			defer mfs.Close()

			mockWatcherFactory := mocks.NewMockWatcherFactory()

			loader, err := secrets.NewFileSecretLoader(
				context.Background(),
				secrets.WithBasePath("/mnt/secrets_store"),
				secrets.WithWatcherFactory(mockWatcherFactory),
				secrets.WithFileReader(mfs),
			)
			require.NoError(t, err)
			defer loader.Close()

			// Setup test-specific files
			tt.setupFiles(mfs)

			// Close loader if test requires it
			if tt.shouldClose {
				loader.Close()
			}

			// Act
			keys, err := loader.ListSecretKeys()

			// Assert
			if tt.expectError {
				assert.Error(t, err)
				if tt.shouldClose {
					assert.Contains(t, err.Error(), "closed")
				}
				assert.Equal(t, tt.expectedKeys, keys)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, keys, "expected non-nil result for active loader")
				assert.Len(t, keys, tt.expectedLength, "unexpected number of keys")

				// For non-empty expected keys, verify all expected keys are present
				if len(tt.expectedKeys) > 0 {
					for _, expectedKey := range tt.expectedKeys {
						assert.Contains(t, keys, expectedKey, "expected key not found: %s", expectedKey)
					}
				}
			}
		})
	}
}

func TestSecretLoader_ListSecretKeys_DirectoryErrors(t *testing.T) {
	tests := []struct {
		name        string
		basePath    string
		expectedErr string
	}{
		{
			name:        "non-existent directory returns error",
			basePath:    "/non/existent/path",
			expectedErr: "failed to read secrets directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mfs := mocks.NewMockFileSystem()
			defer mfs.Close()

			mockWatcherFactory := mocks.NewMockWatcherFactory()

			loader, err := secrets.NewFileSecretLoader(
				context.Background(),
				secrets.WithBasePath(tt.basePath),
				secrets.WithWatcherFactory(mockWatcherFactory),
				secrets.WithFileReader(mfs),
			)
			require.NoError(t, err)
			defer loader.Close()

			// Act
			keys, err := loader.ListSecretKeys()

			// Assert
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
			assert.Empty(t, keys)
		})
	}
}
