#!/usr/bin/env bash
valid_ipv4() {
    local ip=$1 part
    local -a parts
    [[ $ip =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] || return 1
    IFS=. read -r -a parts <<< "$ip"
    for part in "${parts[@]}"; do
        [[ $part == 0 || $part != 0* ]] || return 1
        (( 10#$part <= 255 )) || return 1
    done
}
validate_config() {
    jq -e 'type=="object" and ((keys-["version","disk","identity","username","hostname","network","interface","address","gateway","dns","confirmation","desktop","graphics","utilities","mirror","timezone"])|length)==0 and .version==1 and ([.disk,.identity,.username,.hostname,.network,.interface,.address,.gateway,.dns,.confirmation]|all(type=="string")) and (.timezone==null or (.timezone|type=="string")) and (.mirror==null or (.mirror|type=="string")) and (.desktop==null or (.desktop|type=="string")) and (.graphics==null or (.graphics|type=="string")) and (.utilities==null or (.utilities|type=="array" and all(.[];type=="string") and (unique|length)==length))' <<< "$CONFIG" >/dev/null || fail 'Invalid configuration schema.'
    # Reject controls in original JSON before Bash strips newlines or NUL bytes.
    jq -e '[.. | strings | explode | all(. >= 32 and . != 127)] | all' <<< "$CONFIG" >/dev/null || fail 'Configuration strings must not contain control characters.'
    # Validate the JSON value before command substitution can strip newlines.
    jq -e --slurpfile zones "$SCRIPT_DIR/timezones.json" '(.timezone | if .==null or .=="" then "UTC" else . end) as $zone | $zones[0] | index($zone)!=null' <<< "$CONFIG" >/dev/null || fail 'Invalid timezone selection.'
    # Used by system configuration and final verification.
    # shellcheck disable=SC2034
    TIMEZONE=$(jq -r '.timezone | if .==null or .=="" then "UTC" else . end' <<< "$CONFIG")
    configure_mirror_root "$(jq -r '.mirror | if .==null or .=="" then "https://repo-default.voidlinux.org" else . end' <<< "$CONFIG")"
    DESKTOP=$(jq -r '.desktop | if .==null or .=="" then "none" else . end' <<< "$CONFIG")
    GRAPHICS=$(jq -r '.graphics | if .==null or .=="" then "default" else . end' <<< "$CONFIG")
    [[ $DESKTOP == none || $DESKTOP == kde || $DESKTOP == niri ]] || fail 'Invalid desktop selection.'
    [[ $GRAPHICS == default || $GRAPHICS == nvidia ]] || fail 'Invalid graphics selection.'
    local utility
    UTILITIES=()
    jq -e '(.utilities//[]) | length<=256 and all(.[]; test("^[a-zA-Z0-9][a-zA-Z0-9+_.-]{0,127}$") and (contains("\n")|not))' <<< "$CONFIG" >/dev/null || fail 'Invalid package selection.'
    while IFS= read -r utility; do
        [[ $utility =~ ^[a-zA-Z0-9][a-zA-Z0-9+_.-]{0,127}$ ]] || fail 'Invalid package name.'
        UTILITIES+=("$utility")
    done < <(jq -r '.utilities//[] | .[]' <<< "$CONFIG")
    DISK=$(jq -r .disk <<< "$CONFIG")
    IDENTITY=$(jq -r .identity <<< "$CONFIG")
    USERNAME=$(jq -r .username <<< "$CONFIG")
    HOSTNAME=$(jq -r .hostname <<< "$CONFIG")
    NETWORK=$(jq -r .network <<< "$CONFIG")
    INTERFACE=$(jq -r .interface <<< "$CONFIG")
    ADDRESS=$(jq -r .address <<< "$CONFIG")
    GATEWAY=$(jq -r .gateway <<< "$CONFIG")
    DNS=$(jq -r .dns <<< "$CONFIG")
    [[ $DISK =~ ^/dev/[a-zA-Z0-9_-]+$ && ${#IDENTITY} -le 1024 && -n $IDENTITY ]] || fail 'Invalid disk identity.'
    [[ $(jq -r .confirmation <<< "$CONFIG") == "ERASE $DISK" ]] || fail 'Explicit disk confirmation is required.'
    [[ $USERNAME =~ ^[a-z_][a-z0-9_-]{0,30}$ && $USERNAME != root ]] || fail 'Invalid username.'
    [[ $HOSTNAME =~ ^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$ ]] || fail 'Invalid hostname.'
    [[ $NETWORK == dhcp || $NETWORK == static ]] || fail 'Invalid network mode.'
    if [[ $NETWORK == static ]]; then
        [[ $INTERFACE =~ ^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,14}$ && $INTERFACE != lo && -d /sys/class/net/$INTERFACE ]] || fail 'Invalid network interface.'
        [[ $ADDRESS =~ ^([^/]+)/([0-9]{1,2})$ ]] || fail 'Invalid IPv4 prefix.'
        local ip=${BASH_REMATCH[1]} prefix=${BASH_REMATCH[2]}
        if ! valid_ipv4 "$ip" || (( 10#$prefix < 1 || 10#$prefix > 30 )); then fail 'Invalid IPv4 prefix.'; fi
        if ! valid_ipv4 "$GATEWAY" || ! valid_ipv4 "$DNS"; then fail 'Invalid gateway or DNS address.'; fi
        local dnsnum
        dnsnum=$(ipv4_number "$DNS")
        (( (dnsnum >> 24) > 0 && (dnsnum >> 24) < 224 && (dnsnum >> 24) != 127 && (dnsnum >> 16) != 43518 )) || fail 'DNS address must be unicast.'
        local ipnum gatewaynum mask network broadcast
        ipnum=$(ipv4_number "$ip"); gatewaynum=$(ipv4_number "$GATEWAY")
        mask=$(( (0xffffffff << (32 - 10#$prefix)) & 0xffffffff ))
        network=$(( ipnum & mask )); broadcast=$(( network | (0xffffffff ^ mask) ))
        (( (ipnum >> 24) > 0 && (ipnum >> 24) < 224 && (ipnum >> 24) != 127 && (ipnum >> 16) != 43518 && (gatewaynum >> 24) > 0 && (gatewaynum >> 24) < 224 && (gatewaynum >> 24) != 127 && (gatewaynum >> 16) != 43518 && gatewaynum != ipnum && (gatewaynum & mask) == network )) || fail 'Invalid static IPv4 route.'
        if (( 10#$prefix <= 30 )); then
            (( ipnum != network && ipnum != broadcast && gatewaynum != network && gatewaynum != broadcast )) || fail 'Network or broadcast addresses cannot be assigned.'
        fi
    else
        [[ -z $INTERFACE && -z $ADDRESS && -z $GATEWAY && -z $DNS ]] || fail 'DHCP configuration contains static fields.'
    fi
}
ipv4_number() {
    local a b c d
    IFS=. read -r a b c d <<< "$1"
    printf '%s\n' "$(( (10#$a << 24) | (10#$b << 16) | (10#$c << 8) | 10#$d ))"
}
read_public_config() {
    CONFIG=$(head -c 65537)
    (( ${#CONFIG} <= 65536 )) || fail 'Configuration exceeds size limit.'
    validate_config
    unset CONFIG
}
read_config() {
    read_public_config
    # Validate raw bytes before Bash can discard embedded NUL characters.
    # shellcheck disable=SC2034
    PASSWORD=$(head -c 130 <&3 | jq -eRrs '
        select(endswith("\n")) | .[0:-1] |
        select(length>=8 and length<=128) |
        select(explode | all(.>=32 and .<=126 and .!=58))
    ' 2>/dev/null) || fail 'Invalid password format or unavailable password channel.'
    exec 3<&-
}
