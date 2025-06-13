# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**git-undo** is a CLI tool that provides a universal "Ctrl+Z" for Git commands. It tracks every mutating Git operation and can reverse them with a single `git undo` command.

## Essential Commands

### Development
```bash
make build          # Compile binary with version info to ./build/git-undo
make test           # Run unit tests
make test-all       # Run unit tests + integration tests (dev mode)
make integration-test-dev   # BATS integration tests (test current changes)
make integration-test-prod  # BATS integration tests (test user experience)
make lint           # Run golangci-lint (auto-installs if needed)
make tidy           # Format, vet, and tidy Go modules
```

### Installation & Management
```bash
make install        # Full installation (binary + shell hooks)
make uninstall      # Complete removal
./install.sh        # Direct installation script
```

### Testing Specific Components
```bash
go test ./internal/app                    # Test main application logic
go test ./internal/githelpers            # Test git command parsing
go test ./internal/git-undo/logging     # Test command logging
go test -v ./internal/app -run TestSequentialUndo  # Run specific failing test
```

## Core Architecture

### Component Relationships
```
User Git Command → Shell Hook (captures command) ┐
                ↓                                 ├→ Deduplication Logic → git-undo --hook → Logger → .git/git-undo/commands
           Git Operation → Git Hook (post-operation) ┘                                              ↓
                                                                                                      │
User runs `git undo` → App.Run() → Undoer Factory → Command-specific Undoer → Git Execution        │
                                                             ↓                                      │
                                                        Logger.ToggleEntry() ←────────────────────┘
```

### Key Packages

**`/internal/app/`** - Central orchestrator
- `app.go`: Main application logic, command routing, self-management
- Handles: `git undo`, `git undo undo` (redo), `--verbose`, `--dry-run`, `--log`
- Self-management: `git undo self update/uninstall/version`

**`/internal/git-undo/undoer/`** - Undo command generation
- Interface-based design with factory pattern
- Command-specific implementations: `add.go`, `commit.go`, `merge.go`, `branch.go`, `stash.go`, `checkout.go`
- Each undoer analyzes git state and generates appropriate inverse commands

**`/internal/git-undo/logging/`** - Command tracking system
- Logs format: `TIMESTAMP|REF|COMMAND` in `.git/git-undo/commands`
- Supports marking entries as undone with `#` prefix
- Deduplication between shell hooks and git hooks using flag files and command identifiers
- Per-repository, per-branch tracking

**`/internal/githelpers/`** - Git integration layer
- `gitcommand.go`: Command parsing with `github.com/mattn/go-shellwords`
- `git_reference.go`: Command classification (porcelain/plumbing, read-only/mutating)
- `githelper.go`: Git execution wrapper with proper error handling

### Dual Hook System Architecture: Shell + Git Hooks

**git-undo** uses a sophisticated dual hook system that combines shell hooks and git hooks to capture git operations from different contexts:

#### Shell Hooks (Primary)
- **Bash**: Uses `DEBUG` trap + `PROMPT_COMMAND` to capture command-line git operations
- **Zsh**: Uses `preexec` + `precmd` hooks
- **Coverage**: Captures user-typed commands with exact flags/arguments (e.g., `git add file1.txt file2.txt`)
- **Advantage**: Preserves original command context for precise undo operations

#### Git Hooks (Secondary/Fallback)
- **Hooks**: `post-commit` and `post-merge` via `/scripts/git-undo-git-hook.sh`
- **Coverage**: Captures git operations that bypass shell (IDEs, scripts, git commands run by other tools)
- **Command Reconstruction**: Since hooks run after the fact, commands are reconstructed from git state:
  - `post-commit`: Extracts commit message → `git commit -m "message"`
  - `post-merge`: Detects merge type → `git merge --squash/--no-ff/--ff`

#### Deduplication Strategy
**Problem**: Both hooks often fire for the same operation, risking duplicate logging.

**Solution**: Smart deduplication via command normalization and timing:

1. **Command Normalization**: Both hooks normalize commands to canonical form
   - `git commit -m "test" --verbose` → `git commit -m "test"`
   - `git merge feature --no-ff` → `git merge --no-ff feature`
   - Handles variations in flag order, quotes, and extra flags

2. **Timing + Hashing**: Creates SHA1 identifier from `normalized_command + git_ref + truncated_timestamp`
   - 2-second time window for duplicate detection
   - Git hook runs first, marks command as logged via flag file
   - Shell hook checks flag file, skips if already logged

3. **Hook Priority**: **Git hook wins** when both detect the same operation
   - Git hooks are more reliable for detecting actual git state changes
   - Shell hooks can capture commands that don't change state (failed commands)

#### When Each Hook System Is Useful

**Shell Hooks Excel At:**
- `git add` operations (exact file lists preserved)
- Commands with complex flag combinations
- Failed commands that still need tracking for user context
- Interactive git operations with user input

**Git Hooks Excel At:**
- IDE-triggered commits/merges (VS Code, IntelliJ, etc.)
- Script-automated git operations
- Git operations from other tools (CI/CD, deployment scripts)
- Operations that bypass shell entirely

#### Installation Process
1. **Binary**: Installs to `$(go env GOPATH)/bin`
2. **Git Hooks**:
   - Sets global `core.hooksPath` to `~/.config/git-undo/hooks/` OR
   - Integrates with existing hooks by appending to existing hook files
   - Copies dispatcher script (`git-undo-git-hook.sh`) to hooks directory
   - Creates `post-commit` and `post-merge` hooks (symlinks preferred, fallback to standalone scripts)
3. **Shell Hooks**: Places in `~/.config/git-undo/` and sources from shell rc files

## Testing Strategy

### Unit Tests
- Uses `github.com/stretchr/testify` with suite pattern
- `testutil.GitTestSuite`: Base suite with git repository setup/teardown
- Mock interfaces for git operations to enable isolated testing
- Export pattern: `export_test.go` files expose internal functions for testing

### Integration Tests
- **BATS Framework**: Bash Automated Testing System
- **Dev Mode** (`--dev`): Tests current working directory changes
- **Prod Mode** (`--prod`): Tests real user installation experience
- Real git repository creation and cleanup
- End-to-end workflow verification

### Current Test Issues
- `TestSequentialUndo` failing on both main and feature branches (see `todo.md`)
- Test expects alternating commit/add undo sequence but fails on second undo

## Command Support & Undo Logic

### Supported Commands
- **commit** → `git reset --soft HEAD~1` (preserves staged changes)
- **add** → `git restore --staged <files>` or `git reset <files>`
- **branch** → `git branch -d <branch>`
- **checkout -b** → `git branch -d <branch>`
- **stash** → `git stash pop` + cleanup
- **merge** → `git reset --merge ORIG_HEAD`

### Undo Command Generation
- Context-aware: checks HEAD existence, merge state, tags
- Handles edge cases: initial commits, amended commits, tagged commits
- Provides warnings for potentially destructive operations
- Uses git state analysis to determine appropriate reset strategy

## Build System & Versioning

### Version Management
- Uses `./scripts/src/pseudo_version.sh` for development builds
- Build-time version injection via `-ldflags "-X main.version=$(VERSION)"`
- Priority: git tags → build-time version → "unknown"

### Dependencies
- **Runtime**: Only `github.com/mattn/go-shellwords` for safe command parsing
- **Testing**: `github.com/stretchr/testify`
- **Linting**: `golangci-lint` (auto-installed via Makefile)

## Important Implementation Details

### Command Logging Format
```
2025-01-09 14:30:45|main|git commit -m "test message"
#2025-01-09 14:25:30|main|git add file.txt    # undoed entry (prefixed with #)
```

### Hook Detection & Environment Variables
- **Git Hook Detection**:
  - Primary: `GIT_UNDO_GIT_HOOK_MARKER=1` (set by git hook script)
  - Secondary: `GIT_HOOK_NAME` (contains hook name like "post-commit")
  - Fallback: `GIT_DIR` environment variable presence
- **Shell Hook Detection**: `GIT_UNDO_INTERNAL_HOOK=1` (set by shell hooks)
- **Flag Files**: `.git/git-undo/.git-hook-<identifier>` for marking git hook execution

### Command Normalization Details
- **Supported Commands**: `commit`, `merge`, `rebase`, `cherry-pick`
- **commit**: Extracts `-m message` and `--amend`, ignores flags like `--verbose`, `--signoff`
- **merge**: Extracts merge strategy (`--squash`, `--no-ff`, `--ff`) and branch name
- **Purpose**: Ensures equivalent commands generate identical identifiers for deduplication

### Error Handling Patterns
- Graceful degradation when not in git repository
- Panic recovery in main application loop
- Git command validation before execution
- Comprehensive error wrapping with context

### Security Considerations
- Safe command parsing with `go-shellwords`
- Git command validation against known command whitelist
- No arbitrary command execution
- Proper file permissions for hook installation
