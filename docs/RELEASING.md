# Distributing Postika

Reviewed against Wails v3 documentation on 2026-10-10 and the project's pinned
Wails `v3.0.0-beta.28` CLI and Go dependency. Downloads will be hosted on our own
website. This is a release plan; no production installers have been published.

## Confirmed release metadata

| Field | Value |
| --- | --- |
| Version | `0.1.0` (Windows numeric resource version `0.1.0.0`) |
| Publisher | Postika Technologies |
| Website | https://postika.softwrhq.com |
| Support | haronkibetrutoh@gmail.com |
| Distribution license | MIT; see the root `LICENSE` |
| Platforms | Windows and Linux; macOS deferred |
| Package signing | Unsigned for this release |

The metadata is applied to `build/config.yml`, Windows resources and NSIS/MSIX
assets, Linux nFPM packages, and maintained desktop entries. NSIS installs
`LICENSE.txt`; Linux packages install `/usr/share/doc/postika/copyright`.
The license is also bundled in the application's frontend assets (including
AppImage builds) at `licenses/postika.txt`. Keep that copy in sync with `LICENSE`.
Linux `.desktop` files use `Version=1.0` for the desktop-entry specification;
`X-AppImage-Version=0.1.0` records the application version.
Default release packaging uses Windows NSIS and Linux DEB/AppImage, without
calling signing tasks. MSIX and RPM/Arch recipes remain optional.

## First release formats

| Platform | Website download | Build environment | Initial target |
| --- | --- | --- | --- |
| Windows | NSIS installer with WebView2 bootstrapper | Windows with NSIS | amd64; arm64 after native testing |
| Linux | DEB and AppImage; RPM after Fedora testing | Linux with GTK4/WebKitGTK development libraries | amd64; arm64 after native testing |

Native build machines or CI runners are recommended for release builds and
testing. Windows cross-compilation does not require CGO by default. macOS and
Linux cross-compilation use Docker; macOS signing and notarization require a Mac.
Do not claim an AppImage supports every Linux distribution without testing its
runtime libraries and supported system versions.

Sources: [Building](https://v3.wails.io/guides/build/building/),
[Cross-platform builds](https://v3.wails.io/guides/build/cross-platform/),
[Windows](https://v3.wails.io/guides/build/windows/),
[macOS](https://v3.wails.io/guides/build/macos/),
[Linux](https://v3.wails.io/guides/build/linux/).

Group search is built into Postika and requires no separate server or executable.

## Current release gaps

- **Browser prerequisite:** users need Chrome/Chromium. `groups_service.go`
  detects an installed browser and uses a dedicated Postika profile. Include this
  requirement on the download page and verify detection on each supported OS.
- **Ubuntu sandbox:** README documents a development-only AppArmor profile tied
  to this checkout's path. Installed packages need verification under Ubuntu's
  user-namespace policy and an appropriate installed-path solution if necessary.
- **Signing deferred:** Windows and Linux installers are intentionally unsigned
  for this release. Package-signing tasks are optional and are not invoked by
  the default release packaging commands.
- **Release automation:** this workspace has no Git repository or release CI.
  Set up version control and native build jobs before relying on automated builds.
- **Updates:** the app has not configured `app.Updater`. Runtime update metadata,
  the update public key and update behavior remain to be
  implemented. Installation/restart should wait until posting has stopped.
- **Single instance:** enable Wails single-instance handling to avoid two app
  processes competing for the same Chrome profile and starting duplicate runs.

## Development and validation

Use the same Wails version in the CLI and `go.mod`; do not install `@latest` in
release CI. Node 24 LTS and Go 1.25+ match the existing project requirements.
Install frontend dependencies with `npm ci --prefix frontend` on clean runners.

```sh
wails3 doctor
npm --prefix frontend run check
npm --prefix frontend run build
go test -buildvcs=false -timeout 60s ./...
go test -buildvcs=false -run TestGroupSearchAndBulkSelectionUI -count=1 -timeout 75s -args -browser-integration
```

The browser integration test uses local fixtures; it does not publish Facebook
posts. Validate each installed application on a clean native system, including
browser detection/login, saved groups, group search, image ordering, rich text,
settings, response copying and posting controls. Keep runtime credentials and
browser profiles outside release artifacts.

Sources: [Development workflow](https://v3.wails.io/guides/dev/workflow/),
[Testing](https://v3.wails.io/guides/testing/),
[Build customization](https://v3.wails.io/guides/build/customization/).

## Packaging commands

Run from the project root with the pinned `wails3` executable on PATH. On this
checkout it is `.tools/bin/wails3`. Signing commands require configured identities.
These are build recipes, not proof of native release testing.

### macOS (deferred; reference only)

```sh
wails3 task darwin:package:universal
wails3 tool sign --input bin/postika.app --notarize
wails3 task darwin:create:dmg
```

The direct sign command preserves the universal bundle. Avoid running a task
that rebuilds/repackages it after signing. Verify Developer ID signing,
notarization/stapling and Gatekeeper acceptance on a clean Mac. If additional
executables are bundled, sign them before the containing app.
`darwin:create:dmg` uses our branded background and DMG icon.

### Windows

```sh
wails3 package GOOS=windows GOARCH=amd64 INSTALL_SCOPE=user
```

The installer is unsigned for this release. Its output is
`bin/postika-amd64-installer.exe`. Do not invoke `windows:sign` or
`windows:sign:installer` for this release. Native installation, shortcuts,
uninstall and WebView2 bootstrapper behavior still need testing. Prefer per-user
installation for the first desktop release; Microsoft Store/MSIX can be added later.

### Linux

```sh
wails3 task linux:create:deb ARCH=amd64
wails3 task linux:create:appimage ARCH=amd64
# After Fedora validation:
wails3 task linux:create:rpm ARCH=amd64
```

GTK4 and WebKitGTK 6.0 are the current stack. Establish supported distributions
and library versions through installation tests. Older Linux support requires
separate GTK3 builds and matching package dependencies; it is not implied by
producing an AppImage. Review bundled dependency locations and licenses.

[Code signing guide](https://v3.wails.io/guides/build/signing/)

## Website downloads and updates

Use immutable versioned download paths and a separate update channel document:

```text
/downloads/postika/0.1.0/postika-0.1.0-windows-amd64-installer.exe
/downloads/postika/0.1.0/postika-0.1.0-linux-amd64.deb
/downloads/postika/0.1.0/postika-0.1.0-linux-amd64.AppImage
/downloads/postika/0.1.0/SHA256SUMS
/updates/postika/stable.json
```

These are proposed public names; copy/rename verified build outputs during
release assembly. Serve downloads over HTTPS with release notes and system
requirements. Build artifacts first, validate them, upload them, then update the
download page and channel manifest.

For future in-app updates, use the Wails `endpoint` provider pointed at our
website. Publish a signed Wails Update Manifest and embed its public key in the
app. Keep the signing private key in the release secret store.

Update payloads differ from installers: macOS needs a ZIP of the signed `.app`;
Windows needs the actual app binary or its ZIP, not the NSIS installer; archives
must have one top-level entry. Linux system packages should normally update via
their package manager, rather than replacing a root-owned installed binary.
Select the update strategy after resolving companion dependencies. Do not feed
the updater DMG/MSI installers or automatically restart during a posting run.

The CLI supports `wails3 updater genkey`, `manifest` and `verify`. Generate the
manifest only from verified update payloads, with the actual HTTPS website URL
and release version. Do not include installer files in that payload directory.

Sources: [Updater](https://v3.wails.io/guides/updater/),
[Update manifest](https://v3.wails.io/reference/update-manifest/).

File associations and custom URL protocols are optional product features;
Postika currently has no document format or deep-link requirement. Single
instance handling is relevant now; claiming common image extensions is not
necessary for normal file selection or drag and drop.
[Single instance](https://v3.wails.io/guides/single-instance/),
[File associations](https://v3.wails.io/guides/file-associations/),
[Custom protocols](https://v3.wails.io/guides/distribution/custom-protocols/).

## Recommended next work

1. Add single-instance handling and test installed application paths.
2. Produce and test unsigned Windows and Linux beta installers.
3. Build release automation and website downloads with checksums.
4. Add verified in-app updates after the first installation path is reliable.

Current machine: Ubuntu 24.04 amd64; GTK4/WebKitGTK and development tools pass
Wails Doctor. Docker is absent; NSIS is installed locally in `.tools/nsis`, with
a wrapper at `.tools/bin/makensis`. No signing credentials are configured.
macOS/Windows installation and release signing have not been validated here.

## Local build artifacts (2026-10-11)

Unsigned version `0.1.0` builds for amd64 are available in `bin/releases/0.1.0`:

- `postika-0.1.0-windows-amd64-installer.exe` (per-user NSIS with WebView2 bootstrapper)
- `postika-0.1.0-linux-amd64.deb`
- `postika-0.1.0-linux-amd64.AppImage`
- `SHA256SUMS` and `LICENSE.txt`

Windows resources were checked for release metadata and an empty signing
certificate directory. The DEB payload and metadata were inspected. The AppImage
was extracted to verify the compiled code, build ID, desktop metadata and license.
SHA-256 checksums were generated and verified. Native installation, posting and
uninstallation tests remain; these files have not been published.

### Ubuntu launcher correction

The DEB now includes `/etc/apparmor.d/postika`, allowing user namespaces for
the installed Postika executable and its WebKit sandbox. Its installation hook
loads the profile on systems with Ubuntu's AppArmor user namespace restriction;
the removal hook unloads it. The global restriction and WebKit sandbox stay
enabled. This follows [Ubuntu's application profile guidance](https://documentation.ubuntu.com/release-notes/24.04/).

The original DEB crashed before displaying a window on this Ubuntu machine:
`bwrap: setting up uid map: Permission denied`, followed by a WebKit SIGTRAP.
Kernel logs confirmed the `unprivileged_userns` AppArmor denial. The corrected
DEB was rebuilt and its AppArmor syntax, hooks and package contents validated.
Installing it requires local sudo authentication; the user confirmed that
the installed DEB now launches successfully. The portable AppImage has no installation
hook and may still need a separately configured profile on affected systems.

### Linux dock identity

The Linux GTK application ID and program name are explicitly `app.postika`.
The launcher is installed as `app.postika.desktop`, with
`StartupWMClass=app.postika` and `Icon=postika`. This lets GNOME associate
running windows with the installed launcher and its icon, including on Wayland.
The package also refreshes the hicolor icon cache during installation.
The updated binary and launcher were rebuilt into the Linux packages; the dock
appearance requires verification after reinstalling and restarting Postika.

### Unconfirmed posting correction (2026-10-12)

The source now retries unconfirmed replay confirmations and transport failures
up to three times using identical request bytes and mutation IDs, then records
the outcome and proceeds to the next group. The first captured browser request
also gets only three additional attempts. Previously unconfirmed ledger entries
are skipped without blocking other selected groups. Account restrictions,
expired sessions and explicit cancellation still stop the run. Retrying after a
lost response can duplicate a post if Facebook already accepted it.

The Go suite passed with `CGO_ENABLED=0 go test -tags server ./...`.
Linux desktop rebuilding is blocked in the current execution environment by
missing GTK4/WebKitGTK development libraries. Existing release artifacts do
not contain this correction yet; rebuild and reinstall before verification.

Use `PATH="$PWD/.tools/bin:$PATH" .tools/bin/wails3 task linux:create:deb`
from the project root to compile the application and create `bin/postika.deb`.
The `linux:generate:deb` task now also requires the build, so invoking it directly
cannot just wrap an old executable. The lower-level `wails3 tool package` command
only packages its existing inputs and must follow a successful compilation.
Install the resulting DEB, then completely quit and relaunch Postika; rebuilding
or reinstalling does not replace the code of an already running process.
