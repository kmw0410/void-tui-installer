#!/usr/bin/env bash
# Shared repository arrays are consumed by bootstrap and preflight modules.
# shellcheck disable=SC2034
configure_mirror_root() {
    local root=${1:-https://repo-default.voidlinux.org} part
    while [[ $root == */ ]]; do root=${root%/}; done
    [[ ${#root} -le 512 && $root =~ ^https://[a-zA-Z0-9]([a-zA-Z0-9.-]*[a-zA-Z0-9])?(:[0-9]{1,5})?(/[a-zA-Z0-9_+.-]+)*$ ]] || fail 'Invalid HTTPS mirror root.'
    local -a parts
    IFS=/ read -r -a parts <<< "${root#https://}"
    for part in "${parts[@]:1}"; do
        [[ $part != . && $part != .. && $part != current && $part != nonfree && $part != multilib ]] || fail 'Enter the mirror root without repository suffixes.'
    done
    MIRROR_ROOT=$root
    REPOSITORY=$root/current
    REPOSITORIES=(-R "$REPOSITORY")
    QUERY_REPOSITORIES=(--repository="$REPOSITORY")
}
persist_mirror() {
    local file name found=0
    mkdir -p "$TARGET/etc/xbps.d"
    chmod 0755 "$TARGET/etc/xbps.d"
    for file in "$TARGET"/usr/share/xbps.d/*-repository-*.conf; do
        [[ -f $file ]] || continue
        name=${file##*/}
        sed "s|https://repo-default.voidlinux.org|$MIRROR_ROOT|g" "$file" > "$TARGET/etc/xbps.d/$name"
        chmod 0644 "$TARGET/etc/xbps.d/$name"
        found=1
    done
    (( found )) || fail 'Packaged repository configuration is unavailable.'
    grep -Fqx "repository=$REPOSITORY" "$TARGET/etc/xbps.d/00-repository-main.conf" || fail 'The selected mirror was not saved.'
}
