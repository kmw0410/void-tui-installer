#!/usr/bin/env bash
verify_root_mount() {
    local expected actual source
    mountpoint -q "$TARGET" || fail 'Target root is not mounted.'
    [[ $(findmnt -rn -M "$TARGET" -o FSTYPE) == "$FILESYSTEM" ]] || fail 'Target root filesystem does not match the selection.'
    if [[ $FILESYSTEM == btrfs ]]; then
        # Btrfs mount IDs can be anonymous; resolve its source partition instead.
        source=$(findmnt -rn -M "$TARGET" -o SOURCE)
        source=${source%%\[*}
        [[ -b $source && $(readlink -f -- "$source") == "$ROOT" ]] || fail 'Btrfs root is mounted from an unexpected device.'
        verify_mount "$TARGET" @
    elif [[ $FILESYSTEM == ext4 ]]; then
        expected=$(lsblk -dnr -o MAJ:MIN -- "$ROOT")
        [[ $expected =~ ^[0-9]+:[0-9]+$ ]] || fail 'Cannot identify the root partition.'
        actual=$(findmnt -rn -M "$TARGET" -o MAJ:MIN)
        [[ $actual == "$expected" ]] || fail 'Target root is mounted from an unexpected device.'
        [[ $(findmnt -rn -M "$TARGET" -o FSROOT) == / ]] || fail 'The complete ext4 root must be mounted.'
    else
        fail 'Invalid root filesystem.'
    fi
}

create_filesystems() {
    [[ $FILESYSTEM == btrfs || $FILESYSTEM == ext4 ]] || fail 'Invalid root filesystem.'
    check_disk
    run mkfs.vfat -F 32 -n EFI "$EFI"
    check_disk
    if [[ $FILESYSTEM == btrfs ]]; then
        create_btrfs_root
    else
        run mkfs.ext4 -F "$ROOT"
        run mount -t ext4 -o noatime "$ROOT" "$TARGET"
        chmod 0755 "$TARGET"
    fi
    verify_root_mount
    mkdir -p "$TARGET/boot/efi"
    chmod 0755 "$TARGET/boot" "$TARGET/boot/efi"
    run mount "$EFI" "$TARGET/boot/efi"
    [[ $(findmnt -rn -M "$TARGET/boot/efi" -o FSTYPE) == vfat ]] || fail 'EFI mount verification failed.'
    ROOT_UUID=$(blkid -s UUID -o value "$ROOT")
    EFI_UUID=$(blkid -s UUID -o value "$EFI")
    [[ $ROOT_UUID =~ ^[a-fA-F0-9-]+$ && $EFI_UUID =~ ^[a-fA-F0-9-]+$ ]] || fail 'Filesystem UUID lookup failed.'
}

write_fstab() {
    {
        if [[ $FILESYSTEM == btrfs ]]; then
            write_btrfs_fstab
        elif [[ $FILESYSTEM == ext4 ]]; then
            printf 'UUID=%s / ext4 defaults,noatime 0 1\n' "$ROOT_UUID"
        else
            fail 'Invalid root filesystem.'
        fi
        printf 'UUID=%s /boot/efi vfat defaults,umask=0077 0 2\n' "$EFI_UUID"
    } > "$TARGET/etc/fstab"
    chmod 0644 "$TARGET/etc/fstab"
}
