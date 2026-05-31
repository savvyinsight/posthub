# Module Ownership & Boundaries

Defines package responsibilities, ownership, dependencies, and version strategy for the modular monolith.

## Package Ownership Matrix

| Package | Responsibility | Owner | Public API | Depends On |
|---------|-----------------|-------|-----------|-----------|
| `contracts` | Domain types, interfaces, shared errors | Shared | ✓ All types | Nothing |
| `config` | Configuration loading and validation | DevOps/Infra | ✓ Config struct | Nothing |
| `logger` | Structured logging wrapper | Shared | ✓ Log functions | Nothing |
| `platform` | Platform abstraction and registry | AI Session: Platform | ✓ Registry interface | contracts, config |
| `storage` | Data persistence (PostgreSQL) | AI Session: Storage | ✓ Store interface | contracts, config |
| `queue` | Job queue abstraction (Redis) | AI Session: Queue | ✓ Queue interface | contracts, config |
| `transform` | Content transformation pipeline | AI Session: Transform | ✓ Pipeline interface | contracts |
| `publish` | Publishing orchestration | AI Session: Publish | ✓ Publisher interface | contracts, storage, transform, queue, logger |

## Dependency Direction (Strict Acyclic)

```
cmd/api     cmd/worker
    ↓           ↓
    →─→─→ publish ←─←─←
           ↓ ↓ ↓
        storage  transform  queue  platform  logger
           ↓      ↓          ↓        ↓       ↓
           →──→──→ contracts ←──←──←──←       ↑
                        ↓
                   Nothing (bottom)
```

### Rules
1. **Never import upward** - `contracts` cannot import from any package
2. **Never import sideways** - `storage` cannot import from `queue`
3. **Never create cycles** - If A imports B, B cannot import A
4. **Leaf packages depend on foundation** - `transform` → `contracts`
5. **Orchestration at top** - `publish` imports from `storage`, `queue`, `transform`

### Enforced in CI
- Static analysis tool checks imports
- Fails if circular dependency detected
- PR blocked if rule violated

## Layer Definitions

### Layer 0: Foundation
**Packages:** `contracts`, `config`, `logger`
- No dependencies (except stdlib)
- Always safe to import
- Fundamental to all other packages
- Changes require careful review

### Layer 1: Infrastructure
**Packages:** `platform`, `storage`, `queue`
- Depend only on Layer 0
- External system abstraction
- Concrete implementations
- Changes affect multiple layers

### Layer 2: Business Logic
**Packages:** `transform`
- Depends on Layer 0 and Layer 1 interfaces
- Domain-specific logic
- Testable in isolation
- Most changes here

### Layer 3: Orchestration
**Packages:** `publish`
- Depends on all layers
- Coordinates workflow
- Cannot be imported by other packages
- Changes affect overall flow

### Commands
**Packages:** `cmd/api`, `cmd/worker`
- Depend on anything
- Entry points only
- No shared logic between commands
- Each binary self-contained

## Package Interface Contracts

### contracts/ (Foundation)
**Public Types:**
```go
type Content struct { ... }
type Publish struct { ... }
type PublishState string
type Platform string
type platformCapability string
```

**Public Interfaces:**
- None (interfaces defined at use point)

**Public Functions:**
- Error constructors: `var ErrNotFound error`

**When to Modify:**
- Adding new domain type
- Extending error set
- Changing core structure (rare)

**Review:** All modifications require explicit review

### config/ (Foundation)
**Public Types:**
- `Config` - Main configuration struct
- `DatabaseConfig`, `RedisConfig`, `LoggerConfig`

**Public Functions:**
- `LoadConfig() (*Config, error)` - From environment/file

**When to Modify:**
- Adding new configuration option
- Changing validation rules

### logger/ (Foundation)
**Public Functions:**
- `Info()`, `Debug()`, `Warn()`, `Error()`
- `String()`, `Int()`, `Bool()` - Field builders

**When to Modify:**
- Adding new log level (rare)
- Adding new field builder

### platform/ (Infrastructure)
**Public Interfaces:**
```go
type Platform interface {
	Capabilities() []Capability
	Authenticate(ctx context.Context, creds *Credentials) error
	Upload(ctx context.Context, asset *Asset) (string, error)
	Publish(ctx context.Context, content *PublishedContent) (string, error)
}
```

**Public Types:**
- `Capability`
- `PlatformRegistry` - Lookup platform implementations

**Internal Implementation:**
- `TwitterPlatform`, `LinkedInPlatform` (mocks only in foundation phase)

**When to Import:**
- From `publish` package only
- Call via registry, not directly

**When to Modify:**
- Adding new platform capability
- Adding new platform type

### storage/ (Infrastructure)
**Public Interfaces:**
```go
type Store interface {
	Get(ctx context.Context, id string) (*Publish, error)
	Save(ctx context.Context, p *Publish) error
	Query(ctx context.Context, filters map[string]interface{}) ([]*Publish, error)
	Delete(ctx context.Context, id string) error
}
```

**Internal Implementation:**
- `PostgresStore` - Real database
- `InMemoryStore` - Testing/development

**When to Import:**
- From `publish` package only

**When to Modify:**
- Changing query strategy
- Adding new query method
- Changing persistence format

### queue/ (Infrastructure)
**Public Interfaces:**
```go
type Queue interface {
	Enqueue(ctx context.Context, job *Job) error
	Dequeue(ctx context.Context) (*Job, error)
	Ack(ctx context.Context, jobID string) error
	Nack(ctx context.Context, jobID string) error
}
```

**Internal Implementation:**
- `RedisQueue` - Real queue
- `InMemoryQueue` - Testing

**When to Import:**
- From `publish` package only

**When to Modify:**
- Adding new job type
- Changing retry strategy

### transform/ (Business Logic)
**Public Interfaces:**
```go
type Transformer interface {
	Transform(ctx context.Context, content *Content) (*TransformedContent, error)
}

type Pipeline interface {
	Execute(ctx context.Context, content *Content) ([]*TransformedContent, error)
}
```

**Dependencies:**
- Can depend on all other packages
- Does NOT depend on `publish`

**When to Modify:**
- Adding new transformation step
- Changing transformation logic
- Optimizing pipeline

### publish/ (Orchestration)
**Public Interfaces:**
```go
type Publisher interface {
	Publish(ctx context.Context, content *Content) error
	GetStatus(ctx context.Context, publishID string) (PublishState, error)
}
```

**Dependencies:**
- `storage.Store`
- `queue.Queue`
- `transform.Pipeline`
- `platform.Platform` (via registry)
- `logger`

**When to Modify:**
- Changing publication workflow
- Adding new state
- Changing orchestration logic

**Never Import From:**
- `publish` should not be imported from infrastructure packages

## Ownership Responsibilities

### Shared Packages (contracts, config, logger)
**Owner:** Engineering Lead / All Sessions

**Responsibilities:**
- Changes require consensus
- Backward compatibility maintained
- Clear versioning
- Documentation up-to-date

**Release Process:**
1. Plan change in GitHub issue
2. Discuss impact on dependent packages
3. Update documentation
4. Implement with new version
5. Merge to main
6. Tag version

### Infrastructure Packages (platform, storage, queue)
**Owner:** Assigned AI Session

**Responsibilities:**
- Implement interfaces from contracts
- Mock implementations for foundation phase
- Unit tests for all public methods
- Error handling consistent
- Structured logging in operations

**Review:**
- Code review by another session
- Interface compliance verified
- Tests ≥70% coverage
- No circular dependencies

### Business Logic (transform)
**Owner:** Assigned AI Session

**Responsibilities:**
- Complex transformation logic
- Testable algorithms
- Production-grade performance
- Clear error messages

**Review:**
- Algorithm correctness
- Edge case handling
- Performance (if relevant)
- Integration with pipeline

### Orchestration (publish)
**Owner:** Assigned AI Session

**Responsibilities:**
- Workflow correctness
- State management
- Error recovery
- Logging of state changes

**Review:**
- All paths tested
- State transitions valid
- No deadlocks
- Idempotency verified

### Commands (cmd/api, cmd/worker)
**Owner:** DevOps / Infrastructure

**Responsibilities:**
- Wiring dependencies
- Configuration loading
- Graceful shutdown
- Health checks

## Versioning Strategy

### Semantic Versioning: v{MAJOR}.{MINOR}.{PATCH}
- **MAJOR:** Architecture change, breaking interface change
- **MINOR:** New feature, backward-compatible enhancement
- **PATCH:** Bug fix, no behavior change

### Current Phase
- **v0.x.x** - Foundation engineering
- No guarantees of backward compatibility
- Focus on correctness and testability

### Version Bumping
- New infrastructure package: v0.{minor++}.0
- New business logic: v0.{minor}.{patch++}
- Critical bug fix: v0.{minor}.{patch++}

### Release Checklist
- [ ] All tests passing
- [ ] Coverage maintained/improved
- [ ] Documentation updated
- [ ] Changelog entry
- [ ] Tag created and pushed
- [ ] Deployment documentation updated

## Boundary Enforcement

### At Package Level
```go
// storage/storage.go - Only public types and interfaces
type Store interface { ... }

// storage/postgres.go - Internal implementation (unexported)
type postgresStore struct { ... }

// storage/postgres_test.go - Tests in same package
func TestPostgresStore_Get(t *testing.T) { ... }
```

### At Directory Level
```
internal/
├── storage/
│   ├── storage.go        # Public interface only
│   ├── postgres.go       # Implementation (unexported)
│   ├── inmemory.go       # Test implementation
│   └── storage_test.go   # Tests
```

### At Import Level
```go
// ALLOWED
import "github.com/user/posthub/internal/storage"

// FORBIDDEN (internal packages cannot be imported from outside)
import "github.com/user/posthub/internal/storage/postgres"
```

## Cross-Package Communication

### Communication Points
1. **Via Shared Interfaces** (publish → storage)
   ```go
   store := storage.NewPostgresStore(config)
   publish := publish.NewPublisher(store)  // store satisfies storage.Store
   ```

2. **Via Contracts** (all packages)
   ```go
   import "github.com/user/posthub/internal/contracts"
   
   func Process(content *contracts.Content) { ... }
   ```

3. **Via Registry** (publish → platform)
   ```go
   registry := platform.NewRegistry()
   platform := registry.Get("twitter")
   ```

### Forbidden Communication
- ❌ Direct import of private packages
- ❌ Casting interfaces to concrete types
- ❌ Reaching into another package's internals
- ❌ Circular imports

## Modification Workflow

### To Add New Method to Interface
1. Identify the interface location (where used)
2. Create PR with interface change
3. All implementations update in same PR
4. Tests added for new method
5. Review and merge

### To Add New Package
1. Ensure it fits in dependency graph
2. Define public interface/types only
3. Plan owner assignment
4. Document in this file
5. Add to CI checks
6. Merge with documentation

### To Refactor Package
1. Identify scope (internal only, no interface change)
2. Keep interface stable
3. All tests must still pass
4. Performance not degraded
5. Merge with no version bump

## Maintenance Checklist

- [ ] No circular dependencies
- [ ] Interfaces at point of use
- [ ] Public API documented
- [ ] Tests for public methods
- [ ] Errors wrapped with context
- [ ] Logging structured
- [ ] No global mutable state
- [ ] Package responsibility clear
- [ ] Version requirements met
- [ ] Documentation up-to-date
