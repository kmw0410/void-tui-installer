#!/usr/bin/env bash
bootstrap() {
    mkdir -p "$TARGET/var/db/xbps/keys"
    chmod 0755 "$TARGET/var" "$TARGET/var/db" "$TARGET/var/db/xbps" "$TARGET/var/db/xbps/keys"
    cp -a /var/db/xbps/keys/. "$TARGET/var/db/xbps/keys/"
    run env XBPS_ARCH=x86_64 xbps-install -S -y -r "$TARGET" "${REPOSITORIES[@]}" "${PACKAGES[@]}"
    mkdir -p "$TARGET"/{dev,proc,sys,run}
    chmod 0755 "$TARGET"/{dev,proc,sys,run}
    local path
    for path in dev proc sys; do
        run mount --rbind "/$path" "$TARGET/$path"
        run mount --make-rslave "$TARGET/$path"
    done
    # A private /run prevents enabling target services in the live supervisor.
    run mount -t tmpfs -o mode=0755,nosuid,nodev tmpfs "$TARGET/run"
    if [[ -L $TARGET/etc/resolv.conf ]]; then
        rm -- "$TARGET/etc/resolv.conf"
    fi
    cp -L /etc/resolv.conf "$TARGET/etc/resolv.conf"
    chmod 0644 "$TARGET/etc/resolv.conf"
}
