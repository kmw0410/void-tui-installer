#!/usr/bin/env bash
# These tests use fixtures and mocks only. They never open block devices.
set -Eeuo pipefail
ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
SCRIPT_DIR=$ROOT_DIR/scripts
source "$ROOT_DIR/scripts/lib/common.sh"
source "$ROOT_DIR/scripts/lib/mirror.sh"
source "$ROOT_DIR/scripts/lib/config.sh"
source "$ROOT_DIR/scripts/lib/software.sh"
source "$ROOT_DIR/scripts/lib/disk.sh"
source "$ROOT_DIR/scripts/lib/btrfs.sh"
source "$ROOT_DIR/scripts/lib/network.sh"
source "$ROOT_DIR/scripts/lib/session.sh"
source "$ROOT_DIR/scripts/lib/system.sh"
TEST_DIR=$(mktemp -d)
trap 'rm -rf -- "$TEST_DIR"' EXIT
count=0
pass() { count=$((count+1)); printf 'ok %s - %s\n' "$count" "$1"; }
reject() {
    local code=0
    # A fresh Bash preserves errexit within validation functions.
    bash -Eeuo pipefail -c 'source "$1/scripts/lib/common.sh"; source "$1/scripts/lib/mirror.sh"; source "$1/scripts/lib/config.sh"; SCRIPT_DIR=$1/scripts; CONFIG=$2; validate_config' _ "$ROOT_DIR" "$1" >/dev/null 2>&1 || code=$?
    (( code != 0 )) || { printf 'Expected rejection\n' >&2; exit 1; }
}
CONFIG='{"version":1,"disk":"/dev/vda","identity":"252:0 21474836480 512 test","username":"alice","hostname":"voidbox","network":"dhcp","interface":"","address":"","gateway":"","dns":"","confirmation":"ERASE /dev/vda"}'
GOOD=$CONFIG
validate_config
pass 'valid DHCP configuration'
for field in disk identity username hostname network interface address gateway dns confirmation desktop graphics mirror timezone; do
    for control in '\n' '\u0000' '\t' '\u007f'; do
        reject "$(jq --arg field "$field" --argjson control "\"$control\"" '.[$field]=((.[$field]//"")+$control)' <<< "$GOOD")"
    done
done
pass 'original JSON control characters are rejected before Bash normalization'
[[ $TIMEZONE == UTC ]]
CONFIG=$(jq '.timezone="Asia/Seoul"' <<< "$GOOD")
validate_config
[[ $TIMEZONE == Asia/Seoul ]]
for zone in '../etc/passwd' 'Unknown/Zone' $'Asia/Seoul\n' 'Asia/Seoul;id'; do
    reject "$(jq --arg zone "$zone" '.timezone=$zone' <<< "$GOOD")"
done
reject "$(jq '.timezone=1' <<< "$GOOD")"
CONFIG=$GOOD
validate_config
pass 'timezone allowlist rejects traversal and invalid values with UTC compatibility'
reject "$(jq '.username="root"' <<< "$GOOD")"
reject "$(jq '.username="alice;reboot"' <<< "$GOOD")"
reject "$(jq '.hostname="bad\nname"' <<< "$GOOD")"
reject "$(jq '.disk="/dev/../sda"' <<< "$GOOD")"
reject "$(jq '.confirmation="yes"' <<< "$GOOD")"
reject "$(jq '.password="leaked"' <<< "$GOOD")"
reject "$(jq '.interface="eth0"' <<< "$GOOD")"
reject "$(jq '.version=2' <<< "$GOOD")"
pass 'reject invalid schema, injection, confirmation, and account values'
for ip in 127.0.0.1 192.168.50.10 0.0.0.0 255.255.255.255; do valid_ipv4 "$ip"; done
for ip in 300.1.1.1 01.1.1.1 1.2.3 1.2.3.4/24 '1.2.3.4;id'; do
    if valid_ipv4 "$ip"; then exit 1; fi
done
[[ $(ipv4_number 192.168.1.10) == 3232235786 ]]
pass 'strict IPv4 parsing and numeric conversion'
for interface_path in /sys/class/net/*; do
    interface_name=${interface_path##*/}
    [[ $interface_name != lo ]] || continue
    STATIC=$(jq --arg iface "$interface_name" '.network="static"|.interface=$iface|.address="192.168.50.10/24"|.gateway="192.168.50.1"|.dns="1.1.1.1"' <<< "$GOOD")
    CONFIG=$STATIC
    validate_config
    reject "$(jq '.address="192.168.50.0/24"' <<< "$STATIC")"
    reject "$(jq '.address="192.168.50.255/24"' <<< "$STATIC")"
    reject "$(jq '.address="127.0.0.2/24"' <<< "$STATIC")"
    reject "$(jq '.address="169.254.1.2/24"' <<< "$STATIC")"
    reject "$(jq '.address="192.168.50.10/31"' <<< "$STATIC")"
    reject "$(jq '.gateway="192.168.51.1"' <<< "$STATIC")"
    reject "$(jq '.gateway="192.168.50.10"' <<< "$STATIC")"
    reject "$(jq '.dns="224.0.0.1"' <<< "$STATIC")"
    pass 'static IPv4 subnet, unicast, and prefix validation'
    break
done
[[ $(partition_path /dev/sda 1) == /dev/sda1 ]]
[[ $(partition_path /dev/vda 2) == /dev/vda2 ]]
[[ $(partition_path /dev/nvme0n1 2) == /dev/nvme0n1p2 ]]
[[ $(partition_path /dev/mmcblk0 1) == /dev/mmcblk0p1 ]]
pass 'SATA, virtio, NVMe, and MMC partition naming'
event stage running bootstrap > "$TEST_DIR/event"
jq -e '.version==1 and .type=="stage" and .status=="running" and .name=="bootstrap"' "$TEST_DIR/event" >/dev/null
event result failed '' 'A "quoted" message' | jq -e '.message=="A \"quoted\" message"' >/dev/null
pass 'JSON protocol escaping and schema'
TARGET=$TEST_DIR/target
mkdir -p "$TARGET/etc"
ROOT_UUID=aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee
EFI_UUID=ABCD-1234
write_fstab
[[ $(wc -l < "$TARGET/etc/fstab") == 6 ]]
[[ $(awk '$3=="btrfs"{n++}END{print n}' "$TARGET/etc/fstab") == 5 ]]
if grep -q '/dev/' "$TARGET/etc/fstab"; then exit 1; fi
grep -q 'subvol=@snapshots' "$TARGET/etc/fstab"
[[ $(stat -c %a "$TARGET/etc/fstab") == 644 ]]
pass 'UUID fstab includes every subvolume and EFI'
enable_service() { [[ $1 == dhcpcd ]]; }
mkdir -p "$TARGET/usr/libexec/dhcpcd-hooks"
printf '# Resolver hook fixture\n' > "$TARGET/usr/libexec/dhcpcd-hooks/20-resolv.conf"
printf 'duid\npersistent\noption static_routes, classless_static_routes\nrequire dhcp_server_identifier\n' > "$TARGET/etc/dhcpcd.conf"
NETWORK=static INTERFACE=eth0 ADDRESS=192.168.1.10/24 GATEWAY=192.168.1.1 DNS=1.1.1.1
NETWORK_MAC=52:54:00:12:34:56
configure_network
grep -qx 'static ip_address=192.168.1.10/24' "$TARGET/etc/dhcpcd.conf"
grep -qx 'static routers=192.168.1.1' "$TARGET/etc/dhcpcd.conf"
grep -qx 'interface voidnet0' "$TARGET/etc/dhcpcd.conf"
grep -q 'ATTR{address}=="52:54:00:12:34:56", NAME="voidnet0"' "$TARGET/etc/udev/rules.d/10-void-installer-network.rules"
NETWORK=dhcp
configure_network
if grep -q 'static ' "$TARGET/etc/dhcpcd.conf"; then exit 1; fi
grep -qx duid "$TARGET/etc/dhcpcd.conf"
grep -qx 'require dhcp_server_identifier' "$TARGET/etc/dhcpcd.conf"
grep -qx 'option domain_name_servers, domain_name, domain_search' "$TARGET/etc/dhcpcd.conf"
[[ ! -e $TARGET/etc/udev/rules.d/10-void-installer-network.rules ]]
configure_network
[[ $(grep -c '^# BEGIN Void TUI Installer$' "$TARGET/etc/dhcpcd.conf") == 1 ]]
mv "$TARGET/usr/libexec/dhcpcd-hooks/20-resolv.conf" "$TARGET/resolver-hook"
if (verify_network_configuration) >/dev/null 2>&1; then exit 1; fi
mv "$TARGET/resolver-hook" "$TARGET/usr/libexec/dhcpcd-hooks/20-resolv.conf"
pass 'persistent DHCP and static dhcpcd configuration'
for desktop in none kde niri; do
    CONFIG=$(jq --arg desktop "$desktop" '.desktop=$desktop|.graphics="default"|.utilities=["firefox","kitty","fcitx5-hangul"]' <<< "$GOOD")
    validate_config
    (
        select_packages
        [[ " ${PACKAGES[*]} " == *' bluez '* && " ${PACKAGES[*]} " == *' tlp '* ]]
        [[ " ${PACKAGES[*]} " == *' firefox '* && " ${PACKAGES[*]} " == *' kitty '* && " ${PACKAGES[*]} " == *' fcitx5 '* ]]
        [[ " ${PACKAGES[*]} " != *' kde-plasma '* ]]
        if [[ $DESKTOP == none ]]; then [[ " ${PACKAGES[*]} " != *' sddm '* ]]; else [[ " ${PACKAGES[*]} " == *' sddm '* ]]; fi
    )
done
CONFIG=$GOOD
validate_config
reject "$(jq '.desktop="gnome"' <<< "$GOOD")"
reject "$(jq '.graphics="nvidia;reboot"' <<< "$GOOD")"
reject "$(jq '.utilities=["--force"]' <<< "$GOOD")"
reject "$(jq '.utilities=["firefox","firefox"]' <<< "$GOOD")"
reject "$(jq '.utilities=false' <<< "$GOOD")"
reject "$(jq '.utilities=["firefox\nkitty"]' <<< "$GOOD")"
pass 'software profile allowlists reject injection and preserve minimal profiles'
(
    DESKTOP=kde NETWORK=static NETWORK_MAC=52:54:00:12:34:56 ADDRESS=192.168.1.10/24 GATEWAY=192.168.1.1 DNS=1.1.1.1
    mkdir -p "$TARGET/etc/runit/runsvdir/default"
    ln -s /etc/sv/dhcpcd "$TARGET/etc/runit/runsvdir/default/dhcpcd"
    enable_service() { [[ $1 == NetworkManager ]]; }
    configure_network
    connection=$TARGET/etc/NetworkManager/system-connections/void-installer.nmconnection
    grep -qx 'method=manual' "$connection"
    grep -qx 'address1=192.168.1.10/24,192.168.1.1' "$connection"
    grep -qx 'mac-address=52:54:00:12:34:56' "$connection"
    grep -qx 'dns=1.1.1.1;' "$connection"
    [[ $(stat -c %a "$connection") == 600 && ! -L $TARGET/etc/runit/runsvdir/default/dhcpcd ]]
    NETWORK=dhcp
    configure_network
    grep -qx 'method=auto' "$connection"
    if grep -q 'address1=' "$connection"; then exit 1; fi
)
pass 'NetworkManager static and DHCP profiles have private permissions and no competing dhcpcd'
(
    DESKTOP=niri NETWORK=static NETWORK_MAC=52:54:00:12:34:56 ADDRESS=192.168.1.10/24 GATEWAY=192.168.1.1 DNS=1.1.1.1
    ln -s /etc/sv/wpa_supplicant "$TARGET/etc/runit/runsvdir/default/wpa_supplicant"
    enable_service() { [[ $1 == NetworkManager ]]; }
    configure_network
    connection=$TARGET/etc/NetworkManager/system-connections/void-installer.nmconnection
    grep -qx 'method=manual' "$connection"
    grep -qx 'dns=1.1.1.1;' "$connection"
    [[ ! -L $TARGET/etc/runit/runsvdir/default/wpa_supplicant ]]
    NETWORK=dhcp
    configure_network
    grep -qx 'method=auto' "$connection"
    if grep -q 'address1=' "$connection"; then exit 1; fi
)
pass 'niri uses NetworkManager for DHCP and static networking without a competing supplicant'
(
    TIMEZONE=Asia/Seoul
    mkdir -p "$TARGET/usr/share/zoneinfo/Asia"
    printf 'TZif fixture\n' > "$TARGET/usr/share/zoneinfo/Asia/Seoul"
    configure_timezone
    verify_timezone
    [[ $(readlink "$TARGET/etc/localtime") == /usr/share/zoneinfo/Asia/Seoul ]]
)
pass 'selected timezone is rendered and verified against installed tzdata'
(
    DESKTOP=niri USERNAME=alice UTILITIES=(firefox kitty fcitx5-hangul)
    chrun() { [[ $1 == chown ]]; }
    configure_niri
    configure_input_method
    grep -q 'spawn "kitty"' "$TARGET/home/alice/.config/niri/config.kdl"
    grep -q 'spawn "firefox"' "$TARGET/home/alice/.config/niri/config.kdl"
    grep -q 'spawn-at-startup "fcitx5"' "$TARGET/home/alice/.config/niri/config.kdl"
    grep -qx 'Name=hangul' "$TARGET/home/alice/.config/fcitx5/profile"
    grep -qx 'spawn-at-startup "/usr/libexec/polkit-gnome-authentication-agent-1"' "$TARGET/home/alice/.config/niri/config.kdl"
    grep -qx 'xwayland-satellite { path "/usr/bin/xwayland-satellite"; }' "$TARGET/home/alice/.config/niri/config.kdl"
    grep -qx 'spawn-at-startup "xdg-user-dirs-update"' "$TARGET/home/alice/.config/niri/config.kdl"
    if grep -q 'spawn-at-startup "xwayland-satellite"' "$TARGET/home/alice/.config/niri/config.kdl"; then exit 1; fi
    UTILITIES=()
    configure_niri
    if grep -Eq 'kitty|firefox|fcitx5' "$TARGET/home/alice/.config/niri/config.kdl"; then exit 1; fi
)
pass 'niri config launches only selected applications and Hangul profile is generated'
(
    for root in https://repo-fastly.voidlinux.org https://mirror.example/voidlinux/ https://mirror.example/void/; do
        configure_mirror_root "$root"
        [[ $REPOSITORY == "${root%/}/current" ]]
    done
    mkdir -p "$TARGET/usr/share/xbps.d"
    printf 'repository=https://repo-default.voidlinux.org/current\n' > "$TARGET/usr/share/xbps.d/00-repository-main.conf"
    printf 'repository=https://repo-default.voidlinux.org/current/nonfree\n' > "$TARGET/usr/share/xbps.d/10-repository-nonfree.conf"
    configure_mirror_root https://mirror.example/voidlinux
    persist_mirror
    grep -qx 'repository=https://mirror.example/voidlinux/current' "$TARGET/etc/xbps.d/00-repository-main.conf"
    grep -qx 'repository=https://mirror.example/voidlinux/current/nonfree' "$TARGET/etc/xbps.d/10-repository-nonfree.conf"
    [[ $(stat -c %a "$TARGET/etc/xbps.d/00-repository-main.conf") == 644 ]]
)
for root in 'http://mirror.example/void' 'https://mirror.example/current' 'https://mirror.example/voidlinux/current' 'https://mirror.example/../void' 'https://user@mirror.example/void' 'https://mirror.example/?x=1'; do
    code=0
    bash -Eeuo pipefail -c 'source "$1/scripts/lib/common.sh"; source "$1/scripts/lib/mirror.sh"; configure_mirror_root "$2"' _ "$ROOT_DIR" "$root" >/dev/null 2>&1 || code=$?
    (( code != 0 ))
done
pass 'mirror roots reject injection and repository suffixes and persist main and nonfree overrides'
(
    DESKTOP=kde USERNAME=alice
    mkdir -p "$TARGET"/usr/share/examples/{pipewire,wireplumber} "$TARGET/usr/share/alsa/alsa.conf.d" "$TARGET/usr/share/applications" "$TARGET/usr/lib/security" "$TARGET/etc/pam.d"
    touch "$TARGET/usr/share/examples/pipewire/20-pipewire-pulse.conf" "$TARGET/usr/share/examples/wireplumber/10-wireplumber.conf" "$TARGET/usr/share/alsa/alsa.conf.d/50-pipewire.conf" "$TARGET/usr/share/alsa/alsa.conf.d/99-pipewire-default.conf" "$TARGET/usr/share/applications/pipewire.desktop" "$TARGET/usr/lib/security/pam_kwallet5.so" "$TARGET/usr/lib/security/pam_gnome_keyring.so"
    printf 'auth include system-login\n-auth optional pam_kwallet5.so\nsession include system-login\n-session optional pam_kwallet5.so auto_start\n' > "$TARGET/etc/pam.d/sddm"
    chrun() { [[ $1 == chown || $1 == ldconfig ]]; }
    configure_audio
    configure_wallet
    configure_wallet
    [[ $(grep -c '^auth optional pam_kwallet5.so$' "$TARGET/etc/pam.d/sddm") == 1 ]]
    [[ -L $TARGET/home/alice/.config/autostart/pipewire.desktop && -L $TARGET/etc/pipewire/pipewire.conf.d/10-wireplumber.conf ]]
    DESKTOP=niri
    configure_wallet
    configure_portals
    grep -qx 'session optional pam_gnome_keyring.so auto_start' "$TARGET/etc/pam.d/sddm"
    grep -qx 'org.freedesktop.impl.portal.FileChooser=gtk;' "$TARGET/home/alice/.config/xdg-desktop-portal/niri-portals.conf"
    grep -qx 'org.freedesktop.impl.portal.Secret=gnome-keyring;' "$TARGET/home/alice/.config/xdg-desktop-portal/niri-portals.conf"
)
pass 'audio startup, optional PAM integration and GTK file chooser are configured without duplicate wallet hooks'
(
    GRAPHICS=nvidia WORK=$TEST_DIR VOID_INSTALLER_HELPER=/bin/true
    PACKAGES=(base-system nvidia linux-headers)
    timeout() { printf 'nvidia-595.104.02_1\n'; }
    check_graphics
    [[ ${PACKAGES[1]} == nvidia-595.104.02_1 ]]
)
code=0
bash -Eeuo pipefail -c '
    source "$1/scripts/lib/common.sh"
    source "$1/scripts/lib/software.sh"
    GRAPHICS=nvidia WORK=$2 VOID_INSTALLER_HELPER=/bin/false
    timeout() { printf "nvidia-595.104.02_1\n"; }
    check_graphics
    printf UNREACHABLE
' _ "$ROOT_DIR" "$TEST_DIR" > "$TEST_DIR/nvidia-rejected" 2>&1 || code=$?
(( code != 0 ))
if grep -q UNREACHABLE "$TEST_DIR/nvidia-rejected"; then exit 1; fi
pass 'NVIDIA verification failure stops installation and approved versions are pinned'
# check_disk must stop before invoking any storage operation for a regular file.
DISK=$TEST_DIR/not-a-device IDENTITY=anything
touch "$DISK"
if bash -Eeuo pipefail -c 'source "$1/scripts/lib/common.sh"; source "$1/scripts/lib/disk.sh"; DISK=$2; IDENTITY=x; check_disk' _ "$ROOT_DIR" "$DISK" >/dev/null 2>&1; then exit 1; fi
pass 'regular-file target is rejected'
# Password input travels on FD 3, survives shell punctuation, and stays off output.
# shellcheck disable=SC2016
SECRET='Test$(never-run)"Pass123'
bash -Eeuo pipefail -c 'source "$1/scripts/lib/common.sh"; source "$1/scripts/lib/mirror.sh"; source "$1/scripts/lib/config.sh"; SCRIPT_DIR=$1/scripts; read_config; [[ ${#PASSWORD} -gt 12 ]]' _ "$ROOT_DIR" <<< "$GOOD" 3<<< "$SECRET" > "$TEST_DIR/secret-output" 2>&1
[[ ! -s $TEST_DIR/secret-output ]]
for secret in short 1234567 'Invalid:Password123' $'Invalid\tPassword123'; do
    if bash -Eeuo pipefail -c 'source "$1/scripts/lib/common.sh"; source "$1/scripts/lib/mirror.sh"; source "$1/scripts/lib/config.sh"; SCRIPT_DIR=$1/scripts; read_config' _ "$ROOT_DIR" <<< "$GOOD" 3<<< "$secret" >/dev/null 2>&1; then exit 1; fi
done
bash -Eeuo pipefail -c 'source "$1/scripts/lib/common.sh"; source "$1/scripts/lib/mirror.sh"; source "$1/scripts/lib/config.sh"; SCRIPT_DIR=$1/scripts; read_config; [[ ${#PASSWORD} == 8 ]]' _ "$ROOT_DIR" <<< "$GOOD" 3<<< '12345678'
pass 'private secret descriptor and password validation'
if bash -Eeuo pipefail -c 'source "$1/scripts/lib/common.sh"; source "$1/scripts/lib/mirror.sh"; source "$1/scripts/lib/config.sh"; SCRIPT_DIR=$1/scripts; read_config' _ "$ROOT_DIR" <<< "$GOOD" 3< <(printf 'ValidSecret123\000Hidden\n') >/dev/null 2>&1; then exit 1; fi
if bash -Eeuo pipefail -c 'source "$1/scripts/lib/common.sh"; source "$1/scripts/lib/mirror.sh"; source "$1/scripts/lib/config.sh"; SCRIPT_DIR=$1/scripts; read_config' _ "$ROOT_DIR" <<< "$GOOD" 3< <(printf 'ValidSecret123\nExtraLine\n') >/dev/null 2>&1; then exit 1; fi
pass 'raw secret validation rejects NUL bytes and additional lines'
for fuser_case in none active diagnostic unexpected; do
    code=0
    bash -Eeuo pipefail -c '
        source "$1/scripts/lib/common.sh"
        source "$1/scripts/lib/disk.sh"
        fuser() {
            case "$1" in /dev/test) ;; *) return 2;; esac
            case "$MODE" in
                none) return 1;;
                active) return 0;;
                diagnostic) printf "Cannot inspect process\n" >&2; return 1;;
                unexpected) return 2;;
            esac
        }
        MODE=$2
        check_device_users /dev/test
    ' _ "$ROOT_DIR" "$fuser_case" >/dev/null 2>&1 || code=$?
    if [[ $fuser_case == none ]]; then [[ $code == 0 ]]; else [[ $code != 0 ]]; fi
done
pass 'active raw-device users and uncertain fuser results are rejected'
# Verify cleanup failures are visible and preserve the private directory.
source "$ROOT_DIR/scripts/lib/common.sh"
TARGET=$TEST_DIR/mounted WORK=$TEST_DIR/work TOP=
mkdir "$WORK"
mountpoint() { return 0; }
umount() { return 1; }
if cleanup 2> "$TEST_DIR/cleanup-log"; then exit 1; fi
[[ -d $WORK && -s $TEST_DIR/cleanup-log ]]
pass 'failed unmount is reported and resources are preserved'
# Failure paths must report the failed stage, explicit cleanup, and final result.
bash -Eeuo pipefail -c '
    source "$1/scripts/lib/common.sh"
    STAGE=bootstrap
    cleanup() { return 0; }
    trap on_exit EXIT
    exit 9
' _ "$ROOT_DIR" > "$TEST_DIR/failure-events" || code=$?
[[ ${code:-0} == 9 ]]
jq -se 'length==4 and .[0].type=="stage" and .[0].name=="bootstrap" and .[0].status=="failed" and .[1].name=="cleanup" and .[1].status=="running" and .[2].status=="completed" and .[3].type=="result" and .[3].status=="failed"' "$TEST_DIR/failure-events" >/dev/null
pass 'failure event sequence includes cleanup and final failure'
# Live-medium ancestry is independently checked even with no mounted descendants.
bash -Eeuo pipefail -c '
    source "$1/scripts/lib/common.sh"
    source "$1/scripts/lib/disk.sh"
    DISK=/dev/vda
    cat() { printf "root=live:UUID=test\n"; }
    resolve_live_source() { printf "/dev/vda1\n"; }
    lsblk() { printf "/dev/vda1\n/dev/vda\n"; }
    check_live_source
' _ "$ROOT_DIR" > "$TEST_DIR/live-output" 2>&1 && exit 1
grep -q 'live installation medium' "$TEST_DIR/live-output"
pass 'unmounted live-medium ancestor is rejected'
if bash -Eeuo pipefail -c '
    source "$1/scripts/lib/common.sh"
    source "$1/scripts/lib/disk.sh"
    blkid() { printf "/dev/vda1\n/dev/vdb1\n"; }
    resolve_live_source LABEL=VOID_LIVE
' _ "$ROOT_DIR" > "$TEST_DIR/ambiguous-live" 2>&1; then exit 1; fi
grep -q 'ambiguous' "$TEST_DIR/ambiguous-live"
pass 'duplicated live-medium identifiers are rejected before any disk operation'
for failure_stage in bootstrap grub snapper; do
    failure_dir=$TEST_DIR/failure-$failure_stage
    mkdir -p "$failure_dir/etc/default" "$failure_dir/usr/share/snapper/config-templates"
    printf 'TIMELINE_CREATE="yes"\n' > "$failure_dir/usr/share/snapper/config-templates/default"
    code=0
    bash -Eeuo pipefail -c '
        source "$1/scripts/install.sh"
        TARGET=$2
        cleanup() { return 0; }
        cp() { if [[ $1 == *config-templates/default ]]; then command cp "$@"; fi; }
        run() { return 23; }
        chrun() { return 23; }
        trap on_exit EXIT
        case "$3" in
            bootstrap) stage bootstrap bootstrap;;
            grub) stage grub configure_grub;;
            snapper) stage snapper configure_snapper;;
        esac
        printf "Unexpected continuation\n"
    ' _ "$ROOT_DIR" "$failure_dir" "$failure_stage" > "$failure_dir/events" 2> "$failure_dir/log" || code=$?
    [[ $code == 23 ]]
    jq -se --arg stage "$failure_stage" 'length==5 and .[0].name==$stage and .[0].status=="running" and .[1].name==$stage and .[1].status=="failed" and .[2].name=="cleanup" and .[3].status=="completed" and .[4].type=="result" and .[4].status=="failed"' "$failure_dir/events" >/dev/null
    pass "$failure_stage command failure stops the engine and cleans up"
done
# A successful chpasswd exit is insufficient if PAM leaves accounts unchanged.
account_dir=$TEST_DIR/account-regression
mkdir -p "$account_dir/etc/default"
printf '#en_US.UTF-8 UTF-8\n' > "$account_dir/etc/default/libc-locales"
printf 'root:!:1:0:99999:7:::\nalice:!:1:0:99999:7:::\n' > "$account_dir/etc/shadow"
code=0
bash -Eeuo pipefail -c '
    source "$1/scripts/install.sh"
    TARGET=$2 USERNAME=alice HOSTNAME=voidbox PASSWORD=RegressionSecret123
    write_fstab() { return 0; }
    configure_timezone() { return 0; }
    chrun() { return 0; }
    chroot() {
        if [[ $2 == chpasswd ]]; then command cat >/dev/null; return 0; fi
        local -a args=("${@:2}")
        args[${#args[@]}-1]="$TARGET/etc/shadow"
        "${args[@]}"
    }
    configure_system
' _ "$ROOT_DIR" "$account_dir" > "$account_dir/output" 2>&1 || code=$?
[[ $code != 0 ]]
grep -q 'Account password hash verification failed' "$account_dir/output"
if grep -q 'RegressionSecret123' "$account_dir/output"; then exit 1; fi
pass 'PAM no-op password success is rejected without exposing the secret'
snapper_dir=$TEST_DIR/snapper-policy
mkdir -p "$snapper_dir/usr/share/snapper/config-templates" "$snapper_dir/etc/snapper/configs" "$snapper_dir/etc/conf.d" "$snapper_dir/etc/cron.hourly"
printf '%s\n' 'TIMELINE_CREATE="yes"' 'TIMELINE_CLEANUP="yes"' 'TIMELINE_LIMIT_HOURLY="10"' 'TIMELINE_LIMIT_DAILY="10"' 'TIMELINE_LIMIT_WEEKLY="0"' 'TIMELINE_LIMIT_MONTHLY="10"' 'TIMELINE_LIMIT_YEARLY="10"' > "$snapper_dir/usr/share/snapper/config-templates/default"
printf 'SNAPPER_CONFIGS="root"\n' > "$snapper_dir/etc/conf.d/snapper"
touch "$snapper_dir/etc/cron.hourly/snapper"
chmod +x "$snapper_dir/etc/cron.hourly/snapper"
bash -Eeuo pipefail -c '
    source "$1/scripts/install.sh"
    TARGET=$2
    run() { return 0; }
    enable_service() { return 0; }
    mount_subvolume() { mkdir -p "$TARGET/.snapshots"; }
    chrun() {
        if [[ $5 == create-config ]]; then
            [[ $6 == --template && $7 == installer-root ]]
            grep -qx '\''TIMELINE_CREATE="no"'\'' "$TARGET/etc/snapper/config-templates/installer-root"
            cp "$TARGET/etc/snapper/config-templates/installer-root" "$TARGET/etc/snapper/configs/root"
        elif [[ $5 == create ]]; then
            mkdir -p "$TARGET/.snapshots/1"
            printf "metadata\n" > "$TARGET/.snapshots/1/info.xml"
        fi
    }
    configure_snapper
' _ "$ROOT_DIR" "$snapper_dir"
grep -qx 'TIMELINE_CREATE="yes"' "$snapper_dir/etc/snapper/configs/root"
grep -qx 'TIMELINE_LIMIT_HOURLY="5"' "$snapper_dir/etc/snapper/configs/root"
grep -qx 'TIMELINE_LIMIT_YEARLY="0"' "$snapper_dir/etc/snapper/configs/root"
pass 'Snapper creation avoids systemd while enabling the final cron timeline policy'
code=0
bash -Eeuo pipefail -c '
    source "$1/scripts/lib/preflight.sh"
    preflight_command "Package transaction resolution" bash -c "printf '\''package plan only\\n'\''; exit 23"
    printf "UNREACHABLE\n"
' _ "$ROOT_DIR" > "$TEST_DIR/preflight-error" 2>&1 || code=$?
[[ $code == 23 ]]
grep -q 'Preflight failed: Package transaction resolution (exit status 23)' "$TEST_DIR/preflight-error"
if grep -q UNREACHABLE "$TEST_DIR/preflight-error"; then exit 1; fi
pass 'preflight identifies the failed operation and preserves its exit status'
for scenario in tmpfs normal conflict malformed unexpected-error small-target; do
    mkdir "$TEST_DIR/space-$scenario"
    code=0
    bash -Eeuo pipefail -c '
        source "$1/scripts/lib/common.sh"
        source "$1/scripts/lib/preflight.sh"
        scenario=$3
        timeout() {
            if [[ $scenario == malformed ]]; then printf "invalid plan\n"; else
                printf "base-system-1_1 install x86_64 %s 1214251008 500000000\n" "$REPOSITORY"
            fi
            if [[ $scenario == conflict ]]; then printf "ERROR: Transaction aborted due to conflicting packages.\n" >&2; return 11; fi
            if [[ $scenario != normal ]]; then
                printf "ERROR: Transaction aborted due to insufficient disk space (need 1158MB, got 981MB free).\n" >&2
                if [[ $scenario == unexpected-error ]]; then printf "ERROR: unexpected repository failure\n" >&2; fi
                return 28
            fi
        }
        preflight_transaction "$2"
        [[ $PREFLIGHT_REQUIRED_BYTES == 3861734656 ]]
        if [[ $scenario == small-target ]]; then check_target_capacity 4000000000; else check_target_capacity 17179869184; fi
    ' _ "$ROOT_DIR" "$TEST_DIR/space-$scenario" "$scenario" > "$TEST_DIR/space-$scenario.out" 2>&1 || code=$?
    case "$scenario" in
        tmpfs|normal) [[ $code == 0 ]];;
        *) (( code != 0 ));;
    esac
done
pass 'Live tmpfs ENOSPC is separated from real target capacity without accepting conflicts or malformed plans'
# Preparation fixtures mock the Live guard, lock and package manager; no host packages are changed.
for scenario in missing ready guard-failure update-failure install-failure; do
    code=0
    bash -Eeuo pipefail -c '
        source "$1/scripts/lib/common.sh"
        source "$1/scripts/lib/preflight.sh"
        scenario=$2
        log=$3
        installed=0
        preflight_live_environment() {
            printf "guard\n" >> "$log"
            if [[ $scenario == guard-failure ]]; then return 23; fi
        }
        acquire_installer_lock() { printf "lock\n" >> "$log"; }
        command() {
            if [[ $1 == -v && $2 == jq && $scenario != ready && $installed == 0 ]]; then return 1; fi
            if [[ $1 == -v ]]; then return 0; fi
            builtin command "$@"
        }
        xbps-install() {
            printf "%s\n" "$*" >> "$log"
            if [[ $* == "-Sy xbps" ]]; then
                if [[ $scenario == update-failure ]]; then return 37; fi
            else
                if [[ $scenario == install-failure ]]; then return 38; fi
                [[ $* == "-Sy jq gptfdisk parted dosfstools btrfs-progs psmisc" ]]
                installed=1
            fi
        }
        preflight_environment() {
            [[ $scenario == ready || $installed == 1 ]]
            printf "checked\n" >> "$log"
        }
        prepare_live_environment
    ' _ "$ROOT_DIR" "$scenario" "$TEST_DIR/prepare-$scenario.log" > "$TEST_DIR/prepare-$scenario.out" 2>&1 || code=$?
    case "$scenario" in
        missing)
            [[ $code == 0 ]]
            [[ $(cat "$TEST_DIR/prepare-$scenario.log") == $'guard\nlock\n-Sy xbps\n-Sy jq gptfdisk parted dosfstools btrfs-progs psmisc\nchecked' ]];;
        ready)
            [[ $code == 0 ]]
            [[ $(cat "$TEST_DIR/prepare-$scenario.log") == $'guard\nlock\nchecked' ]];;
        guard-failure)
            [[ $code == 23 && $(cat "$TEST_DIR/prepare-$scenario.log") == guard ]];;
        update-failure)
            [[ $code == 37 ]]
            [[ $(cat "$TEST_DIR/prepare-$scenario.log") == $'guard\nlock\n-Sy xbps' ]];;
        install-failure)
            [[ $code == 38 ]]
            ! grep -q checked "$TEST_DIR/prepare-$scenario.log";;
    esac
done
pass 'Live preparation installs missing tools after guards, skips ready sessions and stops on XBPS failures'
grep -q '^VOID_PREP environment running$' "$TEST_DIR/prepare-guard-failure.out"
if grep -q '^VOID_PREP environment completed$' "$TEST_DIR/prepare-guard-failure.out"; then exit 1; fi
grep -q '^VOID_PREP xbps skipped$' "$TEST_DIR/prepare-ready.out"
grep -q '^VOID_PREP install skipped$' "$TEST_DIR/prepare-ready.out"
grep -q '^VOID_PREP install running$' "$TEST_DIR/prepare-install-failure.out"
if grep -q '^VOID_PREP install completed$' "$TEST_DIR/prepare-install-failure.out"; then exit 1; fi
grep -q '^VOID_PREP install completed$' "$TEST_DIR/prepare-missing.out"
pass 'Live preparation reports running, skipped and completed states without marking failed work complete'
for scenario in partition_disk:2:0 partition_disk:3:1 create_filesystems:1:0 create_filesystems:2:1; do
    IFS=: read -r operation fail_at expected <<< "$scenario"
    operation_log=$TEST_DIR/storage-guard.log
    : > "$operation_log"
    code=0
    bash -Eeuo pipefail -c '
        source "$1/scripts/lib/common.sh"
        source "$1/scripts/lib/disk.sh"
        source "$1/scripts/lib/btrfs.sh"
        count=0
        check_static_interface() { :; }
        run() { printf "%s\n" "$*" >> "$operation_log"; }
        operation=$2
        fail_at=$3
        operation_log=$4
        check_disk() { count=$((count+1)); (( count < fail_at )) || fail "Fixture disk identity changed."; }
        DISK=/dev/fixture
        EFI=/dev/fixture1
        ROOT=/dev/fixture2
        "$operation"
    ' _ "$ROOT_DIR" "$operation" "$fail_at" "$operation_log" >/dev/null 2>&1 || code=$?
    (( code != 0 ))
    [[ $(wc -l < "$operation_log") -eq $expected ]]
done
pass 'changed disk state blocks each subsequent destructive command using mocks only'
for selection in vim missing vim-9.1_1; do
    package_log=$TEST_DIR/package-validation.log
    : > "$package_log"
    code=0
    bash -Eeuo pipefail -c '
        source "$1/scripts/lib/common.sh"
        source "$1/scripts/lib/preflight.sh"
        UTILITIES=("$2")
        package_log=$3
        REPOSITORY=https://mirror.example/current
        REPOSITORIES=(-R "$REPOSITORY")
        QUERY_REPOSITORIES=(--repository="$REPOSITORY")
        PACKAGES=(vim)
        mkdir() { :; }
        cp() { :; }
        preflight_command() { :; }
        timeout() {
            [[ $* == *"-i --repository=https://mirror.example/current -p pkgname"* ]] || return 1
            case "${*: -1}" in
                vim|vim-9.1_1) printf "vim\n";;
                *) return 1;;
            esac
        }
        preflight_transaction() { printf "transaction\n" >> "$package_log"; }
        preflight_repository /fixture
    ' _ "$ROOT_DIR" "$selection" "$package_log" >/dev/null 2>&1 || code=$?
    if [[ $selection == vim ]]; then
        (( code == 0 ))
        grep -qx transaction "$package_log"
    else
        (( code != 0 ))
        [[ ! -s $package_log ]]
    fi
done
pass 'selected packages require exact names in the chosen main repository before transaction resolution'
(
    # Private metadata and repository calls are replaced; no XBPS operation runs.
    source "$ROOT_DIR/scripts/lib/preflight.sh"
    CHECK_DIR=$TEST_DIR/package-list
    package_log=$TEST_DIR/package-list.log
    preflight_environment() { :; }
    mktemp() { command mkdir "$CHECK_DIR"; printf '%s\n' "$CHECK_DIR"; }
    cp() { [[ $1 == -a && $2 == /var/db/xbps/keys/. && $3 == "$CHECK_DIR/var/db/xbps/keys/" ]]; }
    timeout() {
        if [[ $1 == 180 ]]; then
            [[ $* == *"xbps-install -S -r $CHECK_DIR -i -R https://mirror.example/void/current"* ]]
            printf 'sync\n' >> "$package_log"
            printf 'Fixture synchronization diagnostics\n'
        else
            [[ $* == *"xbps-query -r $CHECK_DIR -i --repository=https://mirror.example/void/current -s *"* ]]
            printf 'query\n' >> "$package_log"
            printf '[-] vim-9.1_1 Text editor\n'
        fi
    }
    list_repository_packages https://mirror.example/void > "$TEST_DIR/package-list.out" 2> "$TEST_DIR/package-list.err"
    grep -qx '\[-\] vim-9.1_1 Text editor' "$TEST_DIR/package-list.out"
    [[ $(wc -l < "$TEST_DIR/package-list.out") -eq 1 ]]
    grep -qx 'Fixture synchronization diagnostics' "$TEST_DIR/package-list.err"
    [[ $(wc -l < "$package_log") -eq 2 ]]
)
pass 'package listing isolates metadata, ignores configured repositories and separates diagnostics from records'
for scenario in passed conflict; do
    code=0
    bash -Eeuo pipefail -c '
        source "$1/scripts/install.sh"
        scenario=$2
        preflight_environment() { :; }
        acquire_installer_lock() { :; }
        check_static_interface() { :; }
        preflight_repository() {
            if [[ $scenario == conflict ]]; then printf "Fixture dependency conflict\n" >&2; return 23; fi
            PREFLIGHT_REQUIRED_BYTES=4000000000
        }
        check_graphics() { :; }
        check_disk() { :; }
        lsblk() { printf "32000000000\n"; }
        check_installation_plan
    ' _ "$ROOT_DIR" "$scenario" <<< "$GOOD" > "$TEST_DIR/review-$scenario.out" 2> "$TEST_DIR/review-$scenario.err" || code=$?
    if [[ $scenario == passed ]]; then
        (( code == 0 ))
        jq -se '.[-1].name=="capacity" and .[-1].required_bytes==4000000000 and .[-1].disk_bytes==32000000000 and any(.[];.name=="graphics" and .status=="skipped")' "$TEST_DIR/review-$scenario.out" >/dev/null
    else
        (( code == 23 ))
        jq -se '.[-1].name=="repository" and .[-1].status=="running" and all(.[];.name!="disk")' "$TEST_DIR/review-$scenario.out" >/dev/null
        grep -qx 'Fixture dependency conflict' "$TEST_DIR/review-$scenario.err"
    fi
done
pass 'review checks use public config without FD 3 and retain partial results on conflicts'
account_dir=$TEST_DIR/locked-root
mkdir -p "$account_dir/etc/default"
printf '#en_US.UTF-8 UTF-8\n' > "$account_dir/etc/default/libc-locales"
bash -Eeuo pipefail -c '
    source "$1/scripts/install.sh"
    TARGET=$2 USERNAME=alice HOSTNAME=voidbox PASSWORD=FixtureSecret123
    write_fstab() { :; }
    configure_timezone() { :; }
    chrun() {
        if [[ $1 == usermod ]]; then
            [[ $* == "usermod -L root" ]]
            sed -i "s/^root:[^:]*:/root:!:/" "$TARGET/etc/shadow"
        fi
    }
    chroot() {
        if [[ $2 == chpasswd ]]; then
            command cat > "$TARGET/password-input"
            printf "root:x:1:0:99999:7:::\nalice:\$6\$%080d:1:0:99999:7:::\n" 0 > "$TARGET/etc/shadow"
            return
        fi
        local -a args=("${@:2}")
        args[${#args[@]}-1]="$TARGET/etc/shadow"
        "${args[@]}"
    }
    configure_system
' _ "$ROOT_DIR" "$account_dir" > "$account_dir/output" 2>&1
[[ $(wc -l < "$account_dir/password-input") -eq 1 ]]
grep -q '^alice:' "$account_dir/password-input"
if grep -q '^root:' "$account_dir/password-input"; then exit 1; fi
grep -q '^root:!:' "$account_dir/etc/shadow"
grep -qx '%wheel ALL=(ALL:ALL) ALL' "$account_dir/etc/sudoers.d/10-wheel"
if grep -q 'FixtureSecret123' "$account_dir/output"; then exit 1; fi
pass 'only the user receives a password while root is locked and wheel sudo requires authentication'
printf '1..%s\n' "$count"
