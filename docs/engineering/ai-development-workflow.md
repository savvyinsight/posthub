# AI-Assisted Development Workflow

Optimized for multi-session collaboration with AI agents (e.g., GitHub Copilot) in a modular monolith.

## Session Structure

### Session Ownership
Each Copilot session owns specific packages/modules:
- **Session A:** `internal/platform/*`
- **Session B:** `internal/storage/*`
- **Session C:** `internal/transform/*`

**Rule:** Different sessions should NOT modify the same file simultaneously.

### Session Start
1. AI reviews `docs/engineering/*` to understand architecture
2. AI reviews owned package interfaces in `contracts/*`
3. AI reviews recent commits in owned scope
4. AI documents assumptions in task description

### Session End
1. All code merged to feature branch
2. PR created with clear description
3. Tests passing locally
4. No unresolved conversations

## Ownership Boundaries

### Forbidden Cross-Package Changes
❌ Modify another session's package without explicit coordination
❌ Change shared interfaces in `contracts/` without discussion
❌ Reorganize package structure
❌ Rename exported functions/types without review

### Safe to Modify
✅ Internal implementation of owned package
✅ Tests within owned package
✅ Add new interfaces to `contracts/` (if needed, document in PR)
✅ Expand validation in `contracts/errors.go`
✅ Configuration changes in `internal/config/`

## AI Constraints

### Must Verify
- [ ] No circular dependencies introduced (check imports)
- [ ] Package boundary respected
- [ ] Interfaces used at point of definition
- [ ] Tests written for all exported functions
- [ ] Errors wrapped with context
- [ ] No global state

### Must Not Do
- ❌ Implement real platform integrations (mock only)
- ❌ Modify authentication/authorization logic
- ❌ Call external APIs
- ❌ Implement frontend or UI
- ❌ Add AI/ML features
- ❌ Modify unrelated packages
- ❌ Skip tests
- ❌ Use framework magic (explicit code only)

## Context Maintenance

### What AI Should Keep in Memory
- Package responsibilities
- Shared interfaces from `contracts/`
- Dependency direction rules
- Testing patterns
- Naming conventions
- Error handling strategy

### What AI Should Ask About
- Architecture changes
- New package structure
- Shared interface modifications
- Cross-package coordination
- Scope changes

## Code Review for AI Sessions

### AI-Generated Code Review Focus
1. **Imports Check** - No circular dependencies?
2. **Interface Compliance** - All methods implemented?
3. **Test Coverage** - All public functions tested?
4. **Error Handling** - Errors wrapped with context?
5. **Package Boundary** - No leaking into other packages?
6. **Naming** - Follows conventions?
7. **Logging** - Structured, no sensitive data?

### Specific Checks for AI Code
- [ ] Comments explain why, not what
- [ ] No `interface{}` in public APIs
- [ ] Interfaces are minimal (≤3 methods typically)
- [ ] Constructor functions explicit (no `init()`)
- [ ] Context propagation correct
- [ ] Tests are not mocking the thing being tested

## Handoff Protocol

### From AI Session A to AI Session B
**Document in PR description:**
```markdown
## Package Dependencies
- Uses: `contracts.Publisher` interface
- Uses: `internal/logger` package
- Exports: `internal/storage.Store` interface

## Assumptions Made
- Redis available at localhost:6379
- Database migrations run
- Logger configured before use

## Next Steps
- Session B should implement `platform/registry.go`
- Session B needs to extend `contracts/publish.go` with X field
```

### From AI to Human
**In PR comment:**
```markdown
## Architecture Decisions
1. Used redis/v8 client directly (no wrapper) - low-level control needed
2. Store interface in same package where used - coupling prevention
3. Retries implemented with exponential backoff - production requirement

## Questions for Review
- Should retry max-attempts be configurable?
- Stream grouping strategy correct for multi-consumer?
```

## Coordination Across Sessions

### Modifying Shared `contracts/`
**Before adding/changing interface:**
1. Open issue or PR
2. Link to dependent packages
3. Explain why change needed
4. Wait for review
5. Merge before dependent code

**Example:**
```go
// contracts/publish.go - PR: add PublishMetadata
type Publish struct {
	ID       string
	Content  string
	Metadata *PublishMetadata  // NEW - coordination point
}

type PublishMetadata struct {
	CreatedAt time.Time
	Platform  string
}
```

### Dependency Graph
```
cmd/api → internal/publish → internal/transform
                ↓
         internal/storage → contracts
                ↓
         internal/config
         internal/logger
```

**Rule:** Can't import from higher levels. `contracts` depends on nothing.

## Session Failure Recovery

### If Session Crashes Mid-Implementation
1. Check feature branch for committed work
2. Identify last working commit
3. Review tests for what was done
4. Continue from next logical step
5. Update task description in next session

### If Conflict Arises Between Sessions
1. **Pause both sessions**
2. **Human resolves** in main branch
3. **Notify both sessions** of resolution
4. **Continue from resolved state**

## Testing Across Sessions

### Unit Tests (Session Owns)
- Each session tests its own package
- Mock interfaces at package boundary

### Integration Tests (Scheduled After Merges)
- Run after features merged to main
- Test across multiple packages
- Run in CI/CD automatically

## Performance Considerations

### For AI Sessions
- Don't optimize prematurely (focus on correctness)
- Profile only if issue identified
- Document performance assumptions
- Add benchmarks for critical paths

### Query Optimization
- Write clear SQL first, optimize if slow
- Add query logging in dev mode
- Profile with actual data volumes
- Document index strategy in architecture

## Documentation Handoff

### AI Creates/Updates
- Inline code comments (why, not what)
- Package godoc in source files
- Test comments for complex scenarios
- PR descriptions with context

### Humans Create/Update
- Architecture decisions in `docs/architecture/`
- Deployment guides
- Operational runbooks
- Long-form design docs

## Session Sign-Off Checklist

Before requesting human review:
- [ ] All tests passing locally
- [ ] Code follows coding-standards.md
- [ ] No circular dependencies
- [ ] Package ownership respected
- [ ] Errors wrapped with context
- [ ] Structured logging used
- [ ] Interfaces defined at use site
- [ ] No global state introduced
- [ ] PR description explains why
- [ ] Commits are logical and buildable
