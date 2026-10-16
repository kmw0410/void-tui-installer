#!/usr/bin/env bash
# Prepare installation tools in the Void Live environment.
set -Eeuo pipefail

if (( EUID != 0 )); then
    printf 'Run this script as root in the Void Live session.\n' >&2
    exit 1
fi
if [[ ! -r /etc/os-release ]] || ! grep -Eq '^ID=("void"|void)$' /etc/os-release || ! grep -Eq '(^| )root=live:(CDLABEL|UUID)=' /proc/cmdline; then
    printf 'Run this script from an official Void Linux Live ISO.\n' >&2
    exit 1
fi
command -v xbps-install >/dev/null || { printf 'Required command is missing: xbps-install\n' >&2; exit 1; }

# XBPS must be updated in its own transaction before installing other packages.
xbps-install -Sy xbps
xbps-install -Sy jq gptfdisk parted dosfstools btrfs-progs psmisc

printf 'Live installation tools are ready. Run ./void-installer-tui.\n'
