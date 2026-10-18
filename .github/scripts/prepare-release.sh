#!/usr/bin/env bash
set -Eeuo pipefail
export TZ=Asia/Seoul

if (( $# != 0 )); then
    printf 'Release versions are read from Makefile; no arguments are accepted.\n' >&2
    exit 1
fi
: "${GITHUB_REPOSITORY:?GITHUB_REPOSITORY is required}"
: "${GITHUB_OUTPUT:?GITHUB_OUTPUT is required}"
for tool in make git gh jq; do
    command -v "$tool" >/dev/null || { printf 'Required release tool is missing: %s\n' "$tool" >&2; exit 1; }
done
base_version=$(make --no-print-directory -s version)
if [[ ! $base_version =~ ^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
    printf 'Invalid Makefile VERSION. Use a numeric base version such as 0.4.1.\n' >&2
    exit 1
fi

# Fail on API errors; never treat an unavailable release history as a first release.
releases=$(gh api --paginate --slurp "repos/$GITHUB_REPOSITORY/releases")
version=$base_version
tag=v$version
revision=0
while git show-ref --verify --quiet "refs/tags/$tag" ||
    jq -e --arg tag "$tag" 'any(.[][]; .tag_name == $tag)' <<< "$releases" >/dev/null; do
    revision=$((revision + 1))
    version=$base_version-$revision
    tag=v$version
done
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
rm -f dist/release-notes.md
range=HEAD
if [[ -n $previous ]]; then range="refs/tags/$previous..HEAD"; fi
git log --reverse --format='%H%x09%s' "$range" > dist/release-commits.txt
if [[ ! -s dist/release-commits.txt ]]; then
    printf 'No new commits since the previous published release; skipping publication.\n'
    printf 'publish=false\n' >> "$GITHUB_OUTPUT"
    exit 0
fi
{
    printf '## Changes\n'
    while IFS=$'\t' read -r commit_sha subject; do
        printf -- '- %s (%.7s)\n' "$subject" "$commit_sha"
    done < dist/release-commits.txt
} > dist/release-notes.md
printf 'publish=true\nversion=%s\ntag=%s\n' "$version" "$tag" >> "$GITHUB_OUTPUT"
