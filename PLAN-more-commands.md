# PLAN-more-commands.md

## Overview

This document outlines a comprehensive plan for expanding git-undo command support beyond the currently implemented commands. Commands are categorized by implementation complexity and organized from easiest to hardest to implement.

## Currently Supported Commands ✅

| Command | Undo Method | Status | Notes |
|---------|-------------|--------|-------|
| `git add` | `git restore --staged` or `git reset` | ✅ Complete | Handles files, flags, initial commits |
| `git commit` | `git reset --soft HEAD~1` | ✅ Complete | Handles merge commits, tags, amendments |
| `git branch <name>` | `git branch -D <name>` | ✅ Complete | Only branch creation, not deletion |
| `git checkout -b` | `git branch -D <name>` | ✅ Complete | Only branch creation scenario |
| `git checkout/switch` | `git checkout -` (via git-back) | ✅ Complete | Dedicated git-back binary |
| `git stash` | `git stash pop && git stash drop` | ✅ Complete | Only stash creation, not pop/apply |
| `git merge` | `git reset --merge ORIG_HEAD` | ✅ Complete | Handles fast-forward and true merges |

---

## 🟢 Easy to Implement (Low Hanging Fruit)

### 1. `git rm` - Remove Files
**Current Command**: `git rm <files>` or `git rm --cached <files>`
**Undo Method**: `git restore --staged <files> && git checkout <files>` (for cached) or `git restore <files>` (for full removal)
**Complexity**: **Low** - Similar to git add logic
**Edge Cases**: 
- Files deleted from working directory vs just unstaged
- Multiple files vs single file
- Directory removal (`git rm -r`)

### 2. `git mv` - Move/Rename Files
**Current Command**: `git mv <old> <new>`
**Undo Method**: `git mv <new> <old>` 
**Complexity**: **Low** - Simple reversal
**Edge Cases**:
- Directory moves
- Multiple file moves in single command
- Cross-directory moves

### 3. `git tag` - Create Tags
**Current Command**: `git tag <tagname>` or `git tag -a <tagname> -m "message"`
**Undo Method**: `git tag -d <tagname>`
**Complexity**: **Low** - Simple deletion
**Edge Cases**:
- Annotated vs lightweight tags
- Tags on specific commits
- Multiple tags created

### 4. `git restore` - Restore Files
**Current Command**: `git restore <files>` or `git restore --staged <files>`
**Undo Method**: 
- For `--staged`: `git add <files>`
- For working tree: Restore from git history/stash
**Complexity**: **Low-Medium** - Need to track previous state
**Edge Cases**:
- Files restored from specific commits
- Partial file restoration

---

## 🟡 Medium Complexity

### 5. `git reset` - Reset Repository State
**Current Command**: `git reset --soft/--mixed/--hard [commit]`
**Undo Method**: 
- Track previous HEAD position
- `git reset --hard <previous_head>` or appropriate restore method
**Complexity**: **Medium** - Multiple reset modes, need state tracking
**Edge Cases**:
- Different reset modes (soft/mixed/hard)
- Reset to specific commits vs relative (HEAD~1)
- Staged/unstaged state preservation

### 6. `git revert` - Create Reverse Commit
**Current Command**: `git revert <commit>`
**Undo Method**: `git reset --hard HEAD~1` (remove the revert commit)
**Complexity**: **Medium** - Need to identify revert commits
**Edge Cases**:
- Multiple commits reverted
- Revert of merge commits
- Conflict resolution during revert

### 7. `git cherry-pick` - Apply Commit from Another Branch
**Current Command**: `git cherry-pick <commit>`
**Undo Method**: `git reset --hard HEAD~1` (remove cherry-picked commit)
**Complexity**: **Medium** - Similar to revert logic
**Edge Cases**:
- Multiple commits cherry-picked
- Cherry-pick with conflicts
- Cherry-pick from remote branches

### 8. `git clean` - Remove Untracked Files
**Current Command**: `git clean -f` or `git clean -fd`
**Undo Method**: **Complex** - Need to backup untracked files before deletion
**Complexity**: **Medium-High** - Requires proactive file backup
**Edge Cases**:
- Different clean modes (-f, -d, -x, -n)
- Selective file cleaning
- Directory structures

---

## 🔴 Hard to Implement (Complex)

### 9. `git rebase` - Reapply Commits
**Current Command**: `git rebase <branch>` or `git rebase -i <commit>`
**Undo Method**: `git reset --hard ORIG_HEAD` or reflog-based recovery
**Complexity**: **High** - Multiple scenarios, interactive rebases
**Edge Cases**:
- Interactive rebases with squash/fixup/edit
- Conflicts during rebase
- Rebase onto different branches
- Multiple branch rebases

### 10. `git pull` - Fetch and Merge
**Current Command**: `git pull [remote] [branch]`
**Undo Method**: Complex combination of fetch/merge undo
**Complexity**: **High** - Combination of fetch + merge operations
**Edge Cases**:
- Pull with rebase (`--rebase`)
- Pull from different remotes
- Fast-forward vs merge scenarios
- Conflict resolution

### 11. `git push` - Upload Changes
**Current Command**: `git push [remote] [branch]`
**Undo Method**: **Very Complex** - Affects remote repositories
**Complexity**: **Very High** - Remote state management
**Edge Cases**:
- Force push scenarios
- Multiple branches pushed
- Protected branches
- Shared repository considerations
- Remote rejection scenarios

### 12. `git submodule` - Manage Submodules
**Current Command**: `git submodule add/update/init`
**Undo Method**: Various depending on subcommand
**Complexity**: **High** - Multiple subcommands, external repositories
**Edge Cases**:
- Submodule addition/removal
- Submodule updates
- Nested submodules
- URL changes

---

## 🟣 Special Cases & Considerations

### 13. `git worktree` - Manage Working Trees
**Current Command**: `git worktree add/remove/prune`
**Undo Method**: Reverse worktree operations
**Complexity**: **Medium-High** - File system operations
**Edge Cases**:
- Multiple worktrees
- Worktree deletion
- Branch associations

### 14. `git remote` - Manage Remotes
**Current Command**: `git remote add/remove/rename`
**Undo Method**: Reverse remote operations
**Complexity**: **Medium** - Configuration management
**Edge Cases**:
- Multiple remotes
- Remote URL changes
- Default remote changes

### 15. `git config` - Configuration Changes
**Current Command**: `git config --local/--global/--system key value`
**Undo Method**: Restore previous configuration values
**Complexity**: **Medium** - Configuration state tracking
**Edge Cases**:
- Different config scopes
- Configuration deletion vs modification
- Complex configuration values

---

## Implementation Strategy

### Phase 1: Quick Wins (Easy Commands)
1. `git rm` - File removal operations
2. `git mv` - File move/rename operations  
3. `git tag` - Tag creation
4. `git restore` - File restoration

### Phase 2: Core Operations (Medium Complexity)
1. `git reset` - Repository state management
2. `git revert` - Reverse commits
3. `git cherry-pick` - Commit application
4. `git clean` - Untracked file management

### Phase 3: Advanced Operations (High Complexity)
1. `git rebase` - History rewriting
2. `git pull` - Remote operations
3. `git submodule` - Submodule management
4. `git worktree` - Working tree management

### Phase 4: Expert Level (Very High Complexity)
1. `git push` - Remote state modification
2. Complex interactive operations
3. Multi-repository scenarios

## Architecture Considerations

### State Tracking Requirements
- **File System State**: For `git clean`, `git mv`, `git rm`
- **Configuration State**: For `git config`, `git remote`
- **Repository State**: For `git reset`, `git rebase`
- **Remote State**: For `git push`, `git pull`

### Safety Mechanisms
- **Backup Creation**: Before destructive operations
- **Conflict Detection**: Before state restoration
- **User Warnings**: For potentially dangerous operations
- **Dry Run Mode**: Preview undo operations

### Error Handling
- **State Validation**: Verify repository state before undo
- **Rollback Capability**: Undo the undo operation
- **Recovery Options**: Alternative undo strategies
- **Clear Error Messages**: Guide users through problems

## Testing Strategy

### Unit Tests
- Command parsing and validation
- Undo command generation logic
- Edge case handling

### Integration Tests
- End-to-end workflow testing
- Multi-command sequences
- Error recovery scenarios

### Safety Tests
- Data preservation verification
- Conflict resolution testing
- Repository integrity validation

## Success Metrics

### Usability
- Commands successfully undone without data loss
- Clear user feedback and warnings
- Intuitive undo behavior

### Reliability  
- Consistent undo behavior across scenarios
- Proper error handling and recovery
- State preservation and restoration

### Performance
- Fast undo command generation
- Minimal repository state inspection
- Efficient logging and tracking

---

*This plan serves as a roadmap for expanding git-undo's command support systematically, prioritizing user value and implementation feasibility.*