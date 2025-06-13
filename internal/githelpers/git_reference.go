package githelpers

// The logic below treats some verbs as always-mutating,
// and others as "conditional" (e.g. branch, checkout) that only
// mutate when given a target name.

// alwaysMutating are commands that always change state.
// Note: list can be later revisited.
var alwaysMutating = map[string]struct{}{
	"add":         {},
	"am":          {},
	"archive":     {}, // e.g. archive --format=zip
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
}

// conditionalMutating are commands that only mutate if they
// have a non-flag argument (e.g. "git branch foo" vs "git branch").
var conditionalMutating = map[string]struct{}{
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

// readOnlyFlags are flags that make a command read-only even if it's in conditionalMutating.
var readOnlyFlags = map[string]map[string]struct{}{
	"branch": {
		"-r":        {},
		"--remotes": {},
		"--list":    {},
		"--all":     {},
	},
	"checkout": {
		// No read-only flags for checkout - all flags are mutating
	},
	"tag": {
		"-l":     {},
		"--list": {},
	},
	"config": {
		"--get":          {},
		"--list":         {},
		"-l":             {}, // short form of --list
		"--get-all":      {},
		"--get-regexp":   {},
		"--get-urlmatch": {},
	},
	"undo": {
		"--log": {},
	},
}

// readOnlySubcommands are subcommands that make a command read-only.
var readOnlySubcommands = map[string]map[string]struct{}{
	"remote": {
		"show":    {},
		"get-url": {},
		"list":    {},
	},
}

// readOnlyRevertedLogic is the list of commands where by default it's mutating but not read-only.
var readOnlyRevertedLogic = map[string]struct{}{
	"undo": {},
}
