# Git Workflow

## Branching Strategy

### Branch Names
Format: `{type}/{scope}-{description}`

Examples:
- `feature/queue-redis-implementation`
- `fix/publish-state-race-condition`
- `docs/add-architecture-diagrams`
- `test/increase-coverage-storage`

Types: `feature`, `fix`, `docs`, `test`, `refactor`, `chore`

### Main Branches
- **`main`** - Production-ready, always deployable, protected
- **`develop`** - Integration branch (optional, direct to main via PR for smaller teams)

## Commit Conventions

### Format
```
<type>(<scope>): <subject>

<body>

Closes #123
```

### Type & Scope
- `feat(platform)` - New feature
- `fix(storage)` - Bug fix
- `docs(architecture)` - Documentation
- `test(publish)` - Test additions
- `refactor(transform)` - Code restructuring
- `chore(deps)` - Dependency updates

### Subject Line
- Lowercase, no period
- Imperative: "add queue interface" not "added queue interface"
- Max 50 characters
- No issue numbers in subject

### Body (when complex)
- Explain **why**, not what
- Wrap at 72 characters
- Blank line before issue reference
- Example:
```
feat(queue): add job priority levels

Redis queues need priority support to process urgent publishes
first. Implementation uses sorted sets with score = -priority
to maintain FIFO within same priority level.

Closes #89
```

### Commits Per PR
- **Logical commits** - Each commit should be compilable and testable
- Squash false starts / WIP commits before review
- Rebase before merge to maintain linear history

## Pull Request Workflow

### PR Creation
1. Push branch to remote
2. Open PR with description:
   ```markdown
   ## What
   Brief description of changes
   
   ## Why
   Why this change was necessary
   
   ## How
   Implementation approach (technical details)
   
   ## Testing
   How to verify (manual steps or test commands)
   
   Closes #123
   ```

### PR Title Format
Same as commit format: `feat(scope): description`

### Code Review Requirements
- **Minimum reviewers:** 1 (for engineering foundation phase)
- **Stale PR:** Close if inactive >7 days
- **Approval workflow:**
  1. Open: Request review
  2. Changes requested: Author updates commits, replies in threads
  3. Approved: Ready for merge
  4. Merge: Delete branch after merge

### Review Expectations
- Focus on logic, tests, architecture adherence
- Check against [coding-standards.md](coding-standards.md)
- Verify test coverage for new code
- Approve only if you'd ship it as-is

### Automatic Checks (CI/CD)
- All tests pass
- No linting errors
- Code coverage >70% on new code
- No security vulnerabilities

## Merge Rules

### Before Merge
- [ ] All CI checks pass
- [ ] At least 1 approval
- [ ] Branch is up to date with main
- [ ] No merge conflicts
- [ ] All conversations resolved

### Merge Method
- **Squash and merge** for single-feature branches (< 3 commits)
- **Rebase and merge** for multi-commit branches with logical progression
- **Never "Create a merge commit"** (no merge commits on main)

### After Merge
- Delete remote branch
- Close related issues
- Monitor deployment in staging

## Conflict Resolution

### When Conflicts Arise
1. Pull latest main: `git pull origin main`
2. Resolve conflicts locally in editor
3. Run tests: `make test`
4. Commit: `git add . && git commit -m "resolve conflicts"`
5. Push and re-request review

### Prevention
- Rebase frequently during development
- Keep PRs focused and small
- Communicate overlapping work

## Tag Strategy

### Release Tags
Format: `v{MAJOR}.{MINOR}.{PATCH}`

Examples:
- `v0.1.0` - Initial engineering foundation release
- `v0.2.1` - Patch for queue fixes
- `v1.0.0` - First production release (rare in this phase)

### Creation
```bash
git tag -a v0.1.0 -m "Initial engineering foundation"
git push origin v0.1.0
```

## Local Development Workflow

### Setup
```bash
git clone <repo>
cd posthub
git config user.name "Your Name"
git config user.email "your.email@example.com"
```

### New Feature
```bash
git checkout -b feature/your-feature main
# Work...
git add .
git commit -m "feat(scope): description"
git push origin feature/your-feature
# Create PR in GitHub UI
```

### Update Local Branch
```bash
git fetch origin
git rebase origin/main
git push origin -f  # Only if you haven't pushed yet
```

### Before Pushing
```bash
make fmt       # Run gofmt
make lint      # Run linter
make test      # Run tests
```

## Protected Branch Rules (main)

- Require PR reviews (1 minimum)
- Require status checks to pass
- Require branches to be up to date
- Allow force pushes: NO
- Dismiss stale reviews on new commits: YES
- Allow auto-merge: NO
