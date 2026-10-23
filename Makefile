.DEFAULT_GOAL := help

# Wails tasks invoke wails3 themselves, so expose the project-local CLI to them.
export PATH := $(CURDIR)/.tools/bin:$(PATH)
WAILS_VERSION := $(shell awk '$$1 == "require" && $$2 == "github.com/wailsapp/wails/v3" {print $$3} $$1 == "github.com/wailsapp/wails/v3" {print $$2}' go.mod)

.PHONY: help dev run build check doctor install-cli install-garble package-all

help:
	@printf '%s\n' 'Postika commands:' '  make dev          Start the desktop app with hot reload' '  make run          Alias for make dev' '  make build        Build the desktop executable' '  make check        Check Svelte and TypeScript' '  make doctor       Check Wails dependencies' '  make install-cli  Install the pinned Wails CLI locally' '  make install-garble Install optional pinned obfuscator' '  make package-all  Package unsigned formats for this OS'

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

package-all:
	wails3 task package:all
