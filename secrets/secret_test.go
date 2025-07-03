package secrets_test

import (
	"context"
	"fmt"
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
		setupSecrets   func(*mocks.MockFileSystem, secrets.SecretLoader)
		expectedKeys   []string
		expectedLength int
		shouldClose    bool
	}{
		{
			name: "empty loader returns empty list",
			setupSecrets: func(mfs *mocks.MockFileSystem, loader secrets.SecretLoader) {
				// No secrets loaded
			},
			expectedKeys:   []string{},
			expectedLength: 0,
			shouldClose:    false,
		},
		{
			name: "single secret loaded",
			setupSecrets: func(mfs *mocks.MockFileSystem, loader secrets.SecretLoader) {
				mfs.WriteFile("/mnt/secrets_store/api-key", []byte("secret-api-key"))
				_, err := loader.GetSecret("api-key")
				require.NoError(t, err)
			},
			expectedKeys:   []string{"api-key"},
			expectedLength: 1,
			shouldClose:    false,
		},
		{
			name: "multiple secrets loaded",
			setupSecrets: func(mfs *mocks.MockFileSystem, loader secrets.SecretLoader) {
				// Setup multiple secrets
				secrets := map[string]string{
					"api-key":           "secret-api-key",
					"database-password": "db-password",
					"jwt-token":         "jwt-secret",
				}
				for key, value := range secrets {
					mfs.WriteFile("/mnt/secrets_store/"+key, []byte(value))
					_, err := loader.GetSecret(key)
					require.NoError(t, err)
				}
			},
			expectedKeys:   []string{"api-key", "database-password", "jwt-token"},
			expectedLength: 3,
			shouldClose:    false,
		},
		{
			name: "closed loader returns nil",
			setupSecrets: func(mfs *mocks.MockFileSystem, loader secrets.SecretLoader) {
				// Setup a secret first
				mfs.WriteFile("/mnt/secrets_store/test-secret", []byte("test-value"))
				_, err := loader.GetSecret("test-secret")
				require.NoError(t, err)
			},
			expectedKeys:   nil,
			expectedLength: 0,
			shouldClose:    true,
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

			// Setup test-specific secrets
			tt.setupSecrets(mfs, loader)

			// Close loader if test requires it
			if tt.shouldClose {
				loader.Close()
			}

			// Act
			keys := loader.ListSecretKeys()

			// Assert
			if tt.expectedKeys == nil {
				assert.Nil(t, keys, "expected nil result for closed loader")
			} else {
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

func TestSecretLoader_ListSecretKeys_ConcurrentAccess(t *testing.T) {
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

	// Setup initial secrets
	initialSecrets := []string{"secret1", "secret2", "secret3"}
	for _, key := range initialSecrets {
		mfs.WriteFile("/mnt/secrets_store/"+key, []byte("value-"+key))
		_, err := loader.GetSecret(key)
		require.NoError(t, err)
	}

	// Act - Test concurrent access
	const numGoroutines = 10
	const numOperations = 100

	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines*numOperations)

	// Launch multiple goroutines that simultaneously call ListSecretKeys
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer func() { done <- true }()

			for j := 0; j < numOperations; j++ {
				keys := loader.ListSecretKeys()

				// Verify basic invariants
				if keys == nil {
					errors <- fmt.Errorf("goroutine %d, iteration %d: got nil keys", goroutineID, j)
					continue
				}

				if len(keys) != len(initialSecrets) {
					errors <- fmt.Errorf("goroutine %d, iteration %d: expected %d keys, got %d",
						goroutineID, j, len(initialSecrets), len(keys))
					continue
				}

				// Verify all expected keys are present
				for _, expectedKey := range initialSecrets {
					found := false
					for _, key := range keys {
						if key == expectedKey {
							found = true
							break
						}
					}
					if !found {
						errors <- fmt.Errorf("goroutine %d, iteration %d: missing key %s",
							goroutineID, j, expectedKey)
						break
					}
				}
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Assert - Check for any errors
	close(errors)
	var errorList []error
	for err := range errors {
		errorList = append(errorList, err)
	}

	if len(errorList) > 0 {
		t.Errorf("Concurrent access test failed with %d errors:", len(errorList))
		for _, err := range errorList {
			t.Errorf("  - %v", err)
		}
	}
}
