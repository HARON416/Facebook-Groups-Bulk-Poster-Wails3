# Distributing Postika

Reviewed against Wails v3 documentation on 2026-10-23 and the project's pinned
Wails `v3.0.0-beta.34` CLI and Go dependency. Downloads will be hosted on our own
website. This is a release plan; no production installers have been published.

## Confirmed release metadata

| Field | Value |
| --- | --- |
| Version | `0.1.0` (Windows numeric resource version `0.1.0.0`) |
| Publisher | Postika Technologies |
| Website | https://postika.softwrhq.com |
| Support | haronkibetrutoh@gmail.com |
| Distribution license | MIT; see the root `LICENSE` |
| Target platforms | macOS (Apple Silicon/Intel), Linux x64, Windows x64 |
| Package signing | Unsigned for this release |

The metadata is applied to `build/config.yml`, Windows resources and NSIS/MSIX
assets, Linux nFPM packages, and maintained desktop entries. NSIS installs
`LICENSE.txt`; Linux packages install `/usr/share/doc/postika/copyright`.
The license is also bundled in the application's frontend assets (including
AppImage builds) at `licenses/postika.txt`. Keep that copy in sync with `LICENSE`.
Linux `.desktop` files use `Version=1.0` for the desktop-entry specification;
`X-AppImage-Version=0.1.0` records the application version.
`make package-all` creates the current platform’s unsigned distribution formats:
Windows per-user NSIS/portable ZIP, macOS app/DMG/updater ZIP, or Linux
AppImage/DEB/RPM/Arch. MSIX remains separate because normal distribution requires
a trusted signature. See [the guide review](WAILS_V3_REVIEW.md).

## First release formats

| Platform | Website download | Build environment | Initial target |
| --- | --- | --- | --- |
| macOS | Separate Apple Silicon and Intel DMGs | macOS with Xcode tools | arm64 and amd64; native testing required |
| Windows | Per-user NSIS installer and portable ZIP | Windows with NSIS; Go for ZIP assembly | amd64 |
| Linux | AppImage, DEB, RPM and Arch | Linux with GTK4/WebKitGTK development libraries | amd64 |

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
- **Release automation:** `.github/workflows/distribution.yml` defines native
  package jobs for all four targets. Its first remote build and native install
  tests still need to run before relying on the generated packages.
- **Updates:** the app checks signed website feeds ten seconds after launch and
  every minute while open, automatically downloads verified payloads, and offers
  Restart to update. All supported formats have an installation path; native
  end-to-end tests remain a release requirement. No format uses website-only
  manual updates. DEB/RPM/Arch packages include the root-owned verification helper and
  depend on PolicyKit for authorization.
- **macOS/Homebrew:** DMG tasks and a cask template exist, but DMGs need native
  builds/testing and Homebrew needs a published tap/cask. Neither is live yet.
- **Single instance:** Wails single-instance handling is enabled to prevent
  competing posting processes. Verify this on native Windows and Linux builds.

## Development and validation

Use the same Wails version in the CLI and `go.mod`; do not install `@latest` in
release CI. Node 24 LTS and Go 1.26.2+ match the existing project requirements.
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

Optional obfuscation uses `make install-garble` (pinned Garble v0.16.0), then
`wails3 build --obfuscated` with `.tools/bin` first on PATH. Keep the generated
`wails_obfuscated.gen.go` in source control; it records stable method IDs for
normal and obfuscated builds. All JSON-facing structs need explicit JSON tags. Go 1.26 rejects Garble’s
linker overlay when the auto-downloaded toolchain lives beneath GOMODCACHE;
for obfuscation, use a regular Go 1.26.2 installation outside the module cache
and put its bin directory on PATH (with GOTOOLCHAIN=local).

## Packaging commands

### Versioning

`build/config.yml` is the authoritative application version. `make release-all`
automatically increments its patch version, synchronizes the native metadata,
commits only those version files, and pushes the commit to the current branch
on `origin` before starting CI. All four jobs build that version commit.

| Release command | Behavior from `0.1.0` |
| --- | --- |
| `make release-all` | Prepare and build `0.1.1` |
| `make release-all BUMP=minor` | Prepare and build `0.2.0` |
| `make release-all BUMP=major` | Prepare and build `1.0.0` |
| `make release-all VERSION=0.3.2` | Prepare and build explicit higher version `0.3.2` |
| `make release-all BUMP=none` | Build the existing `0.1.0` without a version commit or push |

An attempted release reserves its version when the version commit is pushed,
even if packaging later fails. Retry failed builds with `BUMP=none` rather than
reserving another version. A commit failure restores the previous metadata;
a push failure keeps the local version commit and reports how to push/retry.
Branch protection is honored; the command never force-pushes or bypasses it.
Every release still requires clean, already-pushed application source. Plain
`make build`, `make package-all`, and GitHub's manual workflow-dispatch UI build
the existing metadata without incrementing it. Automatic version preparation
lives in the `make release-all` launcher.

The following helpers are optional for preparing a version manually:

| Command | Behavior from `0.1.0` |
| --- | --- |
| `make version-bump` | Patch: `0.1.1` |
| `make version-bump BUMP=minor` | Minor: `0.2.0` |
| `make version-bump BUMP=major` | Major: `1.0.0` |
| `make version-set VERSION=0.3.2` | Explicit higher version: `0.3.2` |
| `make version-check` | Validate every platform's active version fields |
| `make version-sync` | Repair platform metadata from the config without a bump |

Choose one manual bump/set command, review the diff, commit and push, then run
`make release-all BUMP=none` to avoid a second increment. Both automatic and
manual preparation update config, Windows executable resources, NSIS
defines, both MSIX templates, both macOS plists, Linux nFPM metadata, and both
desktop-entry AppImage versions. Windows numeric versions use `X.Y.Z.0`.
The shared runtime/updater version in `build/windows/info.json` is checked on
every platform. Release dispatch, per-target packaging, CI, and the shared Wails
frontend-build task all check consistency; an inconsistent version cannot be
packaged through these release commands.

Only stable `MAJOR.MINOR.PATCH` values without leading zeros are accepted by this
cross-platform release workflow; each component must fit Windows' 0–65535 range.
Explicit equal or lower versions are rejected. For metadata repair, use sync.
If a version field is missing or duplicated, the bump fails before writing any
files. Existing packages, public updater feeds, Git tags, and release-history
examples are not rewritten. The frontend npm package's internal tooling version
is separate from the shipped desktop version. Desktop-entry `Version=1.0` and
OS minimum-version fields are specifications/requirements and remain unchanged.

### One-command build for all desktop targets

```sh
make release-all
```

One-time setup: install and authenticate GitHub CLI with `gh auth login`, enable
Actions, and merge `.github/workflows/distribution.yml` into the default branch.
On Ubuntu/Debian, the local CLI setup is:

```sh
sudo apt-get install gh
gh auth login
```

GitHub CLI starts and downloads the remote workflow; it is not part of the
Wails compiler or packager. `make package-all` remains the local, current-OS
packaging command and does not require GitHub CLI or a GitHub account.

Commit and push the application changes you want to distribute. The launcher
rejects a dirty checkout and a remote branch whose commit differs from HEAD.
It then commits and pushes only its synchronized version metadata by default.
It does not commit unrelated application changes, publish a GitHub release,
upload to the website, or install an application.

The workflow builds these targets independently on native GitHub-hosted runners:

| Target | Packaged artifacts |
| --- | --- |
| Linux amd64 | AppImage, DEB, RPM, Arch package, raw executable |
| Windows amd64 | Per-user NSIS EXE, portable ZIP, raw updater EXE |
| macOS arm64 | DMG and app-bundle ZIP |
| macOS amd64 | DMG and app-bundle ZIP |

Each job installs the Go version from `go.mod`, Node 24, and the Wails CLI version
matching the Go dependency, then checks the frontend and runs Go and release
script tests before packaging. It stages versioned filenames with `LICENSE.txt`
and `SHA256SUMS`. The app bundle is retained locally by the staging script;
CI transports it inside its app ZIP to preserve file modes and symlinks.
The package jobs use the version committed by the launcher. For a retry with
`BUMP=none`, they use the existing synchronized metadata instead.

If the jobs succeeded but downloading failed, resume that run instead of
rebuilding: `make release-resume RUN_ID=<successful-run-id> PUBLISH=1`.
The download retries up to three times in fresh temporary directories. Partial
extracted files from one attempt cannot block another. Resuming does not bump,
commit, push, or dispatch a workflow. It validates the selected workflow's
success, source commit, and release version before downloading. If the checkout
has advanced, explicitly acknowledge the selected build's exact source with
`SOURCE_COMMIT=<full-built-commit-sha>`; packages from that run exclude later
commits. Build a fresh release to include those later changes.

The launcher watches the particular run it requested, checks that it builds
the intended commit, and downloads only after all jobs succeed. Downloads are
temporarily stored under `bin/releases/ci-<run-id>-<unique-suffix>/`. The command verifies all
four target manifests and payloads, then automatically flattens the packages
into `bin/releases/<version>/`. The result has 12 package files, one `LICENSE.txt`,
and a combined `SHA256SUMS`, ready to upload to a versioned hosting folder.
Identical licenses with Windows versus Unix line endings are merged into one
LF copy, and its checksum is recalculated. Package bytes remain unchanged.
The nested download folder is removed only after the flat release is complete.
The command then rechecks all twelve packages and the license against the
combined manifest and removes older local version folders and obsolete CI
caches containing only known packages of the retained or earlier versions.
The normal result is one local `bin/releases/<latest-version>/` folder. A failed
or corrupt new download preserves the previous complete release. Newer local
releases are never discarded for an older build; newer downloads and unrelated
folders are preserved. This local retention policy does not delete hosted Bunny
files, GitHub artifacts, or remote release history.
Corrupt/incomplete downloads or conflicting licenses preserve the download
folder and fail the command. Existing releases are reused only when every file
and checksum matches; different files for the same version are never overwritten.
Move an existing release or bump the synchronized version metadata before
deliberately creating a different build of that version.

Access to the intermediate GitHub artifacts follows
the repository's Actions access rules; no public release is created. CI artifacts
expire after 30 days; copy verified releases to your permanent hosting.

This covers the current unsigned distribution set. MSIX and universal Mac
bundles are separate optional targets. Native install/login/update testing
and Homebrew tap publication remain separate release steps. Website metadata is
generated automatically. `make release-all PUBLISH=1` also signs and publishes
feeds/packages to Bunny; see the publishing section below. No publisher keys are required by this
workflow; macOS retains its existing ad-hoc signature.

The application build and package tasks follow Wails v3's Taskfile pipeline:
NSIS on Windows, native app bundles and Wails DMG tooling on macOS, and Wails
AppImage/nFPM tooling on Linux. Our `package:all` tasks, portable/updater ZIP
helpers, CI orchestration, staging, and checksums are project additions around
that pipeline, not an alternative application builder. Publisher signing and
notarization are not invoked. macOS's existing ad-hoc bundle signature remains;
it is not a Developer ID signature or notarization.

References: [Wails build pipeline](https://v3.wails.io/guides/build/building/),
[Windows packaging](https://v3.wails.io/guides/build/windows/),
[macOS packaging](https://v3.wails.io/guides/build/macos/), and
[Linux packaging](https://v3.wails.io/guides/build/linux/).

Preview without dispatching: `node scripts/release-all.mjs --plan`.
For a single target on its native host, run, for example,
`node scripts/package-release.mjs linux-amd64`. Existing staged version/target
directories are preserved; move one before deliberately rebuilding it.

### Individual packaging commands

Run from the project root with the pinned `wails3` executable on PATH. On this
checkout it is `.tools/bin/wails3`. Signing commands require configured identities.
These are build recipes, not proof of native release testing.

### macOS (Apple Silicon and Intel)

Build each DMG separately on macOS; copy the output before building the other
architecture because the bundle/output names are reused:

```sh
wails3 task darwin:package:all ARCH=arm64
# Copy/rename the resulting DMG to postika-<version>-darwin-arm64.dmg.
wails3 task darwin:package:all ARCH=amd64
# Copy/rename the resulting DMG to postika-<version>-darwin-amd64.dmg.
```

The existing native bundle task applies local ad-hoc signing, with no paid
Developer ID certificate or notarization. Verify launching/installing on both
Mac architectures before publication and document any Gatekeeper prompts.
`darwin:create:dmg` uses our branded background and DMG icon.

The Homebrew release template is `build/homebrew/postika.rb.tmpl`. Replace its
version and architecture-specific SHA-256 placeholders using the final DMGs,
then publish it as `Casks/postika.rb` in a public Homebrew tap and validate both
architectures. The cask declares `auto_updates true`; its installed app uses the same signed
in-app bundle updates as a DMG installation. A tap install command must include
the real published tap name.
Do not advertise `brew install --cask postika` until that command actually works.

### Windows

```sh
wails3 task windows:package:all ARCH=amd64
```

The installer is unsigned for this release. Its output is
`bin/postika-amd64-installer.exe`. Do not invoke `windows:sign` or
`windows:sign:installer` for this release. Native installation, shortcuts,
uninstall and WebView2 bootstrapper behavior still need testing. Prefer per-user
installation for the first desktop release; Microsoft Store/MSIX can be added later.

The portable output is `bin/postika-windows-amd64-portable.zip`, containing
`postika.exe` and `LICENSE.txt`. Users unzip and run it; Chrome/Chromium and
WebView2 must already be installed. Account data still uses the normal OS
configuration directory; portable refers to installation, not portable logins.

### Linux

```sh
wails3 task linux:package ARCH=amd64
# Or individual formats:
wails3 task linux:create:deb ARCH=amd64
wails3 task linux:create:appimage ARCH=amd64
wails3 task linux:create:rpm ARCH=amd64
wails3 task linux:create:aur ARCH=amd64
```

GTK4 and WebKitGTK 6.0 are the current stack. Establish supported distributions
and library versions through installation tests. This project uses the current GTK4 stack only. Producing an AppImage does not
imply compatibility with older distributions. Review bundled dependency locations and licenses.

[Code signing guide](https://v3.wails.io/guides/build/signing/)

## Automatic updates on every supported distribution

The release requirement is **automatic detection and download on every supported
platform, followed by installation through Restart to update**. There is no
manual-download exception for DEB, RPM, Arch, AppImage, DMG, Homebrew, NSIS or portable ZIP.
A browser is used only for first installation or recovery. The website download
page must describe these same update capabilities for each format.

Postika checks ten seconds after launch and every minute while open. A new
release downloads in the background, is verified, and enables the orange
**Restart to update** button. Installation requires an explicit restart request;
active, paused, stopping and scheduled posting runs across all accounts block
it. Draft writes finish first. Downloads can proceed while posting continues.
Failures do not discard the existing running app or trigger a browser fallback.

| Initial distribution | Automatic update payload and installation |
| --- | --- |
| Windows x64 per-user NSIS installer | Signed raw `postika.exe`; Wails replaces/relaunches the existing executable |
| Windows x64 portable ZIP | Same signed executable; original portable location is retained |
| macOS Apple Silicon DMG | ZIP containing one complete `postika.app`; Wails swaps/relaunches the bundle |
| macOS Intel DMG | Matching amd64 app-bundle ZIP, using the same update flow |
| macOS Homebrew cask | Installed app uses the same bundle updater; cask declares `auto_updates true` |
| Linux x64 AppImage | Signed full AppImage; Wails v3 stages beside and replaces the outer file after exit |
| Linux x64 DEB | Signed DEB; package-installed helper authenticates and installs it through apt, then relaunches |
| Linux x64 RPM | Signed RPM; same helper uses DNF upgrade, preserving distro policy |
| Linux x64 Arch | Signed package archive; same helper uses pacman, preserving distro policy |

Windows releases target per-user installations. Portable/AppImage folders and
macOS app installation locations must be writable. AppImage updates target the
outer file from `APPIMAGE`, not the temporary mounted ELF. An executable backup
is retained beside an updated AppImage for recovery. macOS users must copy the
app out of the read-only DMG before running it. Native bundle tasks use ad-hoc
signing; no paid Windows/macOS publisher certificate is configured. Validate
Gatekeeper behavior with the resulting bundle on a clean Mac.

DEB/RPM/Arch installation may display the OS administrator prompt. Rejecting it leaves
Postika open and the verified update available to retry. `/usr/lib/postika/update-admin`
is installed by each system package as root-owned, mode 0755; it copies the downloaded package
into a root-private directory, verifies its embedded-key Ed25519ph signature,
checks package name, architecture and version, rejects downgrades, and invokes
apt, DNF, or pacman. Packages are staged in the user cache because Wails’
standard staging directory beside the installed executable is root-owned.
This preserves package metadata, dependencies, desktop integration and
package hooks. It does not run the GUI as root or replace a package-owned binary
behind the package manager's back. The DEB depends on `pkexec`; RPM/Arch depend on `polkit`. Desktop sessions
also need a working PolicyKit authentication agent.
Package references: [Ubuntu](https://packages.ubuntu.com/noble/amd64/pkexec),
[Debian](https://packages.debian.org/trixie/pkexec).

RPM and Arch installation honors the system’s package-signature policy. A
managed system may reject an unsigned package; the updater reports that failure
and keeps the current application. Do not disable that policy globally to make
an unsigned release install. Test the intended supported distro configuration.

### Signing and payloads

Every update needs the SHA-512 digest and Ed25519ph signature generated by the
pinned Wails updater CLI. Unsigned/digest-only releases, corrupt downloads,
incorrect keys, wrong payload types and non-HTTPS URLs are rejected. This is
update authentication, independent of paid OS publisher code signing.

The shared embedded public key is `internal/updateauth/public-key.pem`. The
corresponding private key is in ignored `.update-keys/updater.key`. Back it up
in a password manager or CI secret store before distributing an updater-enabled
build. Keep the same key for subsequent releases; key rotation needs a deliberate
migration for both the desktop app and the installed package verification helper.
The runtime version comes from `build/windows/info.json` (`ProductVersion`) on
all platforms and must match `build/config.yml` and the release metadata.

Use separate static feeds so Linux package types cannot be confused:

| Feed at `https://postika.softwrhq.com/updates/postika/` | Artifacts |
| --- | --- |
| `stable.json` | Windows raw EXE, macOS app-bundle ZIPs for arm64/amd64, Linux AppImage |
| `stable-deb.json` | Linux DEB for each supported architecture |
| `stable-rpm.json` | Linux RPM for each supported architecture |
| `stable-arch.json` | Linux Arch package for each supported architecture |
| `stable-binary.json` | Optional standalone Linux ELF for developer/raw-binary installations |

Use `darwin` for macOS, `linux` for Linux and `windows` for Windows. Use `arm64`
and `amd64` for architectures. Initial installers (DMG/NSIS/portable ZIP) and
update payloads are separate artifacts. Never put an NSIS installer into the
Windows update feed or a DMG into the macOS update feed.

After `make release-all` downloads and verifies the complete flat release, run:

```sh
make update-feeds
```

This uses the project-local Wails CLI and existing `.update-keys/updater.key` to
create all five feeds in `bin/updates/<version>/`. It verifies the release's
SHA256SUMS, selects compatible payloads for each feed, then verifies every digest
and signature against the public key embedded in the app. Signing runs locally;
the private key is never uploaded. Existing feeds are reused after verification;
incorrect signatures, versions or URLs stop the command. This does not bump the
version or rebuild the app.

### Automated Bunny publishing

Configure ignored `.env` from `.env.example` with your Storage Zone
name, regional Storage API hostname and **Storage Zone password**. A global
Bunny API key is not used. The regional hostname is shown in FTP & API Access;
do not substitute the public CDN hostname. Environment variables take precedence
over values in the file. See [Bunny's Storage HTTP API](https://bunny.net/docs/storage/http)
for authentication, regional endpoints, raw-body PUT uploads, and uppercase
SHA-256 Checksum headers.

```sh
make release-all PUBLISH=1   # Build the next version and publish it
make publish-release        # Publish/retry the current downloaded version only
```

The first command checks credentials and key availability before starting the
normal version bump/build. Default patch versioning means `0.1.3` becomes `0.1.4`.
For native testing before publishing, use plain `make release-all`, test the
packages, then `make publish-release`. The publisher verifies local packages and
signatures, checks existing feeds to prevent a rollback or same-version payload
replacement, and uploads every package to `downloads/<version>/`. Upload paths
create the versioned storage folder. Every public package URL is fetched and
SHA-256 verified before any stable feed is changed, so publishing transfers the
release both up to storage and back down through the CDN. `SHA256SUMS` is uploaded
after the package files, followed by a `release.json` metadata copy.

The publisher then overwrites the five `updates/stable*.json` files and verifies
their public contents and JSON/no-cache headers. Remote older packages are never
deleted. It uses a local lock; run only one publisher across all machines. A
partial feed upload can temporarily leave platforms on different releases;
`make publish-release` resumes the same version without another bump. Different
same-version payloads require a new version. If a process crashed, confirm it has
stopped before removing `bin/.publish.lock`.

For example, a `0.1.4` Windows installer is hosted at:

```text
https://postika-desktop-application.b-cdn.net/downloads/0.1.4/postika-0.1.4-windows-amd64-installer.exe
```

The main feed URL remains:

```text
https://postika-desktop-application.b-cdn.net/updates/stable.json
```

The other four feeds use the same `updates/` directory. Keep legacy `0.1.2`
packages at their already-published flat `/downloads/` URLs. Do not move them
when introducing versioned folders for new releases.

Keep Bunny's `/updates/*.json` cache bypass rule enabled, with `application/json`
and `Cache-Control: no-cache` or `no-store`. The publisher requires those response
headers. Purge stale feeds when first configuring the rule. With cache bypass
active, normal publication does not require cache-purge API credentials. Query
parameters for platform/arch/version/channel must be accepted.

Already-built apps request `https://postika.softwrhq.com/updates/postika/`.
The website's five exact GET/HEAD routes temporarily redirect those requests
over HTTPS to Bunny's `updates/` folder, preserving the query string. Those
routes do not change for each release. Check the public URLs and perform native
old-to-new update tests before advertising working auto updates. An app already
on the published version correctly has no new update to install.

### Generated website metadata

After all four build targets download and verify, `make release-all` generates
`website/releases/<version>.json` and `website/releases/current.json` with all
eight public download URLs. The website always embeds `current.json`; there is
no Go source-code path or download URL to edit for each version. To regenerate
metadata for the current complete local release, use `make website-release`.
Release dates and notes in an existing version JSON are preserved; add actual
release notes there and regenerate before rebuilding the website. Commit the
generated metadata with your next app changes before another all-OS release.

Rebuild and deploy the website after successful package/feed publication. This
workflow does not upload or restart the VPS binary. The running binary keeps
its embedded metadata until replaced; the `release.json` copy on Bunny is not
a live metadata source. `RELEASE_FILE` remains an optional external-file startup
override. Plain `make run` from `website/` needs no environment variables.

On macOS, the release packaging task creates `.app.zip` with one top-level bundle,
preserving executable modes and symlinks via `build/darwin/updatezip`. Do not
substitute a DMG or a ZIP containing `__MACOSX` folders in the update feed.

### Release gates

Automated tests validate signed-payload rejection/acceptance, automatic download,
shutdown cancellation, queue exclusion, built-in Wails AppImage staging, and native
package metadata checks. They do not prove native release readiness. Before advertising
all-platform automatic updates, run real old-to-new tests on Windows NSIS and
portable ZIP, both macOS architectures plus Homebrew, AppImage, and installed
DEB/RPM/Arch. Verify cancelled authorization, package-manager failure, read-only paths,
offline/tampered downloads, draft preservation and posting queue exclusion.
Do not publish a platform until its complete installation/update path passes.

## File associations and single instance

Postika currently has no document format or deep-link requirement. Single
instance handling is relevant now; claiming common image extensions is not
necessary for normal file selection or drag and drop.
[Single instance](https://v3.wails.io/guides/single-instance/),
[File associations](https://v3.wails.io/guides/file-associations/),
[Custom protocols](https://v3.wails.io/guides/distribution/custom-protocols/).

## Recommended next work

1. Test single-instance handling and installed application paths on native builds.
2. Produce and test unsigned Windows and Linux beta installers.
3. Build release automation and website downloads with checksums.
4. Validate real automatic old-to-new updates for every supported distribution.

Current machine (checked 2026-10-23): Ubuntu 26.04 amd64; Wails beta.34 Doctor
reports no issues with Go 1.26.2, GTK4 and WebKitGTK 6.0 development dependencies.
Docker is absent; NSIS is installed locally in `.tools/nsis`, with
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
