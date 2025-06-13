# Secrets Package

A Go package for managing and watching file-based secrets with real-time change notifications.

## Overview

The secrets package provides a robust solution for managing secrets stored in files. It offers:

- File-based secret storage
- Real-time secret change notifications
- Thread-safe concurrent access
- Resource cleanup and error handling
- Configurable file system and watcher implementations

## Installation

```bash
go get github.com/stable-io/commons-go/secrets
```

## Usage

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/stable-io/commons-go/secrets"
)

func main() {
    // Create a new secret loader
    loader, err := secrets.NewFileSecretLoader(context.Background())
    if err != nil {
        log.Fatal(err)
    }
    defer loader.Close()

    // Load a secret
    secret, err := loader.GetSecret("my-secret")
    if err != nil {
        log.Fatal(err)
    }

    // Get the secret value
    value := secret.Value()
    fmt.Printf("Secret value: %s\n", value)

    // Listen for changes
    changes, err := secret.ListenChanges()
    if err != nil {
        log.Fatal(err)
    }

    // Handle secret changes
    go func() {
        for newValue := range changes {
            fmt.Printf("Secret updated: %s\n", newValue)
        }
    }()
}
```

### Configuration Options

The secret loader can be configured using functional options:

```go
loader, err := secrets.NewFileSecretLoader(
    context.Background(),
    secrets.WithBasePath("/custom/path"),           // Custom base path for secrets
)
```


## File Structure

By default, secrets are expected to be in `/mnt/secrets_store/`. Each secret should be a separate file:

```
/mnt/secrets_store/
├── my-secret
├── api-key
└── database-password
```

## Error Handling

The package provides error information through the `Err()` method:

```go
if err := secret.Err(); err != nil {
    log.Printf("Secret error: %v", err)
}
```