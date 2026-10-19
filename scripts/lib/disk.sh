#!/usr/bin/env bash
partition_path() {
    case "$1" in *[0-9]) printf '%sp%s\n' "$1" "$2";; *) printf '%s%s\n' "$1" "$2";; esac
}
disk_identity() {
    local value
    value=$(lsblk -bdn -o MAJ:MIN,SIZE,LOG-SEC,SERIAL,WWN -- "$1")
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    printf '%s' "$value"
}
check_device_users() {
    local output status
    # The canonical absolute /dev path cannot be parsed as an option.
    if output=$(fuser "$1" 2>&1); then
        fail 'Disk has an active process accessing its block device.'
    else
        status=$?
        [[ $status == 1 && -z $output ]] || fail 'Cannot safely determine active block-device users.'
    fi
}
check_disk() {
    local current node base info serial wwn type ro rm size source swap nodes swaps mount_ids node_id dependencies
    [[ -b $DISK && $(readlink -f -- "$DISK") == "$DISK" ]] || fail 'Target must be a canonical block device.'
    current=$(disk_identity "$DISK")
    [[ $current == "$IDENTITY" ]] || fail 'Disk identity changed. Installation is blocked.'
    info=$(lsblk -bdnJ -o TYPE,RO,RM,SIZE,SERIAL,WWN -- "$DISK")
    type=$(jq -r '.blockdevices[0].type' <<< "$info")
    ro=$(jq -r '.blockdevices[0].ro' <<< "$info")
    rm=$(jq -r '.blockdevices[0].rm' <<< "$info")
    size=$(jq -r '.blockdevices[0].size' <<< "$info")
    serial=$(jq -r '.blockdevices[0].serial // ""' <<< "$info")
    wwn=$(jq -r '.blockdevices[0].wwn // ""' <<< "$info")
    [[ $type == disk && ( $ro == false || $ro == 0 ) && ( $rm == false || $rm == 0 ) ]] || fail 'Disk is not an eligible writable non-removable whole disk.'
    if [[ ! $size =~ ^[0-9]+$ ]] || (( size < 17179869184 )); then fail 'Disk must be at least 16 GiB.'; fi
    check_target_capacity "$size"
    [[ -n $serial || -n $wwn ]] || fail 'Disk has no stable hardware identity in the reviewed identity record.'
    # Descendants may only be ordinary partitions, never device-mapper or RAID.
    info=$(lsblk -bJ -p -o NAME,TYPE,MOUNTPOINTS -- "$DISK")
    jq -e '[..|objects|select(has("name"))] | all((.type=="disk" or .type=="part") and ([.mountpoints[]?]|all(.==null)))' <<< "$info" >/dev/null || fail 'Disk has mounted or unsupported descendants.'
    nodes=$(jq -r '..|objects|select(has("name"))|.name' <<< "$info")
    swaps=$(swapon --show=NAME --noheadings --raw) || fail 'Cannot inspect active swap.'
    mount_ids=$(findmnt -rn -o MAJ:MIN) || fail 'Cannot inspect mounted filesystems.'
    [[ -n $mount_ids ]] || fail 'The mount table is empty.'
    while IFS= read -r node; do
        base=${node##*/}
        check_device_users "$node"
        [[ -d /sys/class/block/$base/holders ]] || fail 'Storage topology is unavailable.'
        dependencies=$(find "/sys/class/block/$base/holders" -mindepth 1 -maxdepth 1 -print -quit) || fail 'Cannot inspect disk holders.'
        [[ -z $dependencies ]] || fail 'Disk has active storage holders.'
        if [[ -d /sys/class/block/$base/slaves ]]; then
            dependencies=$(find "/sys/class/block/$base/slaves" -mindepth 1 -maxdepth 1 -print -quit) || fail 'Cannot inspect disk dependencies.'
            [[ -z $dependencies ]] || fail 'Disk has active storage dependencies.'
        else
            [[ -f /sys/class/block/$base/partition ]] || fail 'Whole-disk dependencies cannot be inspected.'
        fi
        # Inspect mount table independently of lsblk, including bind mounts.
        node_id=$(lsblk -dnr -o MAJ:MIN -- "$node")
        [[ $node_id =~ ^[0-9]+:[0-9]+$ ]] || fail 'Cannot inspect the block device number.'
        [[ $'\n'"$mount_ids"$'\n' != *$'\n'"$node_id"$'\n'* ]] || fail 'Disk is mounted.'
        while IFS= read -r swap; do
            [[ -n $swap ]] || continue
            if [[ -b $swap ]]; then
                [[ $(readlink -f "$swap") != "$node" ]] || fail 'Disk contains active swap.'
            else
                source=$(findmnt -rn -o SOURCE -T "$swap") || fail 'Cannot resolve active swap backing storage.'
                [[ ${source%%\[*} != "$node" ]] || fail 'Disk backs an active swap file.'
            fi
        done <<< "$swaps"
    done <<< "$nodes"
    # A live ISO loaded into RAM can leave its source disk unmounted.
    check_live_source
}
resolve_live_source() {
    local source
    source=$(blkid -c /dev/null -t "$1" -o device) || fail 'Cannot resolve the live-medium identifier.'
    [[ $source != *$'\n'* && -b $source && $(readlink -f -- "$source") == "$source" ]] || fail 'Live medium is unavailable or ambiguous.'
    printf '%s\n' "$source"
}
check_live_source() {
    local token label source ancestor ancestors cmdline found=0
    cmdline=$(cat /proc/cmdline)
    for token in $cmdline; do
        case "$token" in
            root=live:CDLABEL=*)
                label=${token#root=live:CDLABEL=}
                source=$(resolve_live_source "LABEL=$label") || fail 'Cannot safely identify the live medium.'
                ancestors=$(lsblk -snrp -o NAME -- "$source") || fail 'Cannot resolve live-medium ancestry.'
                [[ -n $ancestors ]] || fail 'Live-medium ancestry is empty.'
                while IFS= read -r ancestor; do
                    [[ $ancestor != "$DISK" ]] || fail 'The target is the live installation medium.'
                done <<< "$ancestors"
                found=1;;
            root=live:UUID=*)
                source=$(resolve_live_source "UUID=${token#root=live:UUID=}") || fail 'Cannot safely identify the live medium.'
                ancestors=$(lsblk -snrp -o NAME -- "$source") || fail 'Cannot resolve live-medium ancestry.'
                [[ -n $ancestors ]] || fail 'Live-medium ancestry is empty.'
                while IFS= read -r ancestor; do
                    [[ $ancestor != "$DISK" ]] || fail 'The target is the live installation medium.'
                done <<< "$ancestors"
                found=1;;
        esac
    done
    (( found == 1 )) || fail 'Unsupported or ambiguous live-medium boot source.'
}
partition_disk() {
    check_disk
    check_static_interface
    EFI=$(partition_path "$DISK" 1)
    ROOT=$(partition_path "$DISK" 2)
    check_disk
    run sgdisk --zap-all "$DISK"
    check_disk
    run sgdisk --clear --new=1:0:+1G --typecode=1:ef00 --change-name=1:EFI --new=2:0:0 --typecode=2:8300 --change-name=2: "$DISK"
    run partprobe "$DISK"
    run udevadm settle --timeout=30
    local attempt
    for (( attempt=0; attempt<30; attempt++ )); do
        if [[ -b $EFI && -b $ROOT ]]; then break; fi
        sleep 1
    done
    [[ -b $EFI && -b $ROOT ]] || fail 'New partitions did not appear.'
    [[ $(lsblk -dnr -o PKNAME "$EFI") == "${DISK##*/}" && $(lsblk -dnr -o PKNAME "$ROOT") == "${DISK##*/}" ]] || fail 'Partition parent verification failed.'
    run sgdisk --verify "$DISK"
    local layout
    layout=$(lsblk -bdnr -o SIZE -- "$EFI")
    [[ $layout == 1073741824 ]] || fail 'EFI partition size verification failed.'
    layout=$(lsblk -J -o TYPE -- "$DISK")
    jq -e '[..|objects|select(.type? == "part")]|length==2' <<< "$layout" >/dev/null || fail 'Unexpected partitions remain on the disk.'
    check_disk
    [[ $(lsblk -dnr -o PARTTYPE "$EFI") == c12a7328-f81f-11d2-ba4b-00a0c93ec93b && $(lsblk -dnr -o PARTTYPE "$ROOT") == 0fc63daf-8483-4772-8e79-3d69d8477de4 ]] || fail 'Partition type verification failed.'
}
