# Testing Strategy

## Testing Pyramid

```
        /\
       /  \        End-to-end (rare, only system boundaries)
      /────\       1-2 tests per critical flow
     /      \
    /────────\     Integration (after main merges)
   /  tables  \    Test component interaction in isolation
  /____________\

  /────────────\   Unit Tests (majority, written by session)
 /   isolated  \   70-80% of test suite
/______________\  Fast, deterministic, focused
```

## Unit Testing

### Test File Location
```
internal/
  └── storage/
      ├── storage.go           # Implementation
      └── storage_test.go      # Tests in same package
```

### Test Naming
```go
func TestStore_Get(t *testing.T) { ... }           // Happy path
func TestStore_Get_NotFound(t *testing.T) { ... }  // Specific case
func TestStore_Get_InvalidID_Error(t *testing.T) { ... }  // Error
```

Pattern: `Test{Type}_{Method}_{Scenario}`

### Table-Driven Tests (Preferred)
```go
func TestURLValidator_Validate(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
		errMsg  string
	}{
		{name: "valid_url", url: "https://example.com/path", wantErr: false},
		{name: "invalid_scheme", url: "ftp://example.com", wantErr: true, errMsg: "scheme must be https"},
		{name: "localhost_rejected", url: "https://localhost:8080", wantErr: true, errMsg: "localhost not allowed"},
		{name: "empty_url", url: "", wantErr: true, errMsg: "url required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("got error: %v, want error: %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("got error message: %q, want containing: %q", err.Error(), tt.errMsg)
			}
		})
	}
}
```

### Assertion Pattern
- **Don't use assertion libraries** (plain `if` + `t.Errorf`)
- Clear error messages help debugging
- Show both actual and expected

### Test Structure
```go
func TestPublisher_Publish(t *testing.T) {
	// Arrange
	ctx := context.Background()
	pub := &Publisher{store: fakeStore}
	content := &Content{ID: "123", Platform: "twitter"}

	// Act
	err := pub.Publish(ctx, content)

	// Assert
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fakeStore.Called {
		t.Error("Store.Save not called")
	}
}
```

### Error Cases
```go
func TestPublisher_Publish_StoreError(t *testing.T) {
	ctx := context.Background()
	pub := &Publisher{store: &failStore{err: errors.New("db error")}}
	content := &Content{ID: "123"}

	err := pub.Publish(ctx, content)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, storage.ErrStore) {
		t.Errorf("got error: %v, want: %v", err, storage.ErrStore)
	}
}
```

## Mocking

### Mock Interface Pattern
```go
// contracts/storage.go (where interface is defined)
type Store interface {
	Get(ctx context.Context, id string) (*Publish, error)
	Save(ctx context.Context, p *Publish) error
}

// storage_test.go (same package, test-only)
type fakeStore struct {
	getResult *Publish
	getErr    error
	saveCalls int
}

func (f *fakeStore) Get(ctx context.Context, id string) (*Publish, error) {
	return f.getResult, f.getErr
}

func (f *fakeStore) Save(ctx context.Context, p *Publish) error {
	f.saveCalls++
	return nil
}

// Test that uses it
func TestPublisher_CallsStore(t *testing.T) {
	fake := &fakeStore{getResult: &Publish{ID: "123"}}
	pub := &Publisher{store: fake}

	pub.Publish(context.Background(), &Content{ID: "123"})

	if fake.saveCalls != 1 {
		t.Errorf("expected 1 save call, got %d", fake.saveCalls)
	}
}
```

### Mock Guidelines
- **Don't mock:** Concrete types you're testing (defeats purpose)
- **Do mock:** External dependencies (database, API, queue)
- **Fakes over mocks:** Prefer simple fake implementations
- **One fake per test file:** Name clearly (`fakeStore`, `fakePublisher`)

## Coverage Requirements

### Minimum Coverage
- **New code:** 70% coverage
- **Modified code:** No coverage decrease
- **Critical paths:** 100% (error handling, state changes)

### What to Cover
✅ Happy path (at least one)
✅ Error cases (one test per error type)
✅ Boundary conditions (empty, max, nil)
✅ State transitions (before/after effects)

### What NOT to Cover
❌ Standard library code (trust Go)
❌ Generated code
❌ Mocks themselves
❌ `init()` functions (don't use them)

## Context Usage

### Always Pass Context
```go
func (s *Store) Get(ctx context.Context, id string) (*Publish, error) {
	// Use ctx for timeouts, cancellation
	return s.db.QueryRowContext(ctx, "SELECT ...").Scan()
}
```

### Testing with Context
```go
func TestStore_Get_WithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Slow operation
	store := &Store{db: slowDB}
	_, err := store.Get(ctx, "123")

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected timeout error, got: %v", err)
	}
}
```

## Dependency Injection in Tests

### Constructor Pattern
```go
// Implementation
type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Testing
func TestStore_Integration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	store := NewStore(db)
	// Test real behavior
}

func TestStore_Unit(t *testing.T) {
	fakeDB := &fakeDB{}
	store := &Store{db: fakeDB}
	// Test with mock
}
```

### Don't Use
- ❌ Global variables for test setup
- ❌ `init()` functions
- ❌ Test package interfaces different from production

## Concurrency Testing

### For Goroutine Code
```go
func TestQueue_Concurrent(t *testing.T) {
	q := NewQueue()
	var wg sync.WaitGroup
	errs := make(chan error, 100)

	// 10 goroutines publishing
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := q.Enqueue(&Job{ID: fmt.Sprintf("%d", id)})
			if err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("unexpected error: %v", err)
	}

	count := q.Len()
	if count != 10 {
		t.Errorf("expected 10 jobs, got %d", count)
	}
}
```

### Rules
- Use `sync.WaitGroup` for goroutine coordination
- Use channels for result collection
- Always close channels
- Check for race conditions: `go test -race`

## Integration Test Guidelines

### When to Write
- After main branch merge
- Component interaction tested
- Database queries exercised
- Real network calls (mocked)

### Location
```
test/
  └── integration_test.go
```

### Example
```go
// test/integration_test.go
func TestPublish_EndToEnd(t *testing.T) {
	// Setup real components
	db := setupTestDB(t)
	defer db.Close()
	
	store := storage.NewPostgresStore(db)
	publisher := publish.NewPublisher(store)
	
	// Execute workflow
	content := &contracts.Content{
		ID: "test-123",
		Body: "Hello world",
		Platform: "twitter",
	}
	
	err := publisher.Publish(context.Background(), content)
	
	// Assert end result
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	
	// Verify stored
	stored, _ := store.Get(context.Background(), "test-123")
	if stored.Body != "Hello world" {
		t.Errorf("content not stored correctly")
	}
}
```

## Benchmark Tests (Optional)

### When Needed
- Performance-critical paths
- Before and after optimization
- Only after functionality verified

### Pattern
```go
func BenchmarkPublisher_Publish(b *testing.B) {
	pub := NewPublisher(store)
	content := &Content{ID: "123"}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		pub.Publish(context.Background(), content)
	}
}
```

## Test Data Builders

### For Complex Objects
```go
type contentBuilder struct {
	id       string
	body     string
	platform string
}

func newContentBuilder() *contentBuilder {
	return &contentBuilder{
		id:       "test-id",
		body:     "default body",
		platform: "twitter",
	}
}

func (b *contentBuilder) withID(id string) *contentBuilder {
	b.id = id
	return b
}

func (b *contentBuilder) build() *Content {
	return &Content{
		ID:       b.id,
		Body:     b.body,
		Platform: b.platform,
	}
}

// Usage
content := newContentBuilder().withID("custom-id").build()
```

## Test Execution

### Run Tests
```bash
make test           # All tests
make test-short     # Quick tests only
make test-verbose   # With output
go test ./...       # Current package and all sub-packages
```

### Coverage Report
```bash
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Race Detection
```bash
go test -race ./...
```

## Test Review Checklist

- [ ] Tests are focused (one thing per test)
- [ ] Table-driven tests used for multiple cases
- [ ] Error cases tested explicitly
- [ ] Mocks are simple and clear
- [ ] No test interdependencies
- [ ] Assertions are clear with good error messages
- [ ] Context used correctly
- [ ] No flaky tests (run 3+ times)
- [ ] Coverage ≥ 70% on new code
- [ ] Test names describe the scenario
