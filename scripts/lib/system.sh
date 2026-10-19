#!/usr/bin/env bash
configure_system() {
    printf '%s\n' "$HOSTNAME" > "$TARGET/etc/hostname"
    cat > "$TARGET/etc/hosts" <<HOSTS
127.0.0.1 localhost
::1 localhost
127.0.1.1 $HOSTNAME
HOSTS
    write_fstab
    sed -i 's/^#\(en_US.UTF-8 UTF-8\)/\1/' "$TARGET/etc/default/libc-locales"
    printf 'LANG=en_US.UTF-8\n' > "$TARGET/etc/locale.conf"
    chmod 0644 "$TARGET/etc/hostname" "$TARGET/etc/hosts" "$TARGET/etc/locale.conf"
    configure_timezone
    chrun xbps-reconfigure -f glibc-locales
    chrun useradd -m -G wheel -s /bin/bash "$USERNAME"
    # chpasswd receives the secret only on stdin; suppress all diagnostics.
    if ! printf '%s:%s\n' "$USERNAME" "$PASSWORD" | chroot "$TARGET" chpasswd -c SHA512 >/dev/null 2>&1; then
        unset PASSWORD
        fail 'Account password configuration failed.'
    fi
    unset PASSWORD
    chrun usermod -L root
    verify_account_passwords
    mkdir -p "$TARGET/etc/sudoers.d"
    printf '%%wheel ALL=(ALL:ALL) ALL\n' > "$TARGET/etc/sudoers.d/10-wheel"
    chmod 0750 "$TARGET/etc/sudoers.d"
    chmod 0440 "$TARGET/etc/sudoers.d/10-wheel"
    chrun visudo -cf /etc/sudoers
}
configure_timezone() {
    jq -e --arg zone "$TIMEZONE" 'index($zone)!=null' "$SCRIPT_DIR/timezones.json" >/dev/null || fail 'Invalid timezone selection.'
    [[ -r $TARGET/usr/share/zoneinfo/$TIMEZONE && $(head -c 4 "$TARGET/usr/share/zoneinfo/$TIMEZONE") == TZif ]] || fail 'Selected timezone data is unavailable.'
    ln -sfn "/usr/share/zoneinfo/$TIMEZONE" "$TARGET/etc/localtime"
}
verify_timezone() {
    [[ -L $TARGET/etc/localtime && $(readlink "$TARGET/etc/localtime") == "/usr/share/zoneinfo/$TIMEZONE" ]] || fail 'Installed timezone does not match the selection.'
    [[ $(head -c 4 "$TARGET/usr/share/zoneinfo/$TIMEZONE") == TZif ]] || fail 'Installed timezone data is invalid.'
}
verify_account_passwords() {
    # Inspect status without emitting password hashes or account database records.
    # shellcheck disable=SC2016
    chroot "$TARGET" awk -F: -v "user=$USERNAME" '
        $1=="root" {
            if ($2 !~ /^[!*]/) exit 1
            roots++
        }
        $1==user {
            if ($2 !~ /^\$6\$/ || length($2)<80) exit 1
            users++
        }
        END { if (roots!=1 || users!=1) exit 1 }
    ' /etc/shadow >/dev/null 2>&1 || fail 'Account password hash verification failed.'
}
enable_service() {
    local name=$1
    [[ -d $TARGET/etc/sv/$name && -d $TARGET/etc/runit/runsvdir/default ]] || fail 'Required runit service is unavailable.'
    if [[ ! -L $TARGET/etc/runit/runsvdir/default/$name ]]; then
        ln -s "/etc/sv/$name" "$TARGET/etc/runit/runsvdir/default/$name"
    fi
}
