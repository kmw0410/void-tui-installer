#!/usr/bin/env bash
software_catalog() {
    jq "$@" "$SCRIPT_DIR/catalog.json"
}
select_packages() {
    # jq expands $fs from --arg; the shell must leave the expression literal.
    # shellcheck disable=SC2016
    software_catalog -e --arg fs "$FILESYSTEM" '(.base|type=="array" and length>0) and (.filesystems[$fs]|type=="array" and length>0)' >/dev/null || fail 'The software catalog is unavailable.'
    local package
    # shellcheck disable=SC2016
    mapfile -t PACKAGES < <(software_catalog -r --arg fs "$FILESYSTEM" '.base[], .filesystems[$fs][]')
    local -a additions=()
    mapfile -t additions < <(software_catalog -r '.common[]')
    if [[ $DESKTOP != none ]]; then
        # shellcheck disable=SC2016
        mapfile -t additions < <(software_catalog -r --arg desktop "$DESKTOP" '.common[], .graphical[], .desktops[$desktop][]')
    fi
    if [[ $GRAPHICS == nvidia ]]; then additions+=(nvidia linux-headers void-repo-nonfree); fi
    additions+=("${UTILITIES[@]}")
    for package in "${UTILITIES[@]}"; do
        if [[ $package == fcitx5* ]]; then
            local -a modules
            mapfile -t modules < <(software_catalog -r '.fcitx[]')
            additions+=("${modules[@]}")
            break
        fi
    done
    for package in "${additions[@]}"; do
        if [[ ! " ${PACKAGES[*]} " == *" $package "* ]]; then PACKAGES+=("$package"); fi
    done
    REPOSITORIES=(-R "$REPOSITORY")
    QUERY_REPOSITORIES=(--repository="$REPOSITORY")
    if [[ $GRAPHICS == nvidia ]]; then
        REPOSITORIES+=(-R "$REPOSITORY/nonfree")
        QUERY_REPOSITORIES+=(--repository="$REPOSITORY/nonfree")
    fi
}
check_graphics() {
    [[ $GRAPHICS == nvidia ]] || return 0
    [[ -n ${VOID_INSTALLER_HELPER:-} && -x $VOID_INSTALLER_HELPER ]] || fail 'NVIDIA verification requires the installer executable.'
    local pkgver version
    pkgver=$(timeout 30 env XBPS_ARCH=x86_64 xbps-query -r "$WORK/repo" -R "${QUERY_REPOSITORIES[@]}" -p pkgver nvidia)
    version=${pkgver#nvidia-}; version=${version%_*}
    run "$VOID_INSTALLER_HELPER" --check-nvidia "$version"
    # Pin the checked driver even if the repository changes before bootstrap.
    local index
    for index in "${!PACKAGES[@]}"; do
        if [[ ${PACKAGES[index]} == nvidia ]]; then PACKAGES[index]=$pkgver; fi
    done
}
configure_software() {
    if [[ $GRAPHICS == nvidia ]]; then
        mkdir -p "$TARGET/etc/modprobe.d"
        printf 'options nvidia_drm modeset=1\n' > "$TARGET/etc/modprobe.d/void-installer-nvidia.conf"
        chmod 0644 "$TARGET/etc/modprobe.d/void-installer-nvidia.conf"
    fi
    persist_mirror
    enable_service bluetoothd
    enable_service tlp
    enable_service dbus
    enable_service cronie
    if [[ $DESKTOP != none ]]; then
        enable_service dbus
        enable_service sddm
        mkdir -p "$TARGET/etc/sddm.conf.d"
        chmod 0755 "$TARGET/etc/sddm.conf.d"
        printf '[General]\nDisplayServer=x11\n' > "$TARGET/etc/sddm.conf.d/10-void-installer.conf"
        chmod 0644 "$TARGET/etc/sddm.conf.d/10-void-installer.conf"
        configure_audio
        configure_wallet
        configure_user_directories
        chrun usermod -a -G network "$USERNAME"
        if [[ $DESKTOP == niri ]]; then
            # The upstream niri-session launcher requires systemd or dinit.
            # Use a dedicated session bus and elogind on Void/runit instead.
            mkdir -p "$TARGET/usr/local/bin" "$TARGET/usr/share/wayland-sessions"
            chmod 0755 "$TARGET/usr/local" "$TARGET/usr/local/bin" "$TARGET/usr/share/wayland-sessions"
            printf '#!/bin/sh\nexport XDG_CURRENT_DESKTOP=niri\nexport XDG_SESSION_TYPE=wayland\nexec dbus-run-session niri --session\n' > "$TARGET/usr/local/bin/void-niri-session"
            chmod 0755 "$TARGET/usr/local/bin/void-niri-session"
            printf '[Desktop Entry]\nName=Niri\nExec=/usr/local/bin/void-niri-session\nType=Application\nDesktopNames=niri\n' > "$TARGET/usr/share/wayland-sessions/niri.desktop"
            chmod 0644 "$TARGET/usr/share/wayland-sessions/niri.desktop"
            configure_niri
            configure_portals
        fi
    fi
    configure_input_method
}
has_utility() { [[ " ${UTILITIES[*]} " == *" $1 "* ]]; }
configure_niri() {
    local directory=$TARGET/home/$USERNAME/.config/niri
    mkdir -p "$directory"
    # Do not inherit shortcuts for applications the user did not select.
    printf '%s\n' 'binds {' '    Mod+Shift+E { quit; }' '    Mod+Q { close-window; }' '    Mod+Left { focus-column-left; }' '    Mod+Right { focus-column-right; }' '    Mod+Up { focus-window-up; }' '    Mod+Down { focus-window-down; }' '    Mod+Shift+Left { move-column-left; }' '    Mod+Shift+Right { move-column-right; }' '    Mod+Shift+Up { move-window-up; }' '    Mod+Shift+Down { move-window-down; }' '    Mod+F { maximize-column; }' '    Mod+Shift+F { fullscreen-window; }' > "$directory/config.kdl"
    if has_utility kitty; then printf '    Mod+Return { spawn "kitty"; }\n' >> "$directory/config.kdl"; fi
    if has_utility firefox; then printf '    Mod+B { spawn "firefox"; }\n' >> "$directory/config.kdl"; fi
    # niri starts xwayland-satellite on demand; do not spawn a second instance.
    printf '%s\n' '}' 'xwayland-satellite { path "/usr/bin/xwayland-satellite"; }' 'spawn-at-startup "/usr/libexec/polkit-gnome-authentication-agent-1"' 'spawn-at-startup "xdg-user-dirs-update"' 'spawn-at-startup "pipewire"' 'spawn-at-startup "gnome-keyring-daemon" "--start" "--components=secrets,pkcs11"' 'spawn-at-startup "sh" "-c" "dbus-update-activation-environment WAYLAND_DISPLAY XDG_CURRENT_DESKTOP XDG_SESSION_TYPE"' >> "$directory/config.kdl"
    if has_utility fcitx5 || [[ " ${UTILITIES[*]} " == *' fcitx5-'* ]]; then
        printf 'spawn-at-startup "fcitx5"\nenvironment {\n    XMODIFIERS "@im=fcitx"\n    QT_IM_MODULE "fcitx"\n}\n' >> "$directory/config.kdl"
    fi
    chmod 0755 "$TARGET/home/$USERNAME/.config" "$directory"
    chmod 0644 "$directory/config.kdl"
    chrun chown -R "$USERNAME:$USERNAME" "/home/$USERNAME/.config"
}
configure_input_method() {
    local package found=0
    for package in "${UTILITIES[@]}"; do
        if [[ $package == fcitx5* ]]; then found=1; break; fi
    done
    (( found )) || return 0
    local directory=$TARGET/home/$USERNAME/.config/fcitx5
    mkdir -p "$directory"
    if has_utility fcitx5-hangul; then
        printf '[Groups/0]\nName=Default\nDefault Layout=us\nDefaultIM=hangul\n\n[Groups/0/Items/0]\nName=keyboard-us\nLayout=\n\n[Groups/0/Items/1]\nName=hangul\nLayout=\n\n[GroupOrder]\n0=Default\n' > "$directory/profile"
        chmod 0600 "$directory/profile"
    fi
    # KWin launches the input method with its Wayland socket, not XDG autostart.
    if [[ $DESKTOP == kde ]]; then
        printf '[Wayland]\nInputMethod=/usr/share/applications/org.fcitx.Fcitx5.desktop\nVirtualKeyboardEnabled=true\n' > "$TARGET/home/$USERNAME/.config/kwinrc"
        mkdir -p "$TARGET/home/$USERNAME/.config/plasma-workspace/env" "$TARGET/home/$USERNAME/.config/autostart"
        # Let KWin own startup so an XDG autostart cannot steal the D-Bus name.
        printf '[Desktop Entry]\nType=Application\nName=Fcitx 5\nHidden=true\n' > "$TARGET/home/$USERNAME/.config/autostart/org.fcitx.Fcitx5.desktop"
        printf '#!/bin/sh\nexport XMODIFIERS=@im=fcitx\n' > "$TARGET/home/$USERNAME/.config/plasma-workspace/env/fcitx5.sh"
        chmod 0644 "$TARGET/home/$USERNAME/.config/kwinrc" "$TARGET/home/$USERNAME/.config/plasma-workspace/env/fcitx5.sh" "$TARGET/home/$USERNAME/.config/autostart/org.fcitx.Fcitx5.desktop"
        chmod 0755 "$TARGET/home/$USERNAME/.config/plasma-workspace" "$TARGET/home/$USERNAME/.config/plasma-workspace/env" "$TARGET/home/$USERNAME/.config/autostart"
    fi
    chmod 0755 "$TARGET/home/$USERNAME/.config" "$directory"
    chrun chown -R "$USERNAME:$USERNAME" "/home/$USERNAME/.config"
}
verify_software() {
    verify_session_configuration
    local package
    for package in "${UTILITIES[@]}"; do
        if [[ $package == fcitx5* ]]; then
            [[ -x $TARGET/usr/bin/fcitx5 ]] || fail 'The selected input method is missing.'
            if [[ $DESKTOP == kde ]]; then
                [[ -r $TARGET/usr/share/applications/org.fcitx.Fcitx5.desktop ]] || fail 'The Fcitx Wayland input-method entry is missing.'
            fi
            break
        fi
    done
    if [[ $DESKTOP != none ]]; then
        [[ -L $TARGET/etc/runit/runsvdir/default/sddm && -x $TARGET/usr/bin/elogind-inhibit ]] || fail 'Graphical login or seat management is unavailable.'
        [[ -f $TARGET/usr/lib/xorg/modules/input/libinput_drv.so ]] || fail 'The graphical login input driver is missing.'
        if [[ $DESKTOP == niri ]]; then
            chrun su -s /bin/sh -c 'niri validate' "$USERNAME"
            [[ -x $TARGET/usr/local/bin/void-niri-session ]] || fail 'The niri session launcher is missing.'
        else
            [[ -x $TARGET/usr/bin/startplasma-wayland ]] || fail 'The Plasma Wayland session is missing.'
        fi
    fi
    if [[ $GRAPHICS == nvidia ]]; then
        local kernel version
        for kernel in "$TARGET"/boot/vmlinuz-*; do
            version=${kernel##*/vmlinuz-}
            chrun modinfo -k "$version" nvidia
            chrun modinfo -k "$version" nvidia_drm
        done
    fi
}
