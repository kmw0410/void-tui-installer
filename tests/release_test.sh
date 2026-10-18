#!/usr/bin/env bash
# Release metadata fixtures use a temporary Git repository and a mocked GitHub CLI.
set -Eeuo pipefail
ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
TEST_DIR=$(mktemp -d)
trap 'rm -rf -- "$TEST_DIR"' EXIT
mkdir -p "$TEST_DIR/bin" "$TEST_DIR/repo"
cat > "$TEST_DIR/bin/gh" <<'MOCK'
#!/usr/bin/env bash
set -Eeuo pipefail
[[ $1 == api && $2 == --paginate && $3 == --slurp ]]
if [[ ${RELEASE_FIXTURE_FAILURE:-0} == 1 ]]; then exit 23; fi
printf '%s\n' "${RELEASE_FIXTURE_JSON:-[[]]}"
MOCK
chmod +x "$TEST_DIR/bin/gh"
export PATH="$TEST_DIR/bin:$PATH"
cd "$TEST_DIR/repo"
git init -q
git config user.name 'Release Fixture'
git config user.email 'fixture@example.invalid'
printf 'one\n' > sample
git add sample
git commit -qm 'Initial fixture'
first_sha=$(git rev-parse HEAD)
prepare() {
    : > "$TEST_DIR/outputs"
    {
        printf 'VERSION := %s\n' "$1"
        cat <<'MAKE'
.PHONY: version
version:
	@printf '%s\n' '$(VERSION)'
MAKE
    } > Makefile
    GITHUB_REPOSITORY=fixture/project GITHUB_OUTPUT="$TEST_DIR/outputs" \
        bash "$ROOT_DIR/.github/scripts/prepare-release.sh" > "$TEST_DIR/prepare.log" 2>&1
}

prepare 0.4.0
grep -qx 'version=0.4.0' "$TEST_DIR/outputs"
grep -qx 'tag=v0.4.0' "$TEST_DIR/outputs"
grep -qx 'publish=true' "$TEST_DIR/outputs"
[[ $(cat dist/release-notes.md) == $'## Changes\n- Initial fixture ('"${first_sha:0:7}"')' ]]
printf 'ok 1 - first release includes all history with exact seven-character hashes\n'

for version in '' '0.4' 'v0.4.0' '0.4.0-dev' '0.04.0' '0.4.0-1' '0.4.0;id' '0.4.0/extra'; do
    if prepare "$version"; then printf 'Accepted invalid version: %q\n' "$version" >&2; exit 1; fi
    [[ ! -s $TEST_DIR/outputs ]]
done
printf 'ok 2 - invalid Makefile base versions are rejected before producing outputs\n'

git tag v0.4.0
prepare 0.4.0
grep -qx 'tag=v0.4.0-1' "$TEST_DIR/outputs"
git tag v0.4.0-1
prepare 0.4.0
grep -qx 'version=0.4.0-2' "$TEST_DIR/outputs"
git tag -d v0.4.0-1 >/dev/null
printf 'ok 3 - existing tags receive automatic revision suffixes without overwriting\n'

printf 'two\n' >> sample
# The subject deliberately contains literal shell syntax to test data handling.
# shellcheck disable=SC2016
git commit -qam 'Fix $(touch SHOULD_NOT_EXIST)'
second_sha=$(git rev-parse HEAD)
export RELEASE_FIXTURE_JSON='[[{"tag_name":"v0.4.0","draft":false,"prerelease":false,"published_at":"2026-10-18T00:00:00Z"}]]'
prepare 0.4.0
grep -qx 'version=0.4.0-1' "$TEST_DIR/outputs"
grep -qx 'tag=v0.4.0-1' "$TEST_DIR/outputs"
[[ $(cat dist/release-notes.md) == $'## Changes\n- Fix $(touch SHOULD_NOT_EXIST) ('"${second_sha:0:7}"')' ]]
[[ ! -e SHOULD_NOT_EXIST ]]
printf 'ok 4 - numbered releases use only new commits and preserve literal commit subjects\n'

export RELEASE_FIXTURE_JSON='[[{"tag_name":"v0.4.1","draft":true,"prerelease":false,"published_at":null}]]'
prepare 0.4.1
grep -qx 'tag=v0.4.1-1' "$TEST_DIR/outputs"
printf 'ok 5 - draft-only version collisions also receive a revision suffix\n'

export RELEASE_FIXTURE_FAILURE=1
if prepare 0.4.2; then exit 1; fi
[[ ! -s $TEST_DIR/outputs ]]
unset RELEASE_FIXTURE_FAILURE
printf 'ok 6 - API failures do not become first-release history\n'

git tag v0.4.0-1
export RELEASE_FIXTURE_JSON='[[{"tag_name":"v0.4.0-1","draft":false,"prerelease":false,"published_at":"2026-10-18T01:00:00Z"}]]'
prepare 0.4.0
grep -q 'No new commits' "$TEST_DIR/prepare.log"
[[ $(cat "$TEST_DIR/outputs") == publish=false ]]
[[ ! -e dist/release-notes.md ]]
printf 'ok 7 - the same commit is not published again under a new version\n'

export RELEASE_FIXTURE_JSON='[[{"tag_name":"v9.0.0","draft":true,"prerelease":false,"published_at":null}],[{"tag_name":"v0.4.0-1","draft":false,"prerelease":true,"published_at":"2026-10-18T02:00:00Z"},{"tag_name":"v0.4.0","draft":false,"prerelease":false,"published_at":"2026-10-18T00:00:00Z"}]]'
prepare 0.5.0
[[ $(cat dist/release-notes.md) == $'## Changes\n- Fix $(touch SHOULD_NOT_EXIST) ('"${second_sha:0:7}"')' ]]
printf 'ok 8 - paginated history ignores drafts, prereleases and unpublished tags\n'

prepare 0.4.1
grep -qx 'tag=v0.4.1' "$TEST_DIR/outputs"
printf 'ok 9 - changing the Makefile base version resets the revision suffix\n'
printf '1..9\n'
