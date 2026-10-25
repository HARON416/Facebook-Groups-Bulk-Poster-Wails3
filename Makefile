.DEFAULT_GOAL := help

# Wails tasks invoke wails3 themselves, so expose the project-local CLI to them.
export PATH := $(CURDIR)/.tools/bin:$(PATH)
WAILS_VERSION := $(shell awk '$$1 == "require" && $$2 == "github.com/wailsapp/wails/v3" {print $$3} $$1 == "github.com/wailsapp/wails/v3" {print $$2}' go.mod)

.PHONY: help dev run build check doctor install-cli install-garble package-all release-all version-check version-sync version-bump version-set

BUMP ?= patch
export POSTIKA_RELEASE_BUMP := $(BUMP)
export POSTIKA_RELEASE_VERSION := $(VERSION)

help:
	@printf '%s\n' 'Postika commands:' '  make dev          Start the desktop app with hot reload' '  make run          Alias for make dev' '  make build        Build the desktop executable' '  make check        Check Svelte and TypeScript' '  make doctor       Check Wails dependencies' '  make install-cli  Install the pinned Wails CLI locally' '  make install-garble Install optional pinned obfuscator' '  make package-all  Package unsigned formats for this OS'
	@printf '%s\n' '  make release-all  Bump patch, commit/push version metadata, build all OS packages and flatten downloads' '                    BUMP=minor|major|none or VERSION=0.3.2 selects the version policy'
	@printf '%s\n' '  make version-check Check all platform versions agree' '  make version-sync  Sync platform metadata from build/config.yml' '  make version-bump  Increment patch; BUMP=minor or BUMP=major also supported' '  make version-set VERSION=0.2.0  Set a higher version across all platforms'
	@printf '%s\n' '  make update-feeds Sign and verify update feeds for the current downloaded release'

dev run:
	wails3 dev

build:
	wails3 build

check:
	npm --prefix frontend run check

doctor:
	wails3 doctor

install-cli:
	CGO_ENABLED=0 GOBIN="$(CURDIR)/.tools/bin" go install github.com/wailsapp/wails/v3/cmd/wails3@$(WAILS_VERSION)


# Garble is optional; keep the audited version local to this checkout.
install-garble:
	GOBIN="$(CURDIR)/.tools/bin" go install mvdan.cc/garble@v0.16.0

package-all: version-check
	wails3 task package:all

release-all:
	node scripts/release-all.mjs

.PHONY: update-feeds
update-feeds:
	node scripts/update-feeds.mjs

version-check:
	node scripts/version.mjs check

version-sync:
	node scripts/version.mjs sync

version-bump:
	node scripts/version.mjs bump

version-set:
	node scripts/version.mjs set
