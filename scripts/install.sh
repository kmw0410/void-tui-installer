#!/usr/bin/env bash
# shellcheck disable=SC2034
set -Eeuo pipefail
set +x
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
# Modules are trusted installation resources, never user input.
for module in common mirror config session software disk preflight btrfs filesystems bootstrap system network grub snapper verify; do
    # shellcheck source=/dev/null
    source "$SCRIPT_DIR/lib/$module.sh"
done
main() {
    if [[ ${1:-} == --check-install && $# == 1 ]]; then
        check_installation_plan
        return
    fi
    if [[ ${1:-} == --list-packages && $# == 2 ]]; then
        list_repository_packages "$2"
        return
    fi
    if [[ ${1:-} == --prepare-live && $# == 1 ]]; then
        prepare_live_environment
        return
    fi
    if [[ ${1:-} == --check-local && $# == 1 ]]; then
        preflight_environment
        return
    fi
    if [[ ${1:-} == --check && $# -le 2 ]]; then
        preflight_environment
        configure_mirror_root "${2:-https://repo-default.voidlinux.org}"
        CHECK_DIR=$(mktemp -d /tmp/void-installer-check.XXXXXXXX)
        trap 'rm -rf -- "$CHECK_DIR"' EXIT
        preflight_repository "$CHECK_DIR"
        return
    fi
    (( $# == 0 )) || fail 'Usage: install.sh [--prepare-live|--check-local|--check [MIRROR]|--list-packages MIRROR|--check-install]'
    command -v jq >/dev/null || fail 'Required command is missing: jq'
    trap on_exit EXIT
    trap on_signal TERM INT HUP
    event stage running preflight
    preflight_environment
    acquire_installer_lock
    read_config
    select_packages
    WORK=$(mktemp -d /tmp/void-installer.XXXXXXXX)
    TARGET=$WORK/target
    TOP=$WORK/top
    mkdir "$TARGET" "$TOP"
    preflight_install
    event stage completed preflight
    stage partitioning partition_disk
    stage filesystems create_filesystems
    stage bootstrap bootstrap
    stage system configure_system
    stage network configure_network
    stage software configure_software
    stage grub configure_grub
    if [[ $FILESYSTEM == btrfs ]]; then
        stage snapper configure_snapper
    else
        event stage running snapper
        event stage skipped snapper
    fi
    stage verification verify_installation
    stage cleanup cleanup
    FINISHED=1
    event result success '' 'Installation completed. Remove the installation medium before rebooting.'
}
if [[ ${BASH_SOURCE[0]} == "$0" ]]; then main "$@"; fi
