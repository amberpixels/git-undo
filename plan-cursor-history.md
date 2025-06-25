# Plan: Branching-Aware Log System

## Overview
Update git-undo logging to handle branching by keeping only the current "branch" of undoable actions, eliminating the need for complex cursor management while solving undo/redo after branching scenarios.

## Current Problem
The existing system marks commands as "undone" with `#` prefix but fails to handle:
- Multiple consecutive undos followed by new commands (branching)
- `git undo undo` (redo) operations in complex scenarios
- Example failing case: `A → B → C → undo → undo → F` should result in log containing only `F` and `A`

## Solution Architecture
Keep existing log-based system but make it "branch-aware":
- When logging new commands after undos, truncate log to current branch only
- Separate navigation commands (git-back) with `N` prefix 
- Mutation commands remain unprefixed for git-undo processing
- Natural undo/redo behavior emerges from simplified log state

## Implementation Steps

### Phase 1: Failing Tests (TDD Approach) ✅
**Status**: COMPLETED - Integration test demonstrates current branching limitation

#### 1.1 Integration Test - End-to-End Branching ✅
- **File**: `scripts/integration/integration-test.bats`
- **Test**: `@test "6A: Phase 6a - cursor-history: branching behavior after undo + new command"`
- **Scenario**: A → B → C → undo → undo → F → test undo/undo behavior
- **Status**: ✅ COMPLETED - Test demonstrates branching limitation and provides target behavior

### Phase 2: Update Logging System

#### 2.1 Branch-Aware Log Writer
- **File**: `internal/git-undo/logging/logger.go`
- **New Logic**: 
  - When logging new command after undos, truncate log to keep only current branch
  - Example: `A B C` → undo C,B → add F results in log: `F A` (not `F #C #B A`)
  - **IMPORTANT**: Never truncate `N` prefixed navigation commands - git-back needs full navigation history
  - Only truncate undone mutation commands when branching occurs

#### 2.2 Navigation Command Prefixing
- **File**: `internal/git-undo/logging/logger.go` 
- **Changes**:
  - Prefix navigation commands (checkout, switch, etc.) with `N`
  - Example log: `N git checkout main`, `git add file.txt`, `N git switch feature`
  - git-undo ignores `N` prefixed entries
  - git-back processes `N` prefixed entries only

#### 2.3 Log State Detection
- **File**: `internal/git-undo/logging/logger.go`
- **Methods**:
  - `CountConsecutiveUndoneCommands()` - count recent `#` prefixed mutation commands only
  - `TruncateToCurrentBranch()` - remove undone mutation commands, preserve all `N` commands
  - `IsNavigationCommand(cmd)` - detect checkout/switch commands

### Phase 3: Integration with Existing System

#### 3.1 Update Command Logging
- **File**: `internal/git-undo/logging/logger.go`
- **Changes**:
  - Modify `LogCommand()` to handle branch truncation
  - Add navigation command detection and prefixing
  - Maintain backward compatibility with existing logs

#### 3.2 Update Command Reading
- **File**: `internal/git-undo/logging/logger.go`
- **Changes**:
  - Skip `N` prefixed commands in `GetLastCommand()` for git-undo
  - Process only `N` prefixed commands for git-back
  - Handle mixed old/new log format

#### 3.3 App Layer Updates
- **File**: `internal/app/app.go`
- **Changes**:
  - No major changes needed - existing undo logic should work
  - Ensure git-back only processes navigation commands
  - Add branch detection when logging new commands

### Phase 4: Testing and Validation

#### 4.1 Unit Tests
- **File**: `internal/git-undo/logging/logger_test.go`
- **Tests**:
  - `TestBranchTruncation()` - verify log truncation on branching
  - `TestNavigationPrefixing()` - verify N prefix for navigation commands
  - `TestMixedLogFormat()` - verify compatibility with existing logs

#### 4.2 Integration Test Updates
- **File**: `scripts/integration/integration-test.bats`
- **Changes**:
  - Update Phase 6A test to expect success (currently fails)
  - Add tests for navigation command separation
  - Verify git-back still works with N prefixed commands

### Phase 5: Final Integration

#### 5.1 Edge Case Handling
- **File**: `internal/git-undo/logging/logger.go`
- **Cases**:
  - Empty logs
  - Mixed navigation and mutation commands
  - Rapid command sequences
  - Log corruption recovery

#### 5.2 Regression Testing
- **Purpose**: Ensure all existing functionality still works
- **Coverage**: Run all existing integration tests
- **Files**: All existing `@test` cases in integration-test.bats

## Key Implementation Details

### Log Truncation Logic
```bash
# Before: N git checkout main, A, B, C → undo undo → log shows: N git checkout main, #C, #B, A
# After adding F: log shows: N git checkout main, F, A (only mutation commands truncated)
# Navigation commands are NEVER truncated - git-back needs full navigation history
```

### Navigation Command Handling
```bash
# git checkout main → logged as: N git checkout main  
# git add file.txt → logged as: git add file.txt
# git switch feature → logged as: N git switch feature
```

### Branch Detection
- Count consecutive `#` prefixed mutation commands (ignore `N` commands)
- If new mutation command logged after undos, truncate those undone mutation commands
- Keep only the "current branch" of undoable actions
- **Always preserve all navigation commands** regardless of branching

## Success Criteria
1. ✅ Failing test demonstrates current limitation (Phase 6A in integration-test.bats)
2. ⏳ `A → B → C → undo → undo → F → undo → undo` produces correct result (A + F staged)
3. ⏳ All existing git-undo functionality preserved
4. ⏳ Clean separation between navigation (git-back) and mutation (git-undo) commands
5. ⏳ Backward compatibility with existing repositories
6. ⏳ Phase 6A integration test passes

## Implementation TODO List

### Phase 2: Update Logging System
- [ ] Modify `LogCommand()` to detect navigation vs mutation commands
- [ ] Add `N` prefix for navigation commands (checkout, switch)
- [ ] Implement branch truncation logic when logging after undos (mutation commands only)
- [ ] Add `CountConsecutiveUndoneCommands()` helper (ignore `N` commands)
- [ ] Add `TruncateToCurrentBranch()` helper (preserve all `N` commands)

### Phase 3: Integration
- [ ] Update command reading to skip `N` prefixed commands for git-undo
- [ ] Update git-back to process only `N` prefixed commands
- [ ] Test with existing repositories (backward compatibility)

### Phase 4: Testing
- [ ] Add unit tests for branch truncation logic
- [ ] Add unit tests for navigation command prefixing
- [ ] Update Phase 6A integration test to expect success
- [ ] Run full regression test suite

### Phase 5: Final Integration
- [ ] Handle edge cases (empty logs, mixed commands)
- [ ] Verify all existing integration tests pass
- [ ] Update documentation if needed

## Benefits of This Approach
- **Simple**: Reuses existing log format with minimal changes
- **Backward Compatible**: Works with existing repositories
- **Natural**: Undo/redo behavior emerges from simplified log state
- **Maintainable**: No complex cursor management or new data structures
- **Fast**: Minimal performance impact