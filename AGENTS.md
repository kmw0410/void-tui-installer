# 프로젝트 작업 지침

## 프로젝트 개요와 구조

이 프로젝트는 공식 Void Linux Live ISO에서 실행하는 Go/Bubble Tea v2 TUI와 Bash 설치 엔진이다. 설치 대상은 x86_64 glibc, UEFI/GPT, GRUB, Btrfs/ext4이며 개발 호스트에서 실제 설치를 실행하지 않는다.

- `cmd/void-installer/`: 실행 진입점과 CLI 옵션.
- `internal/ui/`: 화면, 입력, 레이아웃, 설치 세션 UI.
- `internal/installer/`: 설정, 장치 탐색, 사전 검사, 엔진 실행, 이벤트 및 로그 처리.
- `internal/validation/`: 입력값 검증.
- `scripts/install.sh`, `scripts/lib/`: Bash 설치 엔진과 단계별 구현.
- `scripts/embed.go`: 설치 스크립트와 JSON 자료를 실행 파일에 포함한다.
- `scripts/catalog.json`: Go와 Bash가 공유하는 기본·파일시스템·데스크톱 패키지 구성.
- `scripts/timezones.json`: 양쪽에서 공유하는 시간대 허용 목록.
- `tests/`: Bash fixture 테스트와 QEMU 검증 도구.
- `docs/architecture.md`: 구성, 프로토콜, 보안 경계와 설계.
- `docs/testing.md`: 실제 검증 결과와 미검증 항목.
- `docs/research.md`: 공식 문서와 upstream 조사 기록.
- `pkg-install.sh`: Live 환경의 필수 도구 준비 스크립트.
- `dist/`: 빌드 실행 파일, 배포용 준비 스크립트와 체크섬.

## 구현 원칙

- 작업 전 `README.md`와 관련 설계 및 검증 문서를 확인한다. 실제 코드와 문서가 다르면 확인 후 해당 변경 범위의 문서도 갱신한다.
- Go는 탐색, 입력, 검토와 엔진 세션을 담당하고, 실제 설치 작업은 Bash가 담당한다.
- 설정 및 허용 목록 변경 시 Go와 Bash 양쪽의 독립 검증을 유지한다. 공유 JSON 자료와 관련 테스트도 함께 확인한다.
- 디스크 삭제 전 정확한 `ERASE <disk>` 확인, 장치 식별 재검사, 마운트·swap·의존 장치·Live 매체 보호를 유지한다. 불확실한 상태에서는 설치를 차단한다.
- 암호는 별도 FD 3으로 전달하며 설정 JSON, 명령 인자, 이벤트 또는 로그에 노출하지 않는다. `eval`을 사용하지 않는다.
- 엔진 stdout은 JSON Lines 이벤트 전용으로 유지한다. 진단은 stderr로 보내고, 취소 시 프로세스 회수와 마운트 정리가 끝나도록 한다.
- 프로그램 문자열과 코드 주석은 영어로 작성한다. Go 코드는 `gofmt`로 정리하고 기존 코드 스타일을 따른다.
- 기능이나 설치 동작을 변경하면 관련 README 및 설계 문서를 갱신하고, 검증 결과는 실제 수행한 범위만 기록한다.

## 빌드와 검증

개발 환경에는 Go 1.26 이상, Bash, jq, ShellCheck가 필요하다. race 검사에는 C 컴파일러도 필요하다.

```sh
make build
make check
go test -race ./...
```

- `make build`: CGO를 끈 Linux amd64 실행 파일과 `dist/SHA256SUMS`를 생성한다.
- `make test`: Go 테스트와 디스크를 포맷하지 않는 Bash fixture 테스트를 실행한다.
- `make check`: 위 테스트, `go vet`, Bash 구문 검사와 ShellCheck를 실행한다.
- 필요하면 `GO=/path/to/go`, `SHELLCHECK=/path/to/shellcheck`로 도구 경로를 지정한다.
- 코드 변경에 맞는 검증을 수행하고 실행하지 못한 검사와 이유를 보고한다. 문서만 변경한 경우 내용과 형식을 확인한다.
- 실제 설치 검증은 `docs/testing.md`와 `tests/qemu-live.sh`를 참고해 전용 일반 파일 가상 디스크에서만 수행한다. 호스트 블록 장치를 QEMU나 설치 엔진에 전달하지 않는다.

## 커밋 메시지

- 커밋 제목은 `<type>(<scope>): <summary>` 형식으로 작성한다.
- type은 변경 성격에 맞춰 feat, fix, docs, refactor, test, build, ci, chore 등을 선택한다.
- scope는 실제 변경 대상이나 기능 영역을 간결하게 적으며, 고정된 예시 단어를 반복하지 않는다.
- summary는 변경 내용을 구체적이고 간결한 영어로 작성한다.
- 사용자가 제시한 형식 예시를 실제 커밋 제목으로 복사하지 않는다.
- 공동작업자 트레일러는 사용자가 별도로 요청할 때만 추가한다.

## 작업 기록: WORK.md

작업을 마친 뒤 저장소 루트의 `WORK.md`에 작업 내용을 기록한다. 파일이 없으면 생성한다. 파일명은 대문자 `WORK.md`로 통일한다.

```markdown
# 2026-10-16 | 작업내용 전체 한줄요약
- 작업A 설명
- 작업B 설명
```

- 날짜는 예시를 고정해서 쓰지 않고 작업 당일의 날짜를 `YYYY-MM-DD` 형식으로 작성한다.
- 같은 날짜의 항목이 이미 있으면 날짜 제목을 새로 추가하지 않고, 기존 날짜 항목의 마지막 작업 다음 줄에 `- 설명`을 이어서 작성한다.
- 같은 날짜에 작업이 여러 개 누적되면 제목의 한 줄 요약도 그날 기록된 전체 작업을 포괄하도록 갱신한다.
- 기존 기록을 보존하고 수행한 변경과 검증 결과를 구체적으로 적는다.
