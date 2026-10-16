#!/usr/bin/env bash
# Boot an unmodified, verified Void Live ISO with one isolated virtual disk.
set -euo pipefail
ROOT=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
VM=$ROOT/tests/vm
TOOLS=$ROOT/.tools/vm/root
ISO=$ROOT/.tools/vm/void-live-x86_64-20250202-base.iso
QEMU=${QEMU:-$TOOLS/usr/bin/qemu-system-x86_64}
CODE=${OVMF_CODE:-$TOOLS/usr/share/edk2/x64/OVMF_CODE.4m.fd}
VARS=${OVMF_VARS:-$TOOLS/usr/share/edk2/x64/OVMF_VARS.4m.fd}
MODE=live
BUS=virtio
GRAPHICS=basic
AUDIO=none
MEMORY=${VM_MEMORY_MIB:-3072}
if [[ ! $MEMORY =~ ^[0-9]+$ ]] || (( MEMORY<1024 || MEMORY>32768 )); then
    echo 'VM_MEMORY_MIB must be between 1024 and 32768.' >&2
    exit 1
fi
while (( $# )); do
    case "$1" in
        --installed) MODE=installed; shift;;
        --disk-bus) [[ $# -ge 2 ]] || exit 1; BUS=$2; shift 2;;
        --graphics) [[ $# -ge 2 ]] || exit 1; GRAPHICS=$2; shift 2;;
        --audio) [[ $# -ge 2 ]] || exit 1; AUDIO=$2; shift 2;;
        *) echo 'Usage: qemu-live.sh [--installed] [--disk-bus virtio|sata|nvme] [--graphics basic|virgl] [--audio none|hda]' >&2; exit 1;;
    esac
done
case "$BUS" in
    virtio) DEVICE=(-device 'virtio-blk-pci,drive=target,serial=VOIDTEST001,bootindex=2');;
    sata) DEVICE=(-device 'ide-hd,drive=target,bus=ide.0,serial=VOIDTEST001,bootindex=2');;
    nvme) DEVICE=(-device 'nvme,drive=target,serial=VOIDTEST001,bootindex=2');;
    *) echo 'Unsupported virtual disk bus.' >&2; exit 1;;
esac
AUDIO_ARGS=()
case "$AUDIO" in
    none) ;;
    hda) AUDIO_ARGS=(-audiodev 'driver=none,id=testaudio' -device intel-hda -device 'hda-duplex,audiodev=testaudio');;
    *) echo 'Unsupported virtual audio mode.' >&2; exit 1;;
esac
DISPLAY_ARGS=(-display none)
case "$GRAPHICS" in
    basic) ;;
    virgl)
        [[ ${VM_RENDER_NODE:-} =~ ^/dev/dri/renderD[0-9]+$ && -c $VM_RENDER_NODE ]] || { echo 'VM_RENDER_NODE must name a DRM render node.' >&2; exit 1; }
        DISPLAY_ARGS=(-vga none -device virtio-vga-gl -display "egl-headless,rendernode=$VM_RENDER_NODE")
        ;;
    *) echo 'Unsupported virtual graphics mode.' >&2; exit 1;;
esac
mkdir -p "$VM"
[[ -f $ISO && -f $CODE && -f $VARS ]] || { echo 'Verified ISO and OVMF firmware are required.' >&2; exit 1; }
[[ ! -L $VM/disk.raw && ! -L $VM/vars.fd ]] || { echo 'VM image symlinks are forbidden.' >&2; exit 1; }
if [[ ! -e $VM/disk.raw ]]; then
    [[ $MODE == live ]] || { echo 'Installed image does not exist.' >&2; exit 1; }
    truncate -s 24G "$VM/disk.raw"
fi
[[ -f $VM/disk.raw ]] || { echo 'VM disk must be a regular image file.' >&2; exit 1; }
if [[ ! -e $VM/vars.fd ]]; then cp -- "$VARS" "$VM/vars.fd"; fi
export LD_LIBRARY_PATH=$TOOLS/usr/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}
BOOT=(-boot order=c)
if [[ $MODE == live ]]; then
    BOOT=(-drive "if=none,id=live,media=cdrom,readonly=on,file=$ISO" -device 'ide-cd,drive=live,bus=ide.1,bootindex=1' -boot order=d)
fi
exec "$QEMU" -name void-installer-test -machine q35,accel=kvm -cpu host -smp 2 -m "$MEMORY" \
  -L "$TOOLS/usr/share/qemu" \
  -drive "if=pflash,format=raw,readonly=on,file=$CODE" \
  -drive "if=pflash,format=raw,file=$VM/vars.fd" \
  -drive "if=none,id=target,format=raw,file=$VM/disk.raw" \
  "${DEVICE[@]}" \
  "${BOOT[@]}" \
  -netdev user,id=net0,hostfwd=tcp:127.0.0.1:22222-:22 -device virtio-net-pci,netdev=net0 \
  "${DISPLAY_ARGS[@]}" "${AUDIO_ARGS[@]}" -serial "file:$VM/serial.log" \
  -qmp "unix:$VM/qmp.sock,server=on,wait=off" -no-reboot
