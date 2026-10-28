# Validation record

> [!CAUTION]
> The installation and boot results documented here were obtained in QEMU virtual machines. They do not establish compatibility with physical hardware. Host-side build, unit, fixture and static checks are reported separately.

## 2026-10-19: ext4 support and user testing

Added Disk filesystem selection, compatible public/saved configuration, filesystem-specific packages, guarded ext4 formatting, UUID fstab/GRUB boot, root mount verification and a skipped Snapper stage. Existing fixtures were updated for the filesystems module and e2fsprogs preparation; new Go/Bash test cases were added for defaults, validation, package/settings selection, protocol skip policy, disk-screen controls and mocked ext4 formatting/fstab. At the user's request, the agent ran only make build: no Go tests, Bash fixtures, race checks, ShellCheck, VM boot or actual installation. These automated tests remain unexecuted for this change.

The user subsequently reported successful testing of the build and requested publication of the changes. The tested binary SHA-256 was b116c3d8f47e2e006c9fa50c9d451d6dc4aab12d86a589fe68f06f4063998cff. Exact test steps, filesystem coverage and reboot results were not provided, so no additional coverage is inferred.

## Latest: timezones and niri configuration tools (0.4.0-dev)

Go tests, the race detector, vet, 32 Bash fixture groups, Bash syntax and ShellCheck passed. New tests cover all 419 bundled timezone names against Go's timezone database, rejection of unknown/traversal/newline values in both boundaries, searchable timezone selection/back navigation, 80x24 layout, niri DHCP/static rendering and removal of competing supplicant service links. Bash validates the original timezone JSON value before command substitution, which would otherwise remove trailing newlines. Package-profile tests cover required fonts, user directories, NetworkManager and niri integration without implicitly selecting Firefox, Kitty or an application launcher.

The dedicated existing Void QEMU image, not a host block device, received the real additional packages. Signed Fastly repository resolution and the complete fresh-root niri/base/input/utilities dry-run transaction passed, including tzdata, xwayland-satellite 0.8.3, network-manager-applet 1.36.0, polkit-gnome 0.105, pavucontrol 6.2 and wl-clipboard 2.3.0. The actual configuration functions validated niri's generated KDL, created user-owned standard directories, rendered NetworkManager and verified the timezone link. After an ISO-free clean boot and SDDM password login, the clock remained KST +0900, NetworkManager was running, DHCP DNS was 10.0.2.3 and repository-domain resolution succeeded. The user belonged to wheel and network.

The niri session started one graphical authentication agent. Network Connections and Volume Control both opened as real windows. A Konsole launched explicitly with QT_QPA_PLATFORM=xcb opened through niri's managed xwayland-satellite bridge; the bridge and X11 window were confirmed through processes and compositor IPC. A terminal-launched pkexec request displayed the graphical Authenticate dialog. Password authentication succeeded and created a test directory owned by root with mode 0700 under /run; no password was passed to a diagnostic command. The test screenshots and helper are under tests/vm. The entire disk installation was not repeated for this revision; Wi-Fi hardware association, physical GPUs, and the new KDE user-directory setup after a fresh installation were not separately tested. Static niri networking is covered by fixtures; this follow-up VM boot used DHCP.

Distribution: 0.4.0-dev, static Linux amd64 dist/void-installer-tui, SHA256 bada09f72caf368ed4d1244c2dcf60f18003e6fcbe0a746e2fdaf7f5f817db58. No NTP service, application launcher, bar or extra terminal was added to the installation profiles.

Date: 2026-10-15. The installer runtime is the official Void Live ISO. Host-side compilation and static tests do not validate installation, firmware boot, or live-medium compatibility.

## Isolated guest setup

The development host did not have QEMU, OVMF, or ShellCheck installed. QEMU 11.1.2 and OVMF 202608 were extracted without root into ignored `.tools/vm/root`; ShellCheck 0.11.0 was downloaded from its official release. `/dev/kvm` was available. No host disk or host block device was attached to QEMU.

`tests/qemu-live.sh` boots the unchanged official `void-live-x86_64-20250202-base.iso` under OVMF UEFI with 3 GiB RAM and a 24 GiB regular sparse image at `tests/vm/disk.raw`. The guest target is virtio `/dev/vda` with serial `VOIDTEST001`. Networking is user-mode NAT. The local SSH port binds only to `127.0.0.1:22222`. `tests/qemu-control.py` sends input or captures the guest framebuffer through the local QMP socket. Runtime disks, firmware variables, SSH keys, and logs are local ignored artifacts.

The ISO checksum list's minisign signature verified with the official release public key. The ISO SHA256 matched `0f7439f500740f62dd18972cae448cec7d8a85032c7eb8f1bf946100d9a92161`. Firmware loaded the ISO's GRUB and the official Linux 6.12.11_1 kernel reached the Void Live login prompt. `/proc/cmdline` contained `root=live:CDLABEL=VOID_LIVE` and `/sys/firmware/efi` was available.

Within the guest, the live XBPS package manager was updated with `xbps-install -Sy xbps`; additional required tools were installed using `xbps-install -Sy jq gptfdisk parted dosfstools btrfs-progs`. The base ISO lacks jq, sgdisk, and partprobe. This is a live-session prerequisite, not a build-host prerequisite.

The second Live session also installed `psmisc` for the raw-device open-handle guard. Actual Void `fuser` testing confirmed exit 1 with empty output for an idle virtual disk, and exit 0 with a process ID when a read-only descriptor was open. Void's `fuser` rejects a standalone `--` operand; the command must use the independently validated absolute device path directly.

## Findings during actual Live ISO validation

1. Initial engine `--check` rejected the official ISO because `/etc/os-release` has `ID="void"`, while the original check expected `ID=void`. This was reported for correction before any target disk operation.
2. The next repository preflight failed because XBPS dry-run (`-n`) does not actually synchronize an empty private root's repository metadata. A separate `xbps-install -S` followed by the dry-run resolved every required target package successfully. This was reported for correction before any target disk operation.
3. The first full engine attempt stopped safely during preflight because `lsblk -dn -o MAJ:MIN` pads its output with spaces. The device-number comparison needed raw output or trimming. Cleanup completed and the engine emitted a failed result; no partitioning stage was reached.
4. Interactive TUI testing during a concurrent guest installation exposed a discovery defect: `lsblk --json` without `NAME` or `--tree` returns partitions as separate top-level rows. The frontend skipped these rows and marked their mounted parent eligible. Explicit tree output fixed this, and the parser now independently rejects flattened partition rows. The corrected binary was tested against a read-only mounted NVMe target and blocked at Welcome with `No safe installation disk is available.` The independent Bash checks had continued to protect the target throughout.
5. The first DHCP engine run completed all stages with exit zero, GRUB binaries, kernel/initramfs, and an initial Snapper snapshot. Post-install inspection nevertheless found that the user remained password-locked. Void's `/etc/pam.d/chpasswd` uses `pam_permit.so` for its password operation; plain `chpasswd` did not create password hashes despite reporting success. Explicit SHA512 password setting plus independent hash verification fixed this. Real login and sudo authentication passed on the subsequent complete NVMe and SATA installations. The initial defective run is not counted as a successful acceptance test.

## Required installation acceptance checks

Record an explicit result for each item after a real installation. Until then, an item is unverified; source inspection or mocked tests are not equivalent.

| Scenario | Status |
| --- | --- |
| Official ISO UEFI boot and root login | Passed |
| Live tools and XBPS repository connectivity | Passed |
| Fresh virtio target, DHCP installation | Engine completed; initial credential defect corrected and retested in the NVMe installation below |
| Installed disk UEFI boot without ISO | Passed: OVMF loaded installed GRUB and Linux 6.18.56_1 reached the login prompt |
| User login and sudo | Passed after the corrected full NVMe TUI install: console login as voidtest, sudo password authentication returned uid=0, separate root console login with the same password |
| Btrfs subvolumes and UUID fstab mounts | Passed on installed boot: @, @home, @snapshots, @log, @cache and FAT ESP mounted as specified |
| DHCP connectivity after installed boot | Passed: eth0 received 10.0.2.15/24, default gateway 10.0.2.2; official repository query succeeded |
| Snapper initial snapshot and configuration | Passed: root snapshot 1, separate @snapshots mount, requested retention values |
| Cron timeline creation and cleanup after boot | Passed helper execution: running /etc/cron.hourly/snapper created timeline snapshot 3; explicit timeline cleanup succeeded; cron and D-Bus services running |
| Existing partition table target | Passed: full Go TUI run replaced the previous GPT/Btrfs installation on NVMe |
| SATA target naming | Passed: final binary completed the full Go TUI DHCP installation on /dev/sda, then booted without ISO; console password login, sudo, DHCP, all mounts and services passed |
| NVMe target naming | Passed: /dev/nvme0n1p1 and /dev/nvme0n1p2 created, installed and booted without ISO |
| Persistent static IPv4 and reboot | Passed: selected eth0 hardware became voidnet0 with 10.0.2.15/24 and default gateway 10.0.2.2; remote official repository query succeeded |
| Injected XBPS / GRUB / Snapper failures in VM | Not performed in VM; mocked stage-command failures and cleanup passed in the Bash fixture suite |
| Terminal UI operation in the Live ISO | Full NVMe Static install passed through all seven screens and the Go runner; corrected Linux VT F2 opens and closes logs during actual SATA bootstrap |

No physical-machine installation has been performed. No host disks have been used as test targets.

## Host-side checks

The final source passes `go test ./...`, `go test -race ./...`, `go vet ./...`, ShellCheck 0.11.0 and `bash -n` for every shipped Bash script. The Bash fixture suite has 21 test groups, including independent validation, disk naming, private FD3 input, raw-handle checks, ambiguous live media, failed unmounts, injected XBPS/GRUB/Snapper command failures, the PAM no-op regression and Snapper policy generation. These command-injection tests are mocked, not destructive VM failure scenarios. Go tests cover input/configuration validation, flattened or mounted disk inventories, strict event parsing/order, unexpected and failed process exit, cancellation with a real child process, log redaction/permissions/rotation, screen transitions, resize, masked secrets, queued-command shutdown and the actual Linux VT F2 decoder.

`make build` produces a Linux amd64 CGO-disabled ELF executable. `file` reports statically linked; `readelf -l` has no interpreter segment. The same executable runs in the Live ISO without Go or source files.

Resolved binary package versions during the guest run include `base-system-0.114_2`, `linux6.18-6.18.56_1`, `grub-x86_64-efi-2.12_2`, `btrfs-progs-6.11_1`, `snapper-0.13.1_2`, `dhcpcd-10.3.2_4`, `cronie-1.7.2_2`, and `dosfstools-4.2_1`. The live network interface is `eth0`; the ISO command line does not contain `net.ifnames=0`. Installed DHCP retained eth0, while the static installation used the explicit hardware-pinned voidnet0 name.

For installed-boot diagnostics only, the test guest's existing public SSH key was copied into the installed root and the installed SSH service enabled after the engine had exited. These test access additions are not installer defaults. The first boot test runs `tests/qemu-live.sh --installed` with no ISO attached. Use `--disk-bus nvme` or `--disk-bus sata` for isolated controller-name scenarios; no arbitrary host device argument is accepted.

The first installed image also booted after changing its virtual controller from virtio to NVMe: Linux discovered `/dev/nvme0n1p2` and reached the same login prompt. This tested UUID-based boot/mount portability. The subsequent full NVMe installation separately verified its partitioning path.

The second installation uses the complete Go UI and process runner, with binary SHA256 `3157839c1521b4a06c3992a72f92cd2e76bd7c82a2aaffc7a09cd9c11fc1cc4f`. It includes the explicit SHA512 fix, mounted-descendant discovery fix, raw-open-handle protection, MAC-pinned static networking, and Snapper template workaround. The final review showed the exact NVMe target and static addresses; the required `ERASE /dev/nvme0n1` phrase was entered. A disposable test credential is held only in an ignored mode-0600 local test file and sent as QMP keystrokes without appearing in process arguments. The UI masks it. A private mode-0600 structured installation log was created under guest `/tmp`.

This run reached the success result screen. Its log ended with verification completed, cleanup completed and result success. After a clean OVMF boot with no ISO attached, console password login as `voidtest` succeeded. `sudo -k; sudo id` prompted for the same password and returned uid=0. Root login on tty2 also succeeded with the same password. No credential repairs were performed. The installed services `cronie`, `dbus` and `dhcpcd` were running. The five Btrfs subvolumes and FAT ESP mounted correctly. Running the packaged hourly Snapper helper created timeline snapshot 2, and `snapper cleanup timeline` succeeded. The configuration displayed the requested hourly/daily/weekly/monthly/yearly limits. Static DNS/network access was exercised using `xbps-query -R -p pkgver base-system`, which returned `base-system-0.114_2`.

The Linux console emits F2 as `ESC [[ B`. The pinned Ultraviolet parser reports an unknown CSI plus a letter rather than F2. A Linux-only Bubble Tea event filter now maps the immediately following legacy F1–F5 suffix to the corresponding function key; ordinary input and standard terminal F2 remain unchanged. A regression test exercises the real pinned decoder rather than a synthetic F2 message.

A final SATA/DHCP run uses binary SHA256 `e4038f1a37c22658da4f78218b769b458527b464baf8158f5f10c4e669e6e016`. This includes the Linux F2 compatibility filter and the additional fail-closed duplicate-live-identifier guard. In the actual Live console, F2 opened the logs during installation and switched back to stages. Partition creation on `/dev/sda`, formatting and bootstrap were reached. Ctrl+C displayed the cancellation confirmation; Y requested cancellation, and the UI waited before showing the unsuccessful result. The retained log recorded bootstrap failed, cleanup running/completed and result cancelled. After exit, `pgrep -af 'install.sh|xbps-install'` found no engine/package process and `findmnt -rn -o TARGET | grep void-installer` found no target mounts. The TUI could then be started again and passed its environment/disk checks. This verifies real subprocess termination and mount cleanup after a destructive stage, using only the isolated regular virtual disk.

The SATA/DHCP retry with that same final binary completed every stage and reached the successful result screen. The real log view scrolled with PageUp and resumed following with End; installation continued while it was open. The last log entries verified the kernel/initramfs, sudo authorization, initial snapshot and successful cleanup. Both EFI installation calls reported completion, and no Snapper systemctl warning occurred. This run validates the final source's additional safety guards on actual Void Live, independently of the earlier NVMe static run.

The final SATA image then booted using `bash tests/qemu-live.sh --installed --disk-bus sata`, with no CD-ROM argument. Linux 6.18.56_1 reached the installed `void` login prompt. User `voidtest` logged in with the untouched installer password; sudo authenticated it and returned uid=0. DHCP assigned eth0 10.0.2.15/24 and the default gateway 10.0.2.2. The five requested Btrfs subvolumes and EFI partition mounted from `/dev/sda`; cronie, dbus and dhcpcd were running, and the initial Snapper snapshot was present.

Running the packaged hourly helper on that final system created timeline snapshot 2; explicit timeline cleanup returned successfully. This exercises the packaged job, not a full hour of unattended scheduler timing. Physical hardware, Xfce Live ISO, forced NVRAM-write failure, simulated power loss and real package/bootloader/snapshot command failure injection remain unverified.

## Follow-up: preflight diagnostics

A user reported an environment-check failure that displayed only the tail of the XBPS dry-run package plan. That output does not identify the underlying failure. The earlier UI kept only eight trailing lines, and the CLI shared the same truncation. The revised checker saves the complete output in a mode-0600 temporary log on failure, identifies repository synchronization, package resolution or a specific package lookup together with its exit status, and explicitly distinguishes cancellation and timeout. Successful checks remove the temporary log. Each package query now has a 30-second timeout, and the overall check budget is eight minutes rather than four. No failing operation is ignored or treated as success.

Regression tests preserve an earlier diagnostic followed by 50 package-plan lines, verify the log's private permissions, check cancellation reporting and prove that a failed repository operation stops with its original exit code. `make check` (22 Bash groups), `go test -race ./...` and rebuilding the static binary passed. This diagnostic change did not repeat a destructive VM installation.

## Follow-up: low-memory Live temporary storage

The supplied full log identified the underlying failure: `ERROR: Transaction aborted due to insufficient disk space (need 1158MB, got 981MB free).` Package resolution exited 28 because XBPS checked the private temporary metadata root, not the selected installation disk. The earlier successful VM sessions had 3 GiB RAM, which masked this incorrect assumption.

The revised code separates this exact, source-verified temporary-root ENOSPC case from true repository/dependency errors. It strictly validates the emitted plan, sums installed and archive sizes with a 2 GiB reserve, and checks that amount against the real disk minus the ESP/alignment before erasure. Actual bootstrap still uses the mounted target and normal XBPS free-space checking. Tests cover normal and low-space dry runs, genuine conflicts, malformed plans, unexpected diagnostics accompanying exit 28, and insufficient target capacity. The updated 23-group Bash suite, Go tests/race checks and ShellCheck pass.

Actual non-destructive retest passed with `VM_MEMORY_MIB=2048 bash tests/qemu-live.sh --disk-bus sata`, booting the same verified official ISO. After live-tool preparation, `/tmp` was a 983 MiB tmpfs with about 978 MiB available; no tmpfs resizing or additional RAM was used. Binary SHA256 `0af791b2d5a346151e047faa4af7b78e8368a72af816086d23bdddc49a26b142` returned `Live environment checks passed.` from `--check`. This confirms the corrected preflight on less free temporary space than the reported 1158 MB installed-size requirement. No destructive installation was repeated for this fix, and no user-managed VM or host disk was changed.

## Follow-up: DHCP DNS and distribution filename

The earlier DHCP test did not prove DNS configuration: bootstrap copied Live `/etc/resolv.conf`, and the guest reused the same virtual network. A user report exposed that replacing packaged dhcpcd defaults with a comment removed explicit DNS option requests. The renderer now preserves defaults, adds the DNS request options for DHCP and static configuration, and verifies the installed resolver hook. Fixtures check retained defaults, static-to-DHCP transition, repeated rendering without duplicate blocks, and rejection of a missing hook. The 23-group Bash suite, Go tests/vet, Bash syntax checks and ShellCheck pass.

The existing isolated SATA guest was booted without its ISO. Its actual dhcpcd package provides `/usr/libexec/dhcpcd-hooks/20-resolv.conf`; inspection confirmed its direct resolver-file fallback when resolvconf is unavailable. After loading the revised network renderer, dhcpcd was stopped and `/etc/resolv.conf` deleted. Rendering the DHCP configuration and restarting dhcpcd recreated the file with `nameserver 10.0.2.3`. `getent ahostsv4 repo-default.voidlinux.org` then returned addresses successfully. This verifies resolver regeneration on the installed Void system; it does not reproduce the user's particular DHCP server or repeat a full destructive installation.

The current distribution is `dist/void-installer-tui`, a statically linked Linux amd64 executable with SHA256 `f2edc1799b35db70be13341a0b5a3a15b0ac2db90504cfa96a354e1915998998`. `make build` removes the previous executable name and writes the new name to `dist/SHA256SUMS`.

## Follow-up: minimal desktops, NVIDIA and optional utilities

The new selection flow adds Desktop and Utilities before Review. The package allowlist is embedded from `scripts/catalog.json`. Automated tests cover desktop allowlists, duplicate/unknown/injected utility values, no implicit Firefox/Kitty or full KDE bundle, checkbox/back navigation, scrollable review at 80x24 with visible erasure confirmation, NVIDIA PCI versus audio-function discovery, current versus legacy GPU tables, subsystem matching, and driver-version pinning/failure propagation. The Bash suite now has 27 groups, including NetworkManager static/DHCP files and selected-only niri launchers. Go tests, race tests, vet, ShellCheck and Bash syntax checks pass.

In the isolated installed Void SATA guest, real XBPS repository preflight passed for niri plus every optional utility and the NVIDIA/nonfree/header packages. It produced a valid multi-repository transaction plan and required-byte estimate before any disk operation. The exact NVIDIA package resolved as `nvidia-595.104.02_1`. A read-only check of the development machine's NVIDIA PCI identities against that version's official current-products list passed. No NVIDIA driver was installed or loaded on the development host; NVIDIA DKMS installation and boot on actual NVIDIA hardware remain unverified.

The existing dedicated VM image was extended with the selected KDE/niri and utility packages, rather than repeating disk erasure with this version. The first package attempt stopped on an obsolete cached index's 404; refreshing the index allowed installation of all packages. This exposed two real integration issues: the X11 SDDM greeter needs `xf86-input-libinput` explicitly, and separately supervising elogind can race its D-Bus activation. The graphical catalog now includes the input driver, installation verification checks its module, and the packaged SDDM service activates elogind through D-Bus without a competing elogind service.

After booting without the ISO, SDDM displayed its login screen. Basic QEMU VGA has no suitable accelerated renderer for niri: its process and input method ran but its log explicitly rejected the software renderer. This was not counted as a successful rendered desktop. The harness was extended with an optional VirGL/EGL mode; default basic graphics remains unchanged. The accelerated test used:

```sh
QEMU=/usr/bin/qemu-system-x86_64 VM_RENDER_NODE=/dev/dri/renderD129 \
  bash tests/qemu-live.sh --installed --disk-bus sata --graphics virgl
```

Use an available DRM render node on the test host, not a block device, and a QEMU build with VirGL/EGL support. This test used a virtual virtio GPU rendered by the host's integrated graphics, not NVIDIA passthrough. No user-managed VM was modified. For VirGL, QMP screendump returned `no surface`; screenshots were instead captured inside the guest.

In accelerated mode, real password login through SDDM started `dbus-run-session niri --session`; the log confirmed an initialized renderer and active 1280x800 Virtual-1 output. Super+Return and Super+B launched selected Kitty and Firefox, and niri IPC listed both actual windows. Fcitx5 was running; Ctrl+Space followed by the ASCII key sequence for Hangul produced visible Korean text in Firefox, verified in an actual compositor screenshot.

The same guest was then configured with the KDE profile. SDDM password authentication started `startplasma-wayland`, KWin and plasmashell. A guest screenshot showed the rendered Plasma desktop and panel. The KWin-launched Fcitx5 log confirmed native Wayland input protocol use with GTK_IM_MODULE and QT_IM_MODULE unset; Firefox again accepted visible Korean text. Spectacle was installed only as a test capture tool and is not in the installer catalog.

The KDE-generated NetworkManager static keyfile was activated with the selected Ethernet MAC, 10.0.2.15/24, gateway 10.0.2.2 and DNS 10.0.2.3. nmcli confirmed the exact values; the user retained wheel and gained network membership; NetworkManager and bluetoothd ran. After switching to the generated DHCP profile and deleting the resolver file, NetworkManager reacquired the address, route and DNS and recreated `/etc/resolv.conf`; repository-domain lookup succeeded. The competing dhcpcd service was absent. Audio, Bluetooth pairing, screen sharing, GTK/Qt application-by-application input coverage and a fresh full-install run with these profiles were not tested. Minimal audio/screen-sharing setup is outside these profiles and is documented in README.

A further clean boot without the ISO retained the KDE selection and NetworkManager DHCP/DNS. SDDM password login again started plasmashell and Fcitx5; NetworkManager and bluetoothd were running, repository-domain resolution succeeded, and neither a competing dhcpcd link nor a duplicate elogind service link was present.

The final `0.2.0-dev` distribution is `dist/void-installer-tui`, statically linked for Linux amd64, SHA256 `63896361baa169046d7f84ee0c5c01649af5af53e683993076ded9d6bafeef13`. The embedded engine includes the explicit greeter input driver and D-Bus-only elogind activation fixes. Its checksum file uses the same executable name.

## Follow-up: KDE display settings

The minimal KDE profile omitted `kscreen`, which provides the `kcm_kscreen` Display Configuration module. The profile now includes that package explicitly; existing installations can add it with `sudo xbps-install -S kscreen` and reopen System Settings. Upstream sources: [Void package](https://github.com/void-linux/void-packages/blob/master/srcpkgs/kscreen/template), [KDE settings module](https://github.com/KDE/kscreen/blob/master/kcm/CMakeLists.txt). A regression assertion requires kscreen in KDE and excludes it from console/niri profiles. `make check` passed, and the `0.2.1-dev` static distribution was rebuilt. The graphical session was not rerun for this package-list correction.

## Follow-up: audio, session integration and mirror selection (0.3.0-dev)

The package catalog now includes desktop audio (PipeWire, WirePlumber, JACK and ALSA compatibility), KDE's Konsole/login/audio settings/KWallet, and niri's GNOME/GTK portals, GNOME Keyring and Seahorse. Fcitx5 is one utility choice that expands to the complete supported Qt/GTK/configtool bundle; Hangul remains a separate choice and also brings in the bundle. Go regression tests assert bundle expansion, minimal-profile separation, mirror normalization/rejection, async repository checking, selected-mirror retention and CLI presets without hidden custom-field contents. The Bash fixture suite has 29 groups, including persistent same-basename mirror overrides, main/nonfree path consistency, audio fragments, optional PAM hooks, and GTK file selection. Tests also now validate utility strings before converting their JSON array to lines, preventing embedded-newline values from splitting into apparently valid packages.

The existing isolated installed Void guest received the additional real packages; package resolution and installation succeeded. The exact generated KDE configuration was applied. With an emulated Intel HDA sound card (`--audio hda`), a clean ISO-free boot and SDDM password login started one PipeWire core, WirePlumber and PipeWire's PulseAudio server. `wpctl status` listed actual emulated analog output and input devices, and `pactl info` reported `PulseAudio (on PipeWire 1.6.9)`. `pw-play` successfully played the packaged completion sound through the emulated device; the host audio backend was disabled, so audible speaker quality was not tested. ldconfig listed PipeWire's JACK library before the standalone library. Konsole reported its version, and the SDDM/volume plugins were present. KScreen was installed in this follow-up and its actual `kcm_kscreen.so` module confirmed. KDE's optional pam_kwallet5 hooks were installed once; automatic opening of a newly initialized KWallet was not verified (the wallet query returned false).

The generated niri configuration validated with the installed niri binary. After a further clean boot without the ISO, SDDM password login started niri, PipeWire, WirePlumber and pipewire-pulse; both the HDA output and input were present, and pw-play completed successfully. GNOME Keyring's login collection reported `Locked=false` over the session bus without any password supplied to a diagnostic command. Core, GNOME and GTK portal backends activated on demand, and the FileChooser and ScreenCast interfaces returned versions 4 and 5. The user portal preference explicitly selected GTK file selection and GNOME Keyring secrets. Seahorse installed successfully; its GUI was not independently verified. Actual browser screen-sharing and file-selection round trips, Bluetooth pairing, JACK application playback and physical audio hardware remain untested.

The same installed guest's XBPS configuration was changed using the actual mirror renderer. `xbps-query -L` subsequently listed `https://repo-fastly.voidlinux.org/current` as RSA signed, with the packaged default overridden. A fresh private-root transaction preflight for niri, the input bundle and desktop audio passed against Fastly. Main/nonfree overrides and custom roots containing `/voidlinux` or `/void` are also covered by fixtures; NVIDIA nonfree downloading was not repeated in this follow-up.

An actual official Live ISO session with 2 GiB RAM ran the single binary without sources. The welcome local check completed before repository access. The Mirror screen showed the preselected Fastly CDN, then performed the real repository check asynchronously and advanced to disk selection without erasing anything. A custom `https://repo.jing.rocks/voidlinux` attempt generated the correct `/voidlinux/current/x86_64-repodata` URL but the server returned Service Unavailable; the UI stayed on Mirror with the failing operation and private full-log path. No alternate mirror was silently selected. This also exercised real mirror-error reporting. The custom-field preset behavior discovered during this test was corrected and regression-tested. The final CLI `--check --mirror` was tested separately against Fastly. No fresh destructive installation or NVIDIA hardware boot was performed for this revision.

Final validation passed: Go tests/race, vet, 29 Bash fixture groups, Bash syntax and ShellCheck. The final single executable returned `Live environment checks passed.` from `--check --mirror https://repo-fastly.voidlinux.org` on the 2 GiB Live ISO. Distribution: `0.3.0-dev`, statically linked Linux amd64 `dist/void-installer-tui`, SHA256 `63dcaa6f64d44f632b185719ac64d61601b64cd10e5258522fdb1fac48c26aab`.

## Follow-up: common Bluetooth/TLP, password minimum and version header (0.4.1-dev)

The password minimum is eight printable ASCII characters in both Go and Bash. All profiles select bluez and tlp from the shared catalog and enable bluetoothd/tlp, with final service-link verification. The header displays the CLI build version. Regression assertions cover password boundaries, common package selection and the version header. Bash syntax checks and ShellCheck passed. Full Go tests, vet, race checks and distribution rebuilding could not be completed: the local Go tree is incomplete, module dependencies are uncached and network access is restricted. Bash fixtures could not run because jq is unavailable. No hardware or VM installation was performed, and dist remains the previous build.

## 2026-10-16: root labels and NVIDIA checkbox

Root Btrfs no longer receives a filesystem label; the root GPT partition name is explicitly empty. Desktop offers a separate `[ ]`/`[V]` NVIDIA driver checkbox using Space. UI regression coverage was updated for independent desktop/driver selection, toggling off, and unavailable NVIDIA hardware.

All 32 Bash fixture groups, Bash syntax checks, ShellCheck 0.11.0, and race tests and vet for `internal/installer`, `internal/validation` and `scripts` passed. Full `make check` could not complete because Bubble Tea/Bubbles/Lipgloss/Ultraviolet dependencies were not cached and network access is restricted. UI tests, the full race suite, and a new distribution build remain unverified for this revision; dist retains its existing binary. No installation or hardware test was performed.

## 2026-10-16: consistent controls and isolated review dialog

The final review replaces the header/sidebar/footer with a wide rectangular dialog centered on black. Summary scrolling is independent of the erase field and Back/Install controls. The exact phrase enables Install; Enter in the field only focuses the button, and a second Enter activates it. Tab/Shift+Tab and arrows navigate; Space selects/toggles; Enter confirms; Esc goes back. Static-network mode and interface selection now use focused controls plus Space instead of Ctrl+D/Ctrl+N. Account Tab wraps without submitting. Bluetooth/TLP remain mandatory common packages.

Regression tests were added/updated for centered 80x24, 100x30 and 160x50 layouts, hidden application chrome, explicit Install activation, disabled/wrong-disk confirmation, Back/reset behavior, account focus navigation and static-network controls. These UI tests could not run: `make check` and `make build` both stopped at uncached UI dependencies because network access is restricted. No new executable was produced; dist retains its existing binary. The 32 Bash fixture groups, Bash syntax, ShellCheck, and installer/validation/scripts race tests and vet passed separately. No VM or hardware installation was run.

## 2026-10-16: automatic Live dependency preparation

Welcome now runs the embedded `--prepare-live` mode before discovery. It validates the Live session without jq, takes the installer lock, installs missing Live tools using separate XBPS self-update and tool transactions, and repeats full environment validation. Complete toolsets skip package transactions. CLI `--check` and engine `--check-local` remain checks only. Preparation output is captured in private logs, with failure details and R retry in Welcome.

All 33 Bash fixture groups passed. New mocked scenarios cover missing jq, an already prepared session, rejection before package-manager execution, XBPS self-update failure, tool-installation failure, transaction order and final recheck. No host package manager or block device was used. Bash syntax, ShellCheck 0.11.0, installer/validation/scripts race tests and vet passed. Runner tests cover the new preparation mode argument. Full `make check` and `make build` remain blocked by uncached UI dependencies and restricted network access; the changed Welcome screen and real Live package installation have not been tested in a VM. Existing dist is unchanged.

## 2026-10-16: full build and regression validation completed

After network access became available, all required Go modules downloaded successfully. `make build`, `make check` (ShellCheck 0.11.0) and `go test -race ./...` passed, including the new UI dialog/control tests and all 33 Bash fixture groups. The distribution executable reports 0.4.1-dev and is a stripped, statically linked Linux x86-64 ELF. Both distribution SHA256SUMS entries verified. Installer SHA256: `66f3b2f2d59d4d65cf083b060ca85ded9e0030ea978c21e1ea7b8f1421c5ce80`. This supersedes the earlier dependency-related build/test limitations for the current source. Real Live dependency installation, VM boot and hardware installation remain untested for this revision.

## 2026-10-17: single-line header alignment

UI tests passed, including 80/100/160-column assertions that the title and right-aligned version share the first line and the separator follows immediately. Assertions strip ANSI styling before checking alignment. `make build`, executable version output and both distribution checksum checks passed. No installation was performed.

## 2026-10-17: seven UI improvements

Implemented inline input errors, a consistent Continue position and form submission control, live preparation status, disk details with serial/WWN and blocker guidance, fixed selection descriptions, a compact review summary, and recent installation log previews. Bluetooth/TLP and all engine safety checks remain in place. Disk inspection from failed Welcome cannot advance even when an individual disk is safe. Short review layouts retain the exact erasure field and buttons; the normal installation view shows recent sanitized diagnostics at 80x24.

`make check`, `go test -race ./...`, and `make build` passed. This includes 34 Bash fixture groups, vet, Bash syntax and ShellCheck 0.11.0. New tests cover split preparation markers and invalid markers, subprocess progress, skipped/failed preparation, inline account/gateway errors, explicit account Continue, fixed Continue/description placement, blocked disk details, inspection safety, review ordering and minimum-size log previews. Distribution version output, static Linux x86-64 ELF format and both SHA256SUMS entries verified. No actual Live dependency installation, VM boot or hardware installation was performed for these changes.

## 2026-10-18: GitHub Actions build and release workflow

Added a main-push validation workflow and version-input manual releases on GitHub-hosted ubuntu-latest runners, with Asia/Seoul and `[skip ci]` handling. Local `make check`, `go test -race ./...`, actionlint 1.7.12 and ShellCheck 0.11.0 passed. The existing 34 installer fixture groups and eight new release fixture groups passed. Release tests use a temporary Git repository and a mocked GitHub CLI; they cover initial/incremental notes, exactly seven hash characters, literal shell syntax in commit titles, invalid versions, duplicate tags/drafts, API failures, unchanged commits and paginated history filtering.

A Linux amd64 CGO-disabled sample build using the existing build flags and version `0.4.0-1` was created under `/tmp`, named `void-tui-installer-v0.4.0-1`, and verified through version output, static ELF inspection and its generated SHA256SUMS. Existing dist files were not replaced by this sample. The workflow has not been executed on GitHub, and no real tag, draft, public release or uploaded asset was created. End-to-end runner tool installation, authentication and GitHub publishing remain unverified.

Follow-up: main code pushes now proceed from validation to automatic publication. Manual dispatch requires no version input. The base version comes from Makefile (`0.4.1`) through `make version`, and tag/release/draft collisions select the first free positive revision suffix. With no new commits, publication succeeds as a no-op and stale notes are removed. The default Make target remains build. CI/release operating details were removed from both READMEs at the user's request; the architecture document retains them.

Local `make check`, the full race suite, actionlint/ShellCheck and `make build` passed again. Release fixtures now contain nine groups covering automatic suffix selection, draft-only collisions, base-version changes, invalid Makefile versions, literal notes, history filtering, API failure and successful no-op reruns. The rebuilt dist executable reports `0.4.1`; both checksum entries verified. No live GitHub workflow or release was run for this follow-up.

## 2026-10-19: Configuration, diagnostics and subprocess hardening

Local `make check`, `go test -race ./...` and `make build` passed. The final diagnostic integration-test extension also passed `go test -race ./internal/installer`. Bash fixtures now contain 36 groups; release fixtures remain nine groups. Validation tests reject original JSON strings containing LF, NUL, tab and DEL before command substitution. Mocked storage tests verify that a failed recheck prevents the next erase, partition-table creation or format command. They never open a block device.

Go tests cover redaction after control/ANSI/Unicode-format normalization, removal of OSC sequences, and event/private-log redaction with control-split secrets. Preparation fixtures verify cleanup of surviving background children after normal completion and cancellation of a SIGTERM-ignoring process group. These fixtures execute harmless shell/sleep processes only. No actual Live package installation, QEMU installation, physical hotplug race or hardware installation was performed. Repeated disk checks reduce but do not eliminate the non-atomic check/use window.

## 2026-10-19: Searchable repository package selection

The fixed Utilities menu was replaced by Packages, with an asynchronous selected-mirror catalog, local name/description filtering, version display, persistent checkbox selections, retry/reload and mirror-change invalidation. Additional packages retain the compatibility JSON key `utilities`; Go and Bash enforce name/selection limits independently. Go checks repository membership again at Start, and Bash checks exact pkgname before the existing dependency/conflict/capacity transaction preflight.

Local `make check` and the full race suite passed, including 38 Bash fixture groups and nine release fixture groups. Following the final UI refresh/error-state changes, the UI race suite passed again. `make build` produced the static Linux amd64 executable. New tests cover package parsing/deduplication/limits, exact membership, description/case-insensitive search, selection retention across filters, empty results, pending/failed loads, mirror changes, stale responses and removal of unavailable selections after refresh. Bash fixtures mock private metadata synchronization/query and ensure diagnostics stay off the package-record stream; unavailable packages and version expressions fail before transaction resolution.

A read-only memory-sync query of the official main x86_64 repository returned 14,823 records. `VOID_TEST_PACKAGE_LIST=/tmp/void-installer-package-query.txt go test ./internal/installer -run TestRepositoryPackageSnapshot -v` parsed all 14,823, including installed/uninstalled state markers. No host package installation or package-database update was performed. The complete guarded listing helper and selected-package installation have not been run in a fresh official Live ISO or QEMU installation. Nonfree/multilib browsing is outside this feature; NVIDIA remains separate.

## 2026-10-19: Selected-only view, settings, review checks and locked root

Local `make check`, `go test -race ./...` and `make build` passed with 40 installer Bash fixture groups, nine release fixture groups, vet, Bash syntax and ShellCheck. UI tests cover the Search/checkbox/list/Continue focus cycle, selected-only search intersection, immediate removal and empty-list index clamping. Settings tests cover mode-0600 atomic round trips without password/confirmation fields, valid settings replacement, unrelated-file/symlink/FIFO/oversized-file rejection, unsupported secret fields, trailing data, the minimum-size settings dialog and clearing credentials/consent/check results on load. A changed disk identity invalidates the saved target selection.

Review tests verify that pending or failed checks block installation, changed settings invalidate passed results, stale generations are ignored and failed check results fit 80x24. Bash fixtures run the non-destructive check function with mocked environment/lock/repository/disk commands, feed public JSON without FD 3, verify required/disk capacity records and preserve partial results on a dependency conflict. The real target engine still repeats its full preflight before writes.

The account fixture intercepts chpasswd input and proves it contains only the selected user's password line. Mocked usermod locks the fixture root shadow entry; final verification accepts that lock and the user SHA512 hash, while wheel sudo remains password-protected. The existing PAM no-op fixture still rejects an unchanged user password without printing the secret. No actual root account, host shadow/sudoers file, host block device or installed system was modified. End-to-end Live/QEMU checks, persistent-media settings reload and a fresh installation with locked root remain unverified.

## 2026-10-19: Patched CI toolchain and vulnerability gates

The workflow now resolves the latest Go 1.26 patch, passes the resolved version from validation to release, and runs pinned govulncheck v1.9.0 on source including tests and on the release binary before publication. GOTOOLCHAIN=local prevents silent toolchain changes. The go.mod minimum version and application dependencies are unchanged.

The official Go 1.26.9 toolchain was downloaded into a temporary module cache and used with automatic toolchain switching disabled. Source/test scanning and scanning of a stripped static Linux amd64 binary built under /tmp both reported `No vulnerabilities found`. Local `make check` and `go test -race ./...` passed, including 40 installer Bash fixture groups, nine release groups, vet, Bash syntax and ShellCheck. Actionlint with ShellCheck and git diff whitespace checks passed. Existing dist artifacts were preserved. No GitHub workflow, release publication or installer execution was performed for this change.

## 2026-10-23: Go toolchain vulnerability review

The official Go release history lists Go 1.26.10 and Go 1.27.3 as the current patched releases; both include the `crypto/x509` security fix. The workflow selects the latest available 1.26 patch with `check-latest: true`, then passes that exact resolved version from validation to release, so its selected toolchain includes the fix. The Go vulnerability database's latest `crypto/x509` report is GO-2026-6809 (published 2026-10-21); no vulnerable workflow version was configured. Running pinned govulncheck v1.9.0 against source and tests with the current database reported `No vulnerabilities found`. No workflow or dependency changes were needed. A local release-binary scan was not repeated.

## 2026-10-19: ext4 CI ShellCheck repair

The user supplied an Actions failure showing SC2016 on jq expressions and SC2030/SC2031 after the isolated ext4 TARGET assignment. Both were reproduced locally with ShellCheck 0.11.0. The jq expressions now have two narrowly scoped SC2016 annotations because jq, not Bash, expands the --arg variable. Subsequent fixtures explicitly restore their original target after the isolated ext4 case, removing the subshell data-flow warnings without disabling them.

ShellCheck passed across every script covered by make check, and git diff whitespace validation passed. Go tests, Bash fixtures, race checks, builds and installations were not rerun for this correction. GitHub Actions has not yet rerun with the fix.

## 2026-10-28: Wired connection display name

The generated KDE/niri NetworkManager profile now uses `Wired connection 1` as its display name for DHCP and static IPv4. All 42 Bash fixture groups passed, including both desktop network renderers. Bash syntax, ShellCheck 0.11.0 for the changed script and git diff whitespace checks passed. No distribution rebuild, Go tests or Live/QEMU installation was performed for this label-only change.
