#!/usr/bin/env bash
configure_user_directories() {
    [[ -x $TARGET/usr/bin/xdg-user-dirs-update ]] || fail 'The user directory setup tool is missing.'
    chrun runuser -u "$USERNAME" -- env "HOME=/home/$USERNAME" LC_ALL=en_US.UTF-8 xdg-user-dirs-update
}
configure_audio() {
    local directory=$TARGET/etc/pipewire/pipewire.conf.d
    mkdir -p "$directory" "$TARGET/etc/alsa/conf.d" "$TARGET/etc/ld.so.conf.d"
    chmod 0755 "$TARGET/etc/pipewire" "$directory" "$TARGET/etc/alsa" "$TARGET/etc/alsa/conf.d" "$TARGET/etc/ld.so.conf.d"
    local source
    for source in /usr/share/examples/wireplumber/10-wireplumber.conf /usr/share/examples/pipewire/20-pipewire-pulse.conf; do
        [[ -r $TARGET$source ]] || fail 'Packaged PipeWire startup configuration is missing.'
        ln -sf "$source" "$directory/${source##*/}"
    done
    for source in /usr/share/alsa/alsa.conf.d/50-pipewire.conf /usr/share/alsa/alsa.conf.d/99-pipewire-default.conf; do
        [[ -r $TARGET$source ]] || fail 'Packaged ALSA integration is missing.'
        ln -sf "$source" "$TARGET/etc/alsa/conf.d/${source##*/}"
    done
    # Make JACK clients use PipeWire without a separate JACK server.
    printf '/usr/lib/pipewire-0.3/jack\n' > "$TARGET/etc/ld.so.conf.d/pipewire-jack.conf"
    chmod 0644 "$TARGET/etc/ld.so.conf.d/pipewire-jack.conf"
    chrun ldconfig
    if [[ $DESKTOP == kde ]]; then
        mkdir -p "$TARGET/home/$USERNAME/.config/autostart"
        chmod 0755 "$TARGET/home/$USERNAME/.config" "$TARGET/home/$USERNAME/.config/autostart"
        [[ -r $TARGET/usr/share/applications/pipewire.desktop ]] || fail 'The PipeWire desktop entry is missing.'
        ln -sf /usr/share/applications/pipewire.desktop "$TARGET/home/$USERNAME/.config/autostart/pipewire.desktop"
        chrun chown -R "$USERNAME:$USERNAME" "/home/$USERNAME/.config"
    fi
}
configure_wallet() {
    local pam=$TARGET/etc/pam.d/sddm module
    [[ -f $pam ]] || fail 'The SDDM PAM configuration is missing.'
    if [[ $DESKTOP == kde ]]; then
        # Void's module name is verified from the installed package, not assumed.
        if [[ -f $TARGET/usr/lib/security/pam_kwallet6.so ]]; then module=pam_kwallet6.so
        elif [[ -f $TARGET/usr/lib/security/pam_kwallet5.so ]]; then module=pam_kwallet5.so
        else fail 'The KWallet PAM module is missing.'; fi
        sed -i '/pam_kwallet[56]\.so/d' "$pam"
    else
        module=pam_gnome_keyring.so
        [[ -f $TARGET/usr/lib/security/$module ]] || fail 'The GNOME Keyring PAM module is missing.'
        sed -i '/pam_gnome_keyring\.so/d' "$pam"
    fi
    # Optional modules must never prevent normal password authentication.
    printf '\nauth optional %s\nsession optional %s auto_start\n' "$module" "$module" >> "$pam"
    if [[ $DESKTOP == niri ]]; then printf 'password optional %s use_authtok\n' "$module" >> "$pam"; fi
    chmod 0644 "$pam"
}
configure_portals() {
    local directory=$TARGET/home/$USERNAME/.config/xdg-desktop-portal
    mkdir -p "$directory"
    # Use GTK file selection without adding Nautilus to the minimal profile.
    printf '[preferred]\ndefault=gnome;gtk;\norg.freedesktop.impl.portal.FileChooser=gtk;\norg.freedesktop.impl.portal.Access=gtk;\norg.freedesktop.impl.portal.Notification=gtk;\norg.freedesktop.impl.portal.Secret=gnome-keyring;\n' > "$directory/niri-portals.conf"
    chmod 0755 "$TARGET/home/$USERNAME/.config" "$directory"
    chmod 0644 "$directory/niri-portals.conf"
    chrun chown -R "$USERNAME:$USERNAME" "/home/$USERNAME/.config"
}
verify_session_configuration() {
    [[ $DESKTOP != none ]] || return 0
    [[ -s $TARGET/home/$USERNAME/.config/user-dirs.dirs && -d $TARGET/home/$USERNAME/Downloads && -d $TARGET/home/$USERNAME/Documents && -d $TARGET/home/$USERNAME/Pictures ]] || fail 'Default user directories are missing.'
    [[ -x $TARGET/usr/bin/pipewire && -x $TARGET/usr/bin/wireplumber && -x $TARGET/usr/bin/pipewire-pulse && -x $TARGET/usr/bin/pw-jack ]] || fail 'Audio components are unavailable.'
    [[ -L $TARGET/etc/pipewire/pipewire.conf.d/10-wireplumber.conf && -L $TARGET/etc/pipewire/pipewire.conf.d/20-pipewire-pulse.conf ]] || fail 'Audio startup configuration is missing.'
    if [[ $DESKTOP == kde ]]; then
        [[ -x $TARGET/usr/bin/konsole ]] || fail 'The KDE terminal is missing.'
        local module
        for module in kcm_kscreen kcm_sddm kcm_pulseaudio; do
            [[ -f $TARGET/usr/lib/qt6/plugins/plasma/kcms/systemsettings/$module.so ]] || fail "A required KDE settings module is missing: $module"
        done
    else
        [[ -x $TARGET/usr/bin/xwayland-satellite && -x $TARGET/usr/bin/Xwayland ]] || fail 'X11 compatibility components are unavailable.'
        [[ -x $TARGET/usr/libexec/polkit-gnome-authentication-agent-1 ]] || fail 'The graphical authentication agent is missing.'
        [[ -x $TARGET/usr/bin/nm-connection-editor && -x $TARGET/usr/bin/nmtui && -x $TARGET/usr/bin/pavucontrol ]] || fail 'The niri configuration tools are missing.'
        grep -qx 'spawn-at-startup "/usr/libexec/polkit-gnome-authentication-agent-1"' "$TARGET/home/$USERNAME/.config/niri/config.kdl" || fail 'The graphical authentication agent is not configured to start.'
        [[ -x $TARGET/usr/bin/gnome-keyring-daemon && -x $TARGET/usr/bin/seahorse ]] || fail 'The niri keyring components are missing.'
        [[ -r $TARGET/usr/share/xdg-desktop-portal/portals/gnome.portal && -r $TARGET/usr/share/xdg-desktop-portal/portals/gtk.portal ]] || fail 'The niri portal backends are missing.'
        grep -qx 'org.freedesktop.impl.portal.FileChooser=gtk;' "$TARGET/home/$USERNAME/.config/xdg-desktop-portal/niri-portals.conf" || fail 'The niri file chooser backend is not configured.'
    fi
}
