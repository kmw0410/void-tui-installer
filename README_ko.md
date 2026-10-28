한국어 | [English](README.md)

# Void Linux TUI 설치 프로그램 (비공식)

공식 Void Linux Live ISO에서 실행하는 설치 프로그램입니다. Go/Bubble Tea v2 기반 TUI와 Bash 설치 엔진으로 구성되어 있으며, 설치 대상은 x86_64 glibc, UEFI/GPT, GRUB, Btrfs 또는 ext4 구성입니다.

<details>
<summary>스크린샷</summary>
<ul>
  <li><p>1. 시작 및 Live 환경 준비</p><img src="img/welcome.png" alt="Live 환경 준비 단계가 표시된 시작 화면" width="100%"></li>
  <li><p>2. 저장소 미러</p><img src="img/mirror.png" alt="사용자 지정 HTTPS 저장소 미러 선택 화면" width="100%"></li>
  <li><p>3. 설치 디스크</p><img src="img/disk.png" alt="용량, Serial, 사용 가능 상태가 표시된 설치 디스크 선택 화면" width="100%"></li>
  <li><p>4. 계정 및 호스트 이름</p><img src="img/account.png" alt="사용자 이름, 마스킹된 비밀번호, 호스트 이름 입력 화면" width="100%"></li>
  <li><p>5. 시간대</p><img src="img/timezone.png" alt="Asia/Seoul이 선택된 시간대 검색 화면" width="100%"></li>
  <li><p>6. 네트워크</p><img src="img/network.png" alt="DHCP가 선택된 네트워크 설정 화면" width="100%"></li>
  <li><p>7. 데스크톱</p><img src="img/desktop.png" alt="콘솔, KDE Plasma, Niri 선택과 NVIDIA GPU 미탐지 안내 화면" width="100%"></li>
  <li><p>8. 패키지</p><img src="img/utilities.png" alt="선택 패키지 설치 화면" width="100%"></li>
  <li><p>9. 검토 및 디스크 삭제 확인</p><img src="img/review-installation.png" alt="설치 요약과 디스크 삭제 확인을 표시하는 중앙 검토 창" width="100%"></li>
</ul>
</details>

## 면책 조항

이 설치 프로그램의 사용 여부와 그에 따른 책임은 사용자에게 있습니다. 사용으로 인해 발생하는 문제에 대해 책임지지 않습니다.

테스트 환경은 HP Victus 15(15-fa0051tx, i5-12450H/RTX 3050 Laptop GPU)와 QEMU VM입니다. 다른 환경에서는 결과가 달라질 수 있으며, 예상치 못한 문제가 발생할 수 있습니다. 저장소에 기록된 검증 범위와 미검증 항목은 [검증 기록](docs/testing.md)을 참고하세요.

설치하면 선택한 디스크의 데이터가 삭제됩니다. 정확한 `ERASE <disk>` 확인 문구가 필요하며, 각 파티션 작업과 포맷 명령 전에 장치 식별 정보와 사용 상태를 재검사합니다. 검사가 하드웨어 변경을 원자적으로 차단하지는 못하므로 설치가 끝날 때까지 장치를 연결한 상태로 유지하고 다른 디스크 작업을 피하세요. 진단 로그는 터미널 제어 시퀀스를 제거하고 입력한 암호를 가립니다.

## 의존성

설치 프로그램에는 다음 패키지가 필요합니다.

- `xbps`
- `jq`
- `gptfdisk`
- `parted`
- `dosfstools`
- `btrfs-progs`
- `e2fsprogs`
- `psmisc`

XBPS와 Live 환경의 기본 도구는 미리 제공되어 있어야 합니다. 나머지 설치 도구가 없거나 인식되지 않으면 초기 화면에서 XBPS를 먼저 갱신한 뒤 필요한 패키지를 자동으로 설치합니다. 모든 도구가 있으면 패키지 설치를 건너뜁니다. 별도 스크립트(`bash ./pkg-install.sh`)로 수동 준비할 수도 있습니다. 패키지 설치에는 Live 환경에 설정된 저장소와 이미 연결된 네트워크가 필요합니다.

## 사용 방법

### 빌드

개발 머신에 Go 1.26 이상, Git, Make를 설치한 뒤 저장소를 복제하고 빌드합니다.

```sh
git clone https://github.com/kmw0410/void-tui-installer.git
cd void-tui-installer
make build
```

정적으로 링크된 Linux amd64 실행 파일 `dist/void-installer-tui`, 수동 준비 스크립트 `dist/pkg-install.sh`, 체크섬 파일 `dist/SHA256SUMS`가 생성됩니다. 실행 파일을 Live 환경으로 복사하세요. Live 환경에는 Go, Git, 소스 파일이 필요하지 않습니다. 개발 검사를 실행하려면 Bash, jq, ShellCheck, C 컴파일러도 설치합니다.

```sh
make check
go test -race ./...
```

### 실행

KDE와 niri의 설치된 NetworkManager 유선 연결 이름은 `Wired connection 1`입니다.

공식 x86_64 glibc Live ISO를 UEFI 모드로 부팅하고 Secure Boot를 끈 뒤 네트워크에 연결하세요. 최소 80×24 크기의 터미널에서 root로 실행합니다.

직접 빌드하는 대신 Releases의 바이너리를 사용할 수도 있습니다. 같은 릴리스의 `SHA256SUMS`를 내려받아 `sha256sum -c SHA256SUMS`로 검증하고, 아래 명령의 `void-installer-tui`를 내려받은 버전별 파일명(예: `void-tui-installer-v0.4.0-1`)으로 바꾸세요.

복사한 실행 파일이 있는 디렉터리에서 실행 권한을 부여한 뒤 실행합니다.

```sh
chmod +x ./void-installer-tui
./void-installer-tui
```

`--check` 옵션은 Live 환경과 기본 설치 패키지의 저장소 접근·해결 가능 여부만 검사합니다. 의존성을 자동으로 설치하지 않으므로 초기 준비가 끝난 뒤 실행하세요.

```sh
./void-installer-tui --check
```

**설치하면 선택한 디스크의 모든 데이터가 삭제됩니다.** Review에서 `ERASE /dev/DEVICE`를 정확히 입력하고 `Install` 버튼을 실행해야 설치가 시작됩니다.

VM에서 `Stable serial or WWN is required`가 표시되면 VM을 완전히 종료한 뒤 설치 대상 디스크에 고유한 Serial(예: `VOIDTEST001`)을 지정하고 다시 부팅하세요. virt-manager/libvirt XML에서는 해당 `<disk device='disk'>` 요소 안에 `<serial>VOIDTEST001</serial>`을 추가합니다. Live 환경에서 다음 명령으로 확인할 수 있습니다.

```sh
lsblk -d -o NAME,SIZE,TYPE,SERIAL,WWN
```

## 진행 과정

Welcome → Mirror → Disk → Account → Timezone → Network → Desktop → Packages → Review 순서로 진행합니다. Timezone에서는 도시나 지역 이름을 입력하고 방향키 또는 PageUp/PageDown으로 이동한 뒤 Enter로 시간대(예: `Asia/Seoul`)를 선택합니다. 기본값은 UTC이며 NTP 서비스는 추가하지 않습니다.

Disk에서 위/아래 방향키로 디스크를 고르고 좌/우 방향키 또는 Space로 Btrfs(기본값)와 ext4를 선택한 뒤 진행합니다. 선택한 파일시스템은 Review와 저장 설정에 반영됩니다. 해당 항목이 없는 이전 설정은 Btrfs로 처리합니다. ext4는 단일 루트 파일시스템을 사용하며 Snapper를 자동 구성하지 않습니다.

방향키 또는 Tab/Shift+Tab으로 이동하고, Space로 선택·체크를 전환하고, Enter로 확정하며, Esc로 이전 화면으로 돌아갑니다. 마지막 Review는 검은 배경 중앙의 사각형 창으로 표시됩니다. PageUp/PageDown으로 요약을 스크롤할 수 있으며, 삭제 확인 입력과 Back/Install 버튼은 계속 표시됩니다. 입력칸에서 Enter를 누르면 Install로 이동하고, 해당 버튼에서 Enter를 한 번 더 눌러야 설치가 시작됩니다.

Review에서는 Live 환경, 네트워크 어댑터, 미러·의존성, NVIDIA 지원, 디스크 안전성, 필요 용량을 디스크 변경 없이 검사합니다. 요약 위에 검사 상태를 표시하며, 스크롤하면 항목별 결과와 용량을 확인할 수 있습니다. 현재 설정에 대한 검사가 통과하고 정확한 삭제 확인 문구를 입력해야 Install이 활성화됩니다. 실패하면 Ctrl+R로 재검사합니다. 실제 설치 엔진도 삭제 전에 안전 검사를 반복합니다.

### 설정 저장·불러오기

Review에서 F5로 설정을 저장하고, 설치 전 화면에서 F9로 불러옵니다. 창에서 JSON 파일 경로를 지정하며 기본값은 `~/void-installer-settings.json`입니다. 저장 파일 권한은 0600이고, 암호와 삭제 확인 문구는 저장하지 않습니다. 불러오면 암호·삭제 확인 입력을 비우고 디스크를 다시 탐색한 뒤 Mirror로 돌아가 미러와 대상을 확인하게 합니다. 디스크가 변경되었거나 없거나 안전하지 않으면 다시 선택해야 합니다. Live 환경 종료 후에도 쓰려면 영구 저장 매체에 파일을 보관하세요. 기존의 정상 설정 파일만 덮어쓰며, 무관한 파일과 심볼릭 링크는 거부합니다.

선택한 사용자에게만 암호를 설정하고 `wheel` 그룹과 암호 기반 sudo 권한을 부여합니다. root 암호는 잠그며 관리자 작업은 sudo로 수행합니다.

Welcome에서는 준비 단계별 진행·완료·생략·실패 상태를 표시합니다. 설정 화면의 Continue는 같은 하단 위치에 고정되며, 선택 목록에서 Tab으로 버튼에 이동하거나 Enter로 현재 선택을 확정하고 진행합니다. 계정·고정 네트워크 폼에서는 Enter로 입력칸을 거쳐 Continue로 이동한 뒤 제출합니다. 잘못된 값의 설명은 해당 입력칸 아래에 표시합니다. 디스크 화면은 목록과 모델·용량·Serial/WWN·차단 사유를 분리하며, Welcome 검사 실패 시 D로 정보만 조회할 수 있습니다. Desktop·Packages의 설명은 선택 목록 아래에 고정합니다. Review에는 선택한 데스크톱·드라이버·추가 패키지 개수를 먼저 보여주고 패키지·서비스·저장소 상세를 뒤에 배치합니다. 설치 중에는 민감 정보를 제거한 최근 로그 몇 줄을 표시하며 F2로 전체 로그를 엽니다.

## 설치 프로필

콘솔 전용(CLI), KDE Plasma, Niri 중 하나를 선택할 수 있습니다. KDE Plasma는 `plasma-desktop`, `sddm`, `kwallet-pam`, `kf6-kwallet` 등을 설치합니다. Niri는 `niri`, `sddm`, `xdg-desktop-portal-gnome`, `gnome-keyring`, `seahorse` 등 세션에 필요한 패키지를 설치합니다.

CLI를 포함한 모든 프로필에 `bluez`와 `tlp`를 설치하고 `bluetoothd`, `tlp` runit 서비스를 활성화합니다.

두 데스크톱 프로필에는 `pipewire`, `wireplumber`, `libjack-pipewire`, `alsa-pipewire`를 기본으로 포함합니다. Firefox와 Kitty는 자동으로 설치하지 않으며 Packages에서 선택해야 합니다.

## 패키지 설치

Packages에서는 선택한 Void 미러의 x86_64 glibc 기본 저장소(`/current`)에서 패키지 목록을 받아옵니다. 글자를 입력하면 이름과 설명으로 검색하며, 방향키 또는 PageUp/PageDown으로 이동하고 Space로 체크박스를 선택·해제합니다. 목록에는 이름과 버전을, 하단에는 설명을 표시합니다. 검색어를 바꿔도 선택은 유지됩니다. Tab/Shift+Tab은 검색창 → Show selected only → 패키지 목록 → Continue 순서로 이동합니다. Show selected only에서 Space를 누르면 선택한 패키지만 표시하며 검색 조건도 함께 적용합니다. 이 상태에서 선택을 해제하면 즉시 숨겨집니다. 검색창·목록·Continue에서 Enter로 Review에 진행합니다. 목록을 불러오지 못하면 Ctrl+R로 재시도합니다. 추가 패키지는 최대 256개를 선택할 수 있으며, 미러를 변경하면 기존 목록과 선택은 초기화됩니다.

목록은 Live 환경의 저장소 서명키와 별도 임시 메타데이터를 사용해 XBPS로 확인합니다. 삭제 전에는 패키지 존재 여부, 의존성 충돌, 대상 디스크 용량을 다시 검사합니다. 이 목록에는 nonfree·multilib 패키지를 포함하지 않으며, 별도의 NVIDIA 옵션이 필요한 nonfree 패키지를 처리합니다.

`fcitx5`를 선택하면 `fcitx5-qt`, `fcitx5-qt5`, `fcitx5-qt6`, `fcitx5-gtk`, `fcitx5-gtk+3`, `fcitx5-gtk4`, `fcitx5-configtool`도 함께 설치합니다. `fcitx5-hangul`만 선택해도 같은 묶음을 설치하고 영어/한글 입력 프로필을 생성합니다. `openssh`를 설치해도 `sshd` 서비스는 자동으로 활성화하지 않습니다.

## Niri 기본 키 바인딩

Niri에는 다음 실행 단축키를 제공합니다. Mod는 Super 키(일반적으로 Windows 키)를 뜻합니다. Firefox와 Kitty 단축키는 해당 패키지를 Packages에서 선택한 경우에만 추가됩니다.

- `Mod` + `B`: Firefox 실행
- `Mod` + `Enter`: Kitty 실행
- `Mod` + `Shift` + `E`: 세션 종료

## NVIDIA GPU

보조 GPU를 포함한 NVIDIA 디스플레이 컨트롤러를 자동으로 탐지합니다. Desktop에서 `[ ] Install NVIDIA driver` 체크박스로 이동한 뒤 Space를 눌러 `[V]`로 활성화하면 드라이버를 함께 설치합니다. 데스크톱 선택과 독립적인 옵션입니다.

이 옵션은 nonfree 저장소와 `linux-headers`를 추가합니다. 삭제 전에 탐지한 GPU가 해당 저장소의 드라이버 버전에서 지원되는지 검사하며, 지원되지 않거나 검증할 수 없으면 설치를 차단합니다. 레거시 드라이버는 자동으로 선택하지 않습니다.

## 설치 후 구성

두 파일시스템 모두 1 GiB FAT32 EFI 파티션과 나머지 공간의 루트 파티션을 사용합니다. ext4에서는 `/`를 `defaults,noatime`으로 마운트하고 루트 파일시스템 검사를 활성화합니다. `/home`, 로그, 캐시는 같은 파일시스템의 일반 디렉터리입니다. Snapper를 자동 설치·구성하지 않으며 해당 설치 단계는 생략으로 표시합니다.

기본 Btrfs 구성은 다음과 같습니다.

| 파티션 / 서브볼륨 | 마운트 위치 | 설정 |
| --- | --- | --- |
| EFI, 1 GiB FAT32 | `/boot/efi` | GRUB UEFI 및 fallback 로더 |
| 남은 공간, Btrfs `@` | `/` | `noatime,compress=zstd:3` |
| `@home` | `/home` | 같은 Btrfs 파일시스템 |
| `@snapshots` | `/.snapshots` | root 전용 Snapper 저장소 |
| `@log` | `/var/log` | 루트 스냅샷에서 제외 |
| `@cache` | `/var/cache` | 루트 스냅샷에서 제외 |

루트 파일시스템과 루트 GPT 파티션에는 라벨을 붙이지 않습니다. `/etc/fstab`은 UUID를 사용합니다. swap, 암호화, 듀얼 부팅, 사용자 지정 파티션 분할은 제공하지 않습니다. Btrfs는 `subvol=@`로 부팅하고 ext4는 서브볼륨 옵션 없이 루트 UUID로 부팅합니다. 엔진은 대상 마운트를 해제하기 전에 EFI 파일, GRUB 설정, 서로 대응하는 커널/initramfs 파일, 파일시스템 종류·장치, 계정, sudo, 패키지, 선택한 그래픽 설정과 Btrfs의 Snapper를 검증합니다.

Btrfs에서는 Snapper가 최초 루트 스냅샷을 생성하고 cronie 및 D-Bus와 함께 Void 패키지의 시간별 cron 도우미를 사용합니다. 타임라인 보존 개수는 시간별 5개, 일별 7개, 주별 4개, 월별 3개, 연별 0개입니다. 기존 디렉터리 충돌을 피하기 위해 별도의 `@snapshots` 서브볼륨을 마운트하기 전에 Snapper 설정을 생성합니다. 홈, 로그, 캐시, EFI 내용은 루트 스냅샷에 포함되지 않습니다. **자동 롤백, 복원 도구, grub-btrfs 통합은 제공하지 않습니다.** Snapper의 기본 서브볼륨 롤백은 명시적인 `subvol=@` 부팅 루트를 전환하지 않습니다.
