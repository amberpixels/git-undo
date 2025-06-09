#!/usr/bin/env bash
set -euo pipefail

pseudo_version() {
    local tag ts hash dirty base major minor patch next

    # 1. last semver tag (falls back to v0.0.0)
    tag=$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || echo v0.0.0)
    base=${tag#v}
    IFS=. read -r major minor patch <<<"$base"

    # 2. bump patch for "next release"
    next=$((patch + 1))

    # 3. timestamp of HEAD in UTC yyyymmddhhmmss
    ts=$(git show -s --format=%cd --date=format:%Y%m%d%H%M%S HEAD)

    # 4. 12-char commit hash
    hash=$(git rev-parse --short=12 HEAD)

    # 5. dirty suffix?
    git diff --quiet || dirty="+dirty"

    echo "v${major}.${minor}.${next}-0.${ts}-${hash}${dirty}"
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    pseudo_version "$@"
fi