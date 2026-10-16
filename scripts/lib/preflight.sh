#!/usr/bin/env bash
preflight_live_environment() {
    local cmd
    for cmd in grep cat uname od tr mountpoint; do
        command -v "$cmd" >/dev/null || fail "Required base command is missing: $cmd"
    done
    (( EUID == 0 )) || fail 'Root privileges are required.'
    [[ $(uname -m) == x86_64 && -d /sys/firmware/efi ]] || fail 'An x86_64 UEFI boot is required.'
    [[ -r /sys/firmware/efi/fw_platform_size && $(cat /sys/firmware/efi/fw_platform_size) == 64 ]] || fail '64-bit UEFI firmware is required.'
    if [[ ! -r /etc/os-release ]] || ! grep -Eq '^ID=("void"|void)$' /etc/os-release; then fail 'Run this installer from an official Void Live ISO.'; fi
    grep -Eq '(^| )root=live:(CDLABEL|UUID)=' /proc/cmdline || fail 'An official Void Live session is required.'
    [[ ! -d /sys/firmware/efi/efivars ]] || {
        local secure
        for secure in /sys/firmware/efi/efivars/SecureBoot-*; do
            [[ -f $secure ]] || continue
            [[ $(od -An -tu1 -j4 -N1 "$secure" | tr -d ' ') == 0 ]] || fail 'Disable Secure Boot before installation.'
        done
    }
    [[ -d /var/db/xbps/keys ]] || fail 'Void repository signing keys are unavailable.'
    if ! mountpoint -q /dev || ! mountpoint -q /proc || ! mountpoint -q /sys; then fail 'Required live system filesystems are not mounted.'; fi
}
preflight_environment() {
    preflight_live_environment
    local cmd
    for cmd in jq awk lsblk findmnt mountpoint mount umount blkid swapon fuser sgdisk partprobe udevadm mkfs.vfat mkfs.btrfs btrfs xbps-install xbps-query chroot flock head find readlink grep sed cp mkdir rmdir rm mktemp chmod ln sync cat uname sleep od tr timeout stat; do
        command -v "$cmd" >/dev/null || fail "Required command is missing: $cmd"
    done
}
prepare_live_environment() {
    # Validate the Live session before changing its package database. No jq is needed here.
    preflight_live_environment
    local cmd missing=0
    for cmd in xbps-install flock; do
        command -v "$cmd" >/dev/null || fail "Required base command is missing: $cmd"
    done
    acquire_installer_lock
    for cmd in jq sgdisk partprobe mkfs.vfat mkfs.btrfs btrfs fuser; do
        if ! command -v "$cmd" >/dev/null; then
            printf 'Missing Live tool: %s\n' "$cmd" >&2
            missing=1
        fi
    done
    if (( missing )); then
        # Update XBPS separately, then install the fixed Live-tool package set.
        preflight_command 'Live XBPS update' xbps-install -Sy xbps
        preflight_command 'Live installation tools' xbps-install -Sy jq gptfdisk parted dosfstools btrfs-progs psmisc
    fi
    preflight_environment
}
acquire_installer_lock() {
    exec 9>/run/void-tui-installer.lock
    flock -n 9 || fail 'Another installer is running.'
}
preflight_command() {
    local operation=$1 status
    shift
    if "$@" >&2; then
        return 0
    else
        status=$?
        printf 'Preflight failed: %s (exit status %s).\n' "$operation" "$status" >&2
        return "$status"
    fi
}
preflight_repository() {
    local dir=$1
    mkdir -p "$dir/var/db/xbps/keys"
    cp -a /var/db/xbps/keys/. "$dir/var/db/xbps/keys/"
    # A private XBPS root keeps live-session package databases unchanged.
    preflight_command 'Repository index synchronization' timeout 180 env XBPS_ARCH=x86_64 xbps-install -S -r "$dir" "${REPOSITORIES[@]}"
    preflight_transaction "$dir"
    local package
    for package in "${PACKAGES[@]}"; do
        preflight_command "Repository package lookup: $package" timeout 30 env XBPS_ARCH=x86_64 xbps-query -r "$dir" -R "${QUERY_REPOSITORIES[@]}" -p pkgver "$package"
    done
}
preflight_transaction() {
    local dir=$1 status=0
    # XBPS performs statvfs on this private metadata root even in dry-run mode.
    # ENOSPC is considered only after validating the fully resolved plan below.
    timeout 180 env XBPS_ARCH=x86_64 xbps-install -y -n -r "$dir" "${REPOSITORIES[@]}" "${PACKAGES[@]}" > "$dir/transaction.plan" 2> "$dir/transaction.err" || status=$?
    cat "$dir/transaction.err" "$dir/transaction.plan" >&2
    if (( status == 28 )); then
        # Upstream resolves dependencies, conflicts and shared libraries before
        # its final space check. Reject any other or additional diagnostic.
        awk '
            /^ERROR: Transaction aborted due to insufficient disk space \(need [^,]+, got [^)]+ free\)\.$/ { matched++; next }
            NF { invalid=1 }
            END { exit !(matched==1 && !invalid) }
        ' "$dir/transaction.err" || fail 'Unrecognized XBPS space-check failure; installation is blocked.'
    elif (( status != 0 )); then
        printf 'Preflight failed: Package transaction resolution (exit status %s).\n' "$status" >&2
        return "$status"
    fi
    PREFLIGHT_REQUIRED_BYTES=$(awk -v repository="$REPOSITORY" -v graphics="$GRAPHICS" '
        NF!=6 || $1 !~ /^[A-Za-z0-9][A-Za-z0-9+_.-]*$/ || $2!="install" || ($3!="x86_64" && $3!="noarch") || ($4!=repository && !(graphics=="nvidia" && $4==repository "/nonfree")) || $5 !~ /^[0-9]+$/ || $6 !~ /^[0-9]+$/ { invalid=1; next }
        { installed+=$5; downloads+=$6; count++ }
        END {
            if (invalid || !count || installed<=0 || installed+downloads>1000000000000000) exit 1
            printf "%.0f\n", installed+downloads+2147483648
        }
    ' "$dir/transaction.plan") || fail 'Invalid or incomplete XBPS transaction plan.'
    if (( status == 28 )); then
        printf '%s\n' 'The Live temporary filesystem is not the installation target; target capacity will be checked before erasure.' >&2
    fi
}
check_target_capacity() {
    local size=$1 required=${PREFLIGHT_REQUIRED_BYTES:-}
    [[ $size =~ ^[0-9]+$ && $required =~ ^[0-9]+$ ]] || fail 'Cannot determine the required target capacity.'
    # Reserve the ESP and alignment; the plan includes archive cache and 2 GiB headroom.
    (( size - 1073741824 - 2097152 >= required )) || fail 'Target disk cannot hold the resolved packages, download cache and safety reserve.'
}
preflight_install() {
    preflight_environment
    check_static_interface
    preflight_repository "$WORK/repo"
    check_graphics
    check_disk
}
