#!/usr/bin/env bash
set -Eeuo pipefail
export TZ=Asia/Seoul

version=${1:-}
version=${version#v}
if [[ ! $version =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[1-9][0-9]*)?$ ]]; then
    printf 'Invalid version. Use 0.4.0 or 0.4.0-1, optionally prefixed with v.\n' >&2
    exit 1
fi
tag=v$version
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
: "${GITHUB_OUTPUT:?GITHUB_OUTPUT is required}"
for tool in git gh jq; do
    command -v "$tool" >/dev/null || { printf 'Required release tool is missing: %s\n' "$tool" >&2; exit 1; }
done
if git show-ref --verify --quiet "refs/tags/$tag"; then
    printf 'Tag already exists: %s\n' "$tag" >&2
    exit 1
fi

# Fail on API errors; never treat an unavailable release history as a first release.
releases=$(gh api --paginate --slurp "repos/$GITHUB_REPOSITORY/releases")
if jq -e --arg tag "$tag" 'any(.[][]; .tag_name == $tag)' <<< "$releases" >/dev/null; then
    printf 'Release already exists: %s\n' "$tag" >&2
    exit 1
fi
published_tags=$(jq -r 'add | map(select(.draft == false and .prerelease == false)) | sort_by(.published_at) | reverse | .[].tag_name' <<< "$releases")
previous=
while IFS= read -r candidate; do
    [[ $candidate =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[1-9][0-9]*)?$ ]] || continue
    if git rev-parse --verify --quiet "refs/tags/$candidate^{commit}" >/dev/null &&
        git merge-base --is-ancestor "refs/tags/$candidate" HEAD; then
        previous=$candidate
        break
    fi
done <<< "$published_tags"

mkdir -p dist
range=HEAD
if [[ -n $previous ]]; then range="refs/tags/$previous..HEAD"; fi
git log --reverse --format='%H%x09%s' "$range" > dist/release-commits.txt
if [[ ! -s dist/release-commits.txt ]]; then
    printf 'No new commits since the previous published release.\n' >&2
    exit 1
fi
{
    printf '## Changes\n'
    while IFS=$'\t' read -r commit_sha subject; do
        printf -- '- %s (%.7s)\n' "$subject" "$commit_sha"
    done < dist/release-commits.txt
} > dist/release-notes.md
printf 'version=%s\ntag=%s\n' "$version" "$tag" >> "$GITHUB_OUTPUT"
