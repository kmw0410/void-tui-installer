#!/usr/bin/env bash
verify_installation() {
    local path sub package
    verify_root_mount
    if [[ $FILESYSTEM == btrfs ]]; then
        for sub in @ @home @log @cache @snapshots; do
            case "$sub" in @) path=;; @home) path=/home;; @log) path=/var/log;; @cache) path=/var/cache;; @snapshots) path=/.snapshots;; esac
            verify_mount "$TARGET$path" "$sub"
        done
    fi
    chrun findmnt --verify --tab-file /etc/fstab
    for package in "${PACKAGES[@]}"; do
        chrun xbps-query -p pkgver "$package"
    done
    chrun id "$USERNAME"
    verify_account_passwords
    verify_timezone
    verify_network_configuration
    verify_software
    chrun visudo -cf /etc/sudoers
    chrun sudo -l -U "$USERNAME"
    [[ -s $TARGET/boot/grub/grub.cfg ]] || fail 'GRUB configuration is missing.'
    compgen -G "$TARGET/boot/vmlinuz-*" >/dev/null || fail 'No installed kernel was found.'
    local kernel version
    for kernel in "$TARGET"/boot/vmlinuz-*; do
        version=${kernel##*/vmlinuz-}
        [[ -s $TARGET/boot/initramfs-$version.img ]] || fail 'A kernel is missing its matching initramfs.'
    done
    chrun su -s /bin/sh -c 'test -x / && test -x /home && test -r /etc/hosts && test -r /etc/resolv.conf' "$USERNAME"
    local network_service=dhcpcd
    if [[ $DESKTOP == kde || $DESKTOP == niri ]]; then network_service=NetworkManager; fi
    for path in "$network_service" dbus cronie bluetoothd tlp; do
        [[ -L $TARGET/etc/runit/runsvdir/default/$path ]] || fail 'Required service is not enabled.'
    done
    if [[ $FILESYSTEM == btrfs ]]; then chrun snapper --no-dbus -c root list; fi
    run sync
}
