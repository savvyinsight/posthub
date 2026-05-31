# Coding Standards

## Package Structure

### Directory Layout
```
internal/          # Private packages, unexported outside binary
├── config/        # Configuration loading and validation
├── contracts/     # Domain types and interfaces (shared)
├── logger/        # Structured logging
├── platform/      # Platform abstraction layer
├── publish/       # Publishing orchestration
├── queue/         # Queue abstraction
├── storage/       # Data persistence
└── transform/     # Transformation pipeline

pkg/               # Public packages (rarely used)

cmd/               # Binaries
├── api/           # API server
└── worker/        # Async worker
```

### Package Rules
1. **No circular dependencies** - Enforce via static analysis in CI
2. **No internal package leaks** - `internal/*` never imported from outside posthub binaries
3. **Shared contracts live in `contracts`** - All domain types defined once
4. **Dependencies flow downward only** - cmd → internal → contracts (acyclic)
5. **One responsibility per package** - Clear purpose, focused scope
6. **Testable in isolation** - Mock interfaces defined in same package

## Naming Conventions

### Files
- `*.go` - Implementation files
- `*_test.go` - Unit tests in same package
- Snake case: `config.go`, `config_test.go`, `publish_test.go`

### Types
- `PascalCase` exported
- `camelCase` unexported
- Descriptors before generic names: `UserStore` not `Store`, `RedisQueue` not `Queue`
- Interfaces are minimal and specific: `Publisher`, `Reader`, `Writer` not `Manager`, `Handler`

### Functions/Methods
- `PascalCase` exported
- `camelCase` unexported
- Action-oriented names: `Get`, `Store`, `Publish`, `Transform`, `Validate`
- Query methods return `(T, error)` not bool

### Constants
- `SCREAMING_SNAKE_CASE` for package-level constants
- Group related constants in blocks
- Use iota for sequences when appropriate

### Variables
- Short-lived local vars: `ctx`, `err`, `id`, `msg`
- Longer scope: more descriptive names
- Receivers are single letters: `func (s *Store) Get()`, `func (p Publisher) Publish()`

## Code Organization

### Imports
```go
import (
	"context"
	"errors"
	
	"github.com/user/posthub/internal/contracts"
	"github.com/user/posthub/internal/logger"
)
```

Order:
1. Standard library (blank line)
2. Third-party (blank line)
3. Internal posthub

### Variable Declaration
- `var` for package-level, never `const` for mutable intent
- Short form `:=` for local scope (except in tight loops)
- Blank identifier `_` only for unused imports, not unused values

### Interface Definition
```go
// Interfaces live where they are USED, not where they are implemented
type Publisher interface {
	Publish(ctx context.Context, content *Content) error
}

// Implementation in different package
type PostPublisher struct { ... }
func (p *PostPublisher) Publish(ctx context.Context, c *Content) error { ... }
```

## Error Handling

### Standard Errors in `contracts/errors.go`
- Define domain errors once
- Use `errors.Is()` for checking, never string matching
- Wrap errors with context: `fmt.Errorf("load config: %w", err)`
- Return `error` interface, not concrete types

### Error Types
```go
// Domain-specific error
var ErrPublishNotFound = errors.New("publish not found")

// Implementation: caller defines what error means
func (s *Store) Get(id string) (*Publish, error) {
	if id == "" {
		return nil, fmt.Errorf("get publish: %w", ErrValidationFailed)
	}
	// ...
}
```

### Pattern
- Never panic in production code
- Always return error, let caller decide
- Wrap with context, don't re-wrap
- Use typed errors only in contracts, not internal details

## Logging Standards

### Rules
1. Use structured logging via `logger` package only
2. No `fmt.Print*` in production code
3. Every error logged with context
4. No sensitive data (passwords, API keys, tokens)
5. Debug-level for internal state, info for state changes

### Pattern
```go
// Good
logger.Info("publish created", 
	logger.String("publish_id", id),
	logger.String("platform", platform))

// Bad
logger.Info(fmt.Sprintf("publish created: %s", id))
fmt.Println("debug:", value)
```

## Comments

### When to Write
- **Why**, not what (code shows what)
- Complex algorithms or non-obvious logic
- Public API contracts
- Package-level godoc

### What to Avoid
- Commented-out code (use git history)
- Obvious comments: `x := 1 // set x to 1`
- TODOs without context

### Format
```go
// Store handles persistent storage operations.
type Store interface {
	Get(ctx context.Context, id string) (*Publish, error)
}

// validateURL ensures the URL is parseable and not localhost.
func validateURL(u string) error {
	// ...
}
```

## Testing Patterns

See `testing-strategy.md`

## Forbidden Patterns

1. ❌ Global variables (except constants)
2. ❌ `init()` functions for setup (use explicit constructors)
3. ❌ Interfaces with >3 methods (sign of mixing concerns)
4. ❌ Interfaces that return interfaces (pass through concrete types)
5. ❌ Nil receivers (always check or use pointers)
6. ❌ Interface{} in public APIs (be specific)
7. ❌ Goroutines started without context/lifecycle awareness
8. ❌ Direct platform calls outside `platform/` package
9. ❌ SQL queries as strings (use query builders or parameterized)
10. ❌ Logging inside business logic (return errors instead)

## Code Review Checklist

- [ ] Interfaces defined at point of use
- [ ] Errors wrapped with context
- [ ] No circular dependencies introduced
- [ ] Logging used correctly (structured, no sensitive data)
- [ ] Tests written for new code
- [ ] Comments explain why, not what
- [ ] Naming is descriptive and consistent
- [ ] No global mutable state
- [ ] Package boundary respected
