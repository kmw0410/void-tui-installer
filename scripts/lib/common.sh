#!/usr/bin/env bash
# shellcheck disable=SC2034
# Shared protocol and resource management. Never enable shell tracing.
set +x
export LC_ALL=C
export PATH=/usr/sbin:/usr/bin:/sbin:/bin
umask 077
REPOSITORY=https://repo-default.voidlinux.org/current
PACKAGES=(base-system linux grub-x86_64-efi btrfs-progs dosfstools snapper sudo dhcpcd cronie tzdata)
FILESYSTEM=btrfs
REPOSITORIES=(-R "$REPOSITORY")
QUERY_REPOSITORIES=(--repository="$REPOSITORY")
DESKTOP=none
TIMEZONE=UTC
GRAPHICS=default
UTILITIES=()
STAGE=preflight
TARGET=
TOP=
WORK=
CANCELLED=0
FINISHED=0

event() {
    local type=$1 status=$2 name=${3:-} message=${4:-}
    jq -cn --arg type "$type" --arg status "$status" --arg name "$name" --arg message "$message" \
        '{version:1,type:$type,status:$status} + (if $name!="" then {name:$name} else {} end) + (if $message!="" then {message:$message} else {} end)'
}

fail() { printf '%s\n' "$1" >&2; return 1; }
run() { "$@" >&2; }
chrun() { run chroot "$TARGET" "$@"; }
stage() {
    STAGE=$1
    event stage running "$STAGE"
    "$2"
    event stage completed "$STAGE"
}
cleanup() {
    local failed=0
    if [[ -n $TARGET ]] && mountpoint -q "$TARGET"; then
        if ! umount -R "$TARGET" >&2; then
            printf '%s\n' 'Failed to unmount target filesystems; manual cleanup is required.' >&2
            failed=1
        fi
    fi
    if [[ -n $TOP ]] && mountpoint -q "$TOP"; then
        if ! umount "$TOP" >&2; then
            printf '%s\n' 'Failed to unmount top-level filesystem.' >&2
            failed=1
        fi
    fi
    if [[ -n $WORK && $failed == 0 ]]; then
        # No recursive removal: never remove files through an unexpected mount.
        rmdir "$WORK/target" "$WORK/top" >&2 || failed=1
        if [[ -d $WORK/repo ]]; then rm -rf -- "$WORK/repo"; fi
        rmdir "$WORK" >&2 || failed=1
    fi
    return "$failed"
}
on_exit() {
    local code=$?
    trap - EXIT ERR TERM INT HUP
    unset PASSWORD
    if (( FINISHED )); then return; fi
    event stage failed "$STAGE" ''
    if [[ $STAGE != cleanup ]]; then
        event stage running cleanup
        if cleanup; then
            event stage completed cleanup
        else
            event stage failed cleanup
        fi
    fi
    if (( CANCELLED )); then
        event result cancelled '' 'Installation was interrupted. The target may be incomplete.'
    else
        event result failed '' 'Installation failed. Review the diagnostic log.'
    fi
    (( code != 0 )) || code=1
    exit "$code"
}
on_signal() { CANCELLED=1; exit 130; }
