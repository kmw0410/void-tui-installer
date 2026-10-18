GO ?= go
SHELLCHECK ?= shellcheck
VERSION ?= 0.4.1
.DEFAULT_GOAL := build

.PHONY: version build test check clean
version:
	@printf '%s\n' '$(VERSION)'

build:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags '-s -w -X main.version=$(VERSION)' -o dist/void-installer-tui ./cmd/void-installer
	rm -f dist/void-installer
	install -m 0755 pkg-install.sh dist/pkg-install.sh
	cd dist && sha256sum void-installer-tui pkg-install.sh > SHA256SUMS

test:
	$(GO) test ./...
	bash tests/bash_test.sh
	bash tests/release_test.sh

check: test
	$(GO) vet ./...
	for script in pkg-install.sh scripts/install.sh scripts/lib/*.sh tests/*.sh .github/scripts/*.sh; do bash -n "$$script" || exit; done
	$(SHELLCHECK) -x pkg-install.sh scripts/install.sh scripts/lib/*.sh tests/*.sh .github/scripts/*.sh

clean:
	rm -rf dist
