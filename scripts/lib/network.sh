#!/usr/bin/env bash
check_static_interface() {
    [[ $NETWORK == static ]] || return 0
    local directory=/sys/class/net/$INTERFACE mac first_octet
    [[ -d $directory/device && ! -d $directory/wireless && -r $directory/type && $(cat "$directory/type") == 1 ]] || fail 'Static networking requires a physical wired Ethernet interface.'
    mac=$(cat "$directory/address")
    [[ $mac =~ ^([0-9a-f]{2}:){5}[0-9a-f]{2}$ && $mac != 00:00:00:00:00:00 ]] || fail 'The network interface has no valid hardware address.'
    first_octet=${mac%%:*}
    (( (16#$first_octet & 1) == 0 )) || fail 'The network interface hardware address is not unicast.'
    if [[ -n ${NETWORK_MAC:-} ]]; then
        [[ $NETWORK_MAC == "$mac" ]] || fail 'The selected network interface identity changed.'
    else
        NETWORK_MAC=$mac
    fi
}
configure_network() {
    if [[ ${DESKTOP:-none} == kde || ${DESKTOP:-none} == niri ]]; then configure_networkmanager; return; fi
    [[ -f $TARGET/etc/dhcpcd.conf && ! -L $TARGET/etc/dhcpcd.conf ]] || fail 'Packaged dhcpcd configuration is unavailable.'
    # Preserve the packaged defaults, including DHCP option requests and hooks.
    # Replace only our own block when configuration is rendered again.
    sed -i '/^# BEGIN Void TUI Installer$/,/^# END Void TUI Installer$/d' "$TARGET/etc/dhcpcd.conf"
    cat >> "$TARGET/etc/dhcpcd.conf" <<'DHCP_OPTIONS'

# BEGIN Void TUI Installer
# Request DNS servers and search domains from the DHCP server.
option domain_name_servers, domain_name, domain_search
DHCP_OPTIONS
    if [[ $NETWORK == static ]]; then
        [[ ${NETWORK_MAC:-} =~ ^([0-9a-f]{2}:){5}[0-9a-f]{2}$ ]] || fail 'The selected Ethernet hardware identity is unavailable.'
        mkdir -p "$TARGET/etc/udev/rules.d"
        chmod 0755 "$TARGET/etc/udev" "$TARGET/etc/udev/rules.d"
        printf 'SUBSYSTEM=="net", ACTION=="add", ATTR{address}=="%s", NAME="voidnet0"\n' "$NETWORK_MAC" > "$TARGET/etc/udev/rules.d/10-void-installer-network.rules"
        chmod 0644 "$TARGET/etc/udev/rules.d/10-void-installer-network.rules"
        cat >> "$TARGET/etc/dhcpcd.conf" <<NETWORK_CONFIG
allowinterfaces voidnet0
interface voidnet0
static ip_address=$ADDRESS
static routers=$GATEWAY
static domain_name_servers=$DNS
NETWORK_CONFIG
    else
        if [[ -f $TARGET/etc/udev/rules.d/10-void-installer-network.rules ]]; then
            rm -- "$TARGET/etc/udev/rules.d/10-void-installer-network.rules"
        fi
    fi
    printf '# END Void TUI Installer\n' >> "$TARGET/etc/dhcpcd.conf"
    chmod 0644 "$TARGET/etc/dhcpcd.conf"
    verify_network_configuration
    enable_service dhcpcd
}
verify_network_configuration() {
    if [[ ${DESKTOP:-none} == kde || ${DESKTOP:-none} == niri ]]; then
        [[ -f $TARGET/etc/NetworkManager/system-connections/void-installer.nmconnection && $(stat -c %a "$TARGET/etc/NetworkManager/system-connections/void-installer.nmconnection") == 600 ]] || fail 'NetworkManager connection is missing or has unsafe permissions.'
        local service
        for service in dhcpcd wpa_supplicant connmand; do
            [[ ! -e $TARGET/etc/runit/runsvdir/default/$service && ! -L $TARGET/etc/runit/runsvdir/default/$service ]] || fail 'A conflicting network service is enabled.'
        done
        return
    fi
    grep -Eq '^option[[:space:]]+domain_name_servers,[[:space:]]*domain_name,[[:space:]]*domain_search[[:space:]]*$' "$TARGET/etc/dhcpcd.conf" || fail 'DHCP DNS option requests are missing.'
    [[ -r $TARGET/usr/libexec/dhcpcd-hooks/20-resolv.conf ]] || fail 'The dhcpcd DNS resolver hook is missing.'
}
configure_networkmanager() {
    local directory=$TARGET/etc/NetworkManager/system-connections
    mkdir -p "$directory" "$TARGET/etc/NetworkManager/conf.d"
    chmod 0700 "$directory"
    # Use NetworkManager's internal DHCP client and direct DNS-file management.
    printf '[main]\ndhcp=internal\ndns=default\nrc-manager=file\n' > "$TARGET/etc/NetworkManager/conf.d/10-void-installer.conf"
    chmod 0644 "$TARGET/etc/NetworkManager/conf.d/10-void-installer.conf"
    printf '[connection]\nid=void-installer\nuuid=89d03ed4-2ef0-4a2b-bcc6-42a5e6c4637e\ntype=ethernet\nautoconnect=true\n\n[ethernet]\n' > "$directory/void-installer.nmconnection"
    if [[ $NETWORK == static ]]; then
        [[ ${NETWORK_MAC:-} =~ ^([0-9a-f]{2}:){5}[0-9a-f]{2}$ ]] || fail 'The selected Ethernet hardware identity is unavailable.'
        printf 'mac-address=%s\n\n[ipv4]\nmethod=manual\naddress1=%s,%s\ndns=%s;\nignore-auto-dns=true\n' "$NETWORK_MAC" "$ADDRESS" "$GATEWAY" "$DNS" >> "$directory/void-installer.nmconnection"
    else
        printf '\n[ipv4]\nmethod=auto\n' >> "$directory/void-installer.nmconnection"
    fi
    printf '\n[ipv6]\nmethod=auto\n' >> "$directory/void-installer.nmconnection"
    chmod 0600 "$directory/void-installer.nmconnection"
    local service
    for service in dhcpcd wpa_supplicant connmand; do
        if [[ -L $TARGET/etc/runit/runsvdir/default/$service ]]; then rm -- "$TARGET/etc/runit/runsvdir/default/$service"; fi
    done
    enable_service NetworkManager
    verify_network_configuration
}
