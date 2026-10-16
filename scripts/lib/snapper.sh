#!/usr/bin/env bash
configure_snapper() {
    # Snapper 0.13 calls systemctl for templates with TIMELINE_CREATE=yes.
    # Enable the cron policy only after creating the config without that hook.
    local template=$TARGET/etc/snapper/config-templates/installer-root
    mkdir -p "$TARGET/etc/snapper/config-templates"
    chmod 0755 "$TARGET/etc/snapper/config-templates"
    cp "$TARGET/usr/share/snapper/config-templates/default" "$template"
    sed -i 's/^TIMELINE_CREATE=.*/TIMELINE_CREATE="no"/' "$template"
    grep -qx 'TIMELINE_CREATE="no"' "$template" || fail 'Snapper template format is unsupported.'
    chmod 0644 "$template"
    chrun snapper --no-dbus -c root create-config --template installer-root /
    run btrfs subvolume delete "$TARGET/.snapshots"
    mount_subvolume @snapshots /.snapshots
    chmod 0700 "$TARGET/.snapshots"
    local key value
    while read -r key value; do
        sed -i "s/^$key=.*/$key=\"$value\"/" "$TARGET/etc/snapper/configs/root"
        grep -qx "$key=\"$value\"" "$TARGET/etc/snapper/configs/root" || fail 'Snapper configuration key is unavailable.'
    done <<'POLICY'
TIMELINE_CREATE yes
TIMELINE_CLEANUP yes
TIMELINE_LIMIT_HOURLY 5
TIMELINE_LIMIT_DAILY 7
TIMELINE_LIMIT_WEEKLY 4
TIMELINE_LIMIT_MONTHLY 3
TIMELINE_LIMIT_YEARLY 0
POLICY
    # Void ships an hourly cron helper; it requires a working system D-Bus.
    [[ -x $TARGET/etc/cron.hourly/snapper ]] || fail 'Packaged Snapper cron helper is missing.'
    grep -q 'SNAPPER_CONFIGS=.*root' "$TARGET/etc/conf.d/snapper" || fail 'Snapper cron configuration is missing.'
    enable_service dbus
    enable_service cronie
    chrun snapper --no-dbus -c root create --description 'Initial installation'
    chrun snapper --no-dbus -c root list
    [[ -s $TARGET/.snapshots/1/info.xml ]] || fail 'Initial snapshot metadata is missing.'
}
