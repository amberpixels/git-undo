# TODO

## Test Failures

### TestSequentialUndo - Pre-existing Issue

**Status:** Failing on both `main` and `feature/git-hooks` branches
**Location:** `internal/app/app_test.go:150`

**Issue Description:**
The test expects `file2.txt` to be untracked (`?? file2.txt`) after the second undo operation, but instead sees both files as staged (`A  file1.txt\nA  file2.txt\n`).

**Test Sequence:**
1. Add and commit `file1.txt` → "First commit"
2. Add and commit `file2.txt` → "Second commit"
3. First undo: should undo second commit (keeping `file2.txt` staged) ✅
4. **Second undo: should unstage `file2.txt` ❌ FAILS HERE**
5. Third undo: should undo first commit (keeping `file1.txt` staged)
6. Fourth undo: should unstage `file1.txt`

**Error Messages:**
```
"A  file1.txt\nA  file2.txt\n" does not contain "?? file2.txt"
file2.txt should be untracked after undoing add

failed to execute undo command git commit -m First commit via git reset --soft HEAD~1: exit status 128
```

**Probable Fix:**
The issue appears to be in the undo sequence logic where the test expects alternating commit/add undos, but the actual undo operations aren't properly tracking the sequence state. The fix likely involves:

1. **Check the log ordering:** Verify that `GetLastRegularEntry()` returns commands in the correct order
2. **Verify undo command generation:** Ensure `AddUndoer` correctly handles the case where files should be unstaged vs untracked
3. **Review git state detection:** The undo logic might not properly detect whether files were previously committed or just added

**Investigation Steps:**
1. Add debug logging to see what commands are being logged and retrieved
2. Check if the git hook integration affects command logging order
3. Verify the `git reset --soft HEAD~1` command execution context

**Priority:** Medium (pre-existing issue, not blocking git-hooks feature)