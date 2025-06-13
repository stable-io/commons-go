# Secrets Package

[![Tests](https://github.com/stable-io/commons-go/actions/workflows/secrets-tests.yml/badge.svg)](https://github.com/stable-io/commons-go/actions/workflows/secrets-tests.yml)
[![Coverage](https://codecov.io/gh/stable-io/commons-go/branch/main/graph/badge.svg?flag=secrets)](https://codecov.io/gh/stable-io/commons-go)

Go package for managing and watching file-based secrets with real-time change notifications.


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

By default, the package watches and loads secrets from `/mnt/secrets_store/`. Each secret should be a separate file:

```
/mnt/secrets_store/
├── my-secret
├── api-key
└── database-password
```

You can customize this path using the `WithBasePath` option when creating a new loader.

## Error Handling

The package provides error information through the `Err()` method:

```go
if err := secret.Err(); err != nil {
    log.Printf("Secret error: %v", err)
}
```