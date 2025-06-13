package secrets_test

import (
	"context"
	"github.com/stable-io/commons-go/secrets"
	"testing"
	"time"

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
