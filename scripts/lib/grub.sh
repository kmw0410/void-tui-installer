#!/usr/bin/env bash
configure_grub() {
    # Explicit subvol=@ keeps GRUB and the initramfs on the installed root.
    cat >> "$TARGET/etc/default/grub" <<'GRUB_CONFIG'
GRUB_CMDLINE_LINUX="rootflags=subvol=@"
GRUB_DISABLE_OS_PROBER=true
GRUB_CONFIG
    chmod 0644 "$TARGET/etc/default/grub"
    if [[ -d $TARGET/sys/firmware/efi/efivars ]] && ! mountpoint -q "$TARGET/sys/firmware/efi/efivars"; then
        if ! mount -t efivarfs efivarfs "$TARGET/sys/firmware/efi/efivars" >&2; then
            printf '%s\n' 'EFI variables are unavailable; the fallback bootloader will be installed.' >&2
        fi
    fi
    chrun xbps-reconfigure -fa
    if ! chroot "$TARGET" grub-install --target=x86_64-efi --efi-directory=/boot/efi --bootloader-id=Void >&2; then
        printf '%s\n' 'NVRAM installation failed; installing verified EFI fallback files.' >&2
        chrun grub-install --target=x86_64-efi --efi-directory=/boot/efi --bootloader-id=Void --no-nvram
    fi
    # Always provide the standard fallback loader for restrictive firmware.
    chrun grub-install --target=x86_64-efi --efi-directory=/boot/efi --removable --no-nvram
    chrun grub-mkconfig -o /boot/grub/grub.cfg
    [[ -s $TARGET/boot/efi/EFI/Void/grubx64.efi && -s $TARGET/boot/efi/EFI/BOOT/BOOTX64.EFI ]] || fail 'EFI boot files were not generated.'
    grep -q 'rootflags=subvol=@' "$TARGET/boot/grub/grub.cfg" || fail 'GRUB root subvolume configuration is missing.'
}
