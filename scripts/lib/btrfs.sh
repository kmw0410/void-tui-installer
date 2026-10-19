#!/usr/bin/env bash
mount_subvolume() {
    local name=$1 path=$2
    mkdir -p "$TARGET$path"
    chmod 0755 "$TARGET$path"
    run mount -t btrfs -o "noatime,compress=zstd:3,subvol=$name" "$ROOT" "$TARGET$path"
    verify_mount "$TARGET$path" "$name"
}
verify_mount() {
    local path=$1 subvol=$2 actual
    mountpoint -q "$path" || fail 'A required filesystem is not mounted.'
    [[ $(findmnt -rn -M "$path" -o FSTYPE) == btrfs ]] || fail 'Unexpected filesystem type.'
    actual=$(findmnt -rn -M "$path" -o FSROOT)
    [[ $actual == "/$subvol" ]] || fail 'Incorrect Btrfs subvolume mount.'
}
create_btrfs_root() {
    run mkfs.btrfs -f "$ROOT"
    run mount -t btrfs -o subvolid=5 "$ROOT" "$TOP"
    local sub
    for sub in @ @home @snapshots @log @cache; do
        run btrfs subvolume create "$TOP/$sub"
        if [[ $sub == @snapshots ]]; then chmod 0700 "$TOP/$sub"; else chmod 0755 "$TOP/$sub"; fi
    done
    run umount "$TOP"
    mount_subvolume @ ''
    mount_subvolume @home /home
    mount_subvolume @log /var/log
    mount_subvolume @cache /var/cache
    # Snapper must create its own temporary .snapshots before @snapshots mounts.
}
write_btrfs_fstab() {
    cat <<FSTAB
UUID=$ROOT_UUID / btrfs noatime,compress=zstd:3,subvol=@ 0 0
UUID=$ROOT_UUID /home btrfs noatime,compress=zstd:3,subvol=@home 0 0
UUID=$ROOT_UUID /.snapshots btrfs noatime,compress=zstd:3,subvol=@snapshots 0 0
UUID=$ROOT_UUID /var/log btrfs noatime,compress=zstd:3,subvol=@log 0 0
UUID=$ROOT_UUID /var/cache btrfs noatime,compress=zstd:3,subvol=@cache 0 0
FSTAB
}
