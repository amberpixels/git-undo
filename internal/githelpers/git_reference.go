package githelpers

// The logic below classifies git commands into different behavior types:
// - alwaysMutating: commands that always create or modify repository state
// - alwaysReadOnly: commands that only read information and should not be logged
// - conditionalBehavior: commands whose behavior depends on their arguments

// alwaysMutating are commands that always change state and can be undone with git-undo.
var alwaysMutating = map[string]struct{}{
	"add":         {},
	"am":          {},
	"commit":      {},
	"fetch":       {}, // writes to .git/FETCH_HEAD
	"init":        {},
	"merge":       {},
	"mv":          {},
	"pull":        {}, // but what if nothing to pull?
	"push":        {}, // but what if nothing to push?
	"rebase":      {},
	"reset":       {},
	"revert":      {},
	"rm":          {},
	"stash":       {},
	"submodule":   {}, // e.g. submodule add/update
	"worktree":    {}, // add/remove
	"cherry-pick": {},
	"clone":       {},
	"clean":       {},
}

// alwaysReadOnly are commands that only read information and should not be logged.
var alwaysReadOnly = map[string]struct{}{
	"status":    {},
	"log":       {},
	"diff":      {},
	"show":      {},
	"blame":     {},
	"ls-files":  {},
	"ls-remote": {},
	"grep":      {},
	"shortlog":  {},
	"describe":  {},
	"rev-parse": {},
	"cat-file":  {},
	"help":      {},
	"reflog":    {},
	"name-rev":  {},
	"archive":   {}, // e.g. archive --format=zip (doesn't modify repo)
}

// conditionalBehavior are commands whose behavior depends on their arguments.
// These need special logic to determine if they're mutating, navigating, or read-only.
var conditionalBehavior = map[string]struct{}{
	"branch":   {},
	"checkout": {},
	"restore":  {},
	"switch":   {}, // newer porcelain alias for checkout
	"tag":      {},
	"remote":   {},
	"config":   {},
	"undo":     {},
}

// porcelainCommands is the list of "user-facing" verbs (main porcelain commands).
var porcelainCommands = []string{
	"add", "am", "archive", "bisect", "blame", "branch", "bundle",
	"checkout", "cherry", "cherry-pick", "citool", "clean", "clone",
	"commit", "describe", "diff", "fetch", "format-patch", "gc",
	"grep", "gui", "help", "init", "log", "merge", "mv", "notes",
	"pull", "push", "rebase", "reflog", "remote", "reset", "revert",
	"rm", "shortlog", "show", "stash", "status", "submodule", "switch", "tag",
	"worktree", "config", "restore",
	"undo",
}

// plumbingCommands is the list of low-level plumbing verbs.
var plumbingCommands = []string{
	"apply-mailbox", "apply-patch", "cat-file", "check-attr", "check-ignore",
	"check-mailmap", "check-ref-format", "checkout-index", "commit-tree",
	"diff-files", "diff-index", "diff-tree", "fast-export", "fast-import",
	"fmt-merge-msg", "for-each-ref", "hash-object", "http-backend",
	"index-pack", "init-db", "log-tree", "ls-files", "ls-remote", "ls-tree",
	"merge-base", "merge-index", "merge-tree", "mktag", "mktree",
	"pack-objects", "pack-redundant", "pack-refs", "patch-id",
	"prune", "receive-pack", "remote-ext", "replace", "rev-list",
	"rev-parse", "send-pack", "show-index", "show-ref", "symbolic-ref",
	"unpack-file", "unpack-objects", "update-index", "update-ref",
	"verify-commit", "verify-pack", "verify-tag", "write-tree",
	"name-rev",
}

// customCommands is the list of custom commands (third-party plugins).
var customCommands = []string{
	"undo",
}

// buildLookup builds a map from verb → its CommandType.
func buildLookup() map[string]CommandType {
	m := make(map[string]CommandType, len(porcelainCommands)+len(plumbingCommands))
	for _, cmd := range porcelainCommands {
		m[cmd] = Porcelain
	}
	for _, cmd := range plumbingCommands {
		m[cmd] = Plumbing
	}
	for _, cmd := range customCommands {
		m[cmd] = Custom
	}
	return m
}

var lookup = buildLookup()

// Note: The old readOnlyFlags, readOnlySubcommands, and readOnlyRevertedLogic maps
// have been replaced by the behavior determination logic in gitcommand.go
