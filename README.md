# Postika

Postika is a desktop app for composing posts and sharing them across Facebook
groups with separate accounts, saved drafts, scheduling, and posting controls.
It uses an installed Chrome or Chromium browser for Facebook sessions and is
built with **Wails v3, Go, Svelte 5, TypeScript, and Lexical**.

The project is under active development. The website and update routes are
deployed at `https://postika.softwrhq.com/`. Windows, macOS, and Linux release
assets and signed update feeds are hosted on Bunny CDN at
`https://postika-desktop-application.b-cdn.net`.

## Features

- **Compose posts:** bold, italic, headings, quotes, lists, and Markdown
  shortcuts. Attach JPG, PNG, WebP, or GIF images, preview them, and reorder them
  by dragging or with Alt + Left / Right.
- **Import Facebook page posts:** enter a page URL and the number of posts to
  fetch using the number field with up/down arrows and a “posts” suffix.
  Review the results, edit text, remove images, or delete posts before
  adding them to your posting queue. Fetching depends on what the signed-in
  Facebook account can access.
- **Manage accounts:** use the default account or choose **Add account** in the
  top bar to open a popup, enter a name, and choose **Create & sign in**. Each
  account has its own Facebook browser session, groups, drafts, posting
  preferences, and queue. Switch accounts while other queues run.
- **Choose groups:** load your groups, search them, and select individual groups
  or all matching results. Loaded groups are saved between sessions.
- **Control posting:** post now or schedule a queue, configure minimum and
  maximum delays, and pause, resume, or stop from the shared Posting logs window.
  Queues wait while offline and resume when connectivity returns.
- **Personalize the interface:** 33 languages, including right-to-left layouts,
  and 213 themes. Language and theme choices sync across windows. App-generated
  log messages are translated, with original diagnostics under Debug details.

On first launch, Postika offers a guided tour with spotlights on the real app
controls. Choose **Start guided tour** or **Skip**; the choice is remembered on
this device. Reopen it with **Guide** in the top menu or **How to use → Start guided tour**, or choose
**Read the guide** for the existing written instructions. The tour has Previous,
Next, Close and Finish controls, supports Escape and keyboard navigation, and
preserves the account's draft, workflow step and scroll position. It does not
fetch data, create accounts or start posting. Start it when no posting or group
loading operation is active. Tour copy follows the selected interface language.
Marketplace has separate tours for saved listings, choosing a listing type, and
the item, vehicle, or home editor. Select **Guide** in the top menu of the
current Marketplace window to replay its tour. The groups tour also introduces
the Marketplace button in the header.

## Using the app

Start with the default account or select an existing account from the top bar.
Adding another account is optional: choose **Add account**, enter its name in
the popup, and select **Create & sign in**. Finish Facebook login in the new
account’s Chrome/Chromium window. Cancel, Escape, the close button, or clicking
outside dismisses the popup before creation starts; errors appear inside it.

To rename a profile, open the account selector, hover over its row, and choose
the pen icon. Edit the account name in the popup and select **Save changes**.
You can also highlight a profile with the arrow keys and press **F2**. Renaming
is available in the main window and Posting logs, and keeps the account’s login,
drafts, groups, and running queue intact.

The account selector also has **Add account** below the profile list; the top-bar
button opens the same popup. To delete a profile, hover over its row and choose
the trash icon, then confirm **Delete profile**. This stops its queue, closes its
browser, and permanently removes its saved Facebook login, groups, drafts,
images, preferences, and logs. Existing Facebook posts stay on Facebook. Deleting
the last profile leaves the list empty; choose **Add account** for a fresh login.

The main workspace has three steps:

1. **Create post:** write your message and add at least one image, or fetch
   posts from a Facebook page and review the imported content.
2. **Select groups:** choose **Load groups**, finish Facebook login if prompted,
   then search and select the destinations.
3. **Post or schedule:** set the minimum and maximum interval between posts
   (60–86400 seconds), then choose **Post now** or **Schedule**. Equal values
   give a fixed interval. Posting preferences stay with each account’s draft.

Follow progress in **Posting logs**. Switch its account selector to inspect
another queue without interrupting the current one. Language and theme controls
are in the top bar and apply across windows. Popups use the same theme, fields,
and button styles as the rest of the app.

If Facebook requests login or verification while opening a group for posting,
finish it in Chrome. Postika keeps the queue active, waits for login, and
continues automatically with the selected group and draft. **Stop** cancels the
wait; closing the Facebook tab stops preparation.

Page imports also wait for login and show a prompt in the import popup. If the
session expires while fetching posts or photos, Postika reopens Chrome and
continues the same import after you sign back into the same Facebook account.
Use **Cancel** in the import popup to stop waiting.

Keep Postika open and the computer awake for scheduled or active queues.
**Queues and posting logs are not restored after quitting.**

Results distinguish published, pending approval, failed, skipped, and
unconfirmed submissions. Check an unconfirmed submission on Facebook before
retrying: a lost response can leave the result uncertain, and retrying can
produce duplicates. Facebook changes can require updates to Postika's browser
automation.

## Saved data and privacy

Account metadata, browser sessions, group caches, and settings live under
`postika/` in the OS configuration directory. The default account uses that root;
additional accounts use `profiles/<profile-id>/`.

Drafts are stored per account in the app webview's IndexedDB, including editor
content, selected groups, posting preferences, imported post data, and independent
copies of locally attached image bytes. Imported images retain Facebook CDN
URLs and may need to be fetched again if those URLs expire. Drafts survive
updates when the app's data and webview storage are retained.

Marketplace item listings use **Save draft** to store one draft per account,
including the form fields and photo bytes in a separate IndexedDB database.
Reopening the item editor restores that account's saved draft. Unsaved edits
remain in memory when returning to the listing-type choices in the same window.
Meetup preferences, promotion preference, friend visibility, and up to 20 product
tags are included in the saved draft. This editor currently provides local
drafts and previews; listing publication, paid promotion, and phone photo
transfer are not connected.

## Automatic updates

The updater checks **10 seconds after launch**, then **every minute** while
Postika is open. It downloads and verifies a newer release in the background,
then shows an orange **Restart to update** button. Clicking it installs the
update and relaunches the app, after draft saves finish and all posting queues
are stopped or complete. Scheduled and paused queues also block the restart.

Windows executables, macOS app bundles, and Linux AppImages use Wails v3's
built-in updater. DEB, RPM, and Arch installations use a package-installed
verification helper and the native package manager; they may request
administrator authorization. Updates stay inside the app.

Payloads require SHA-512 checksums and Ed25519ph signatures. These authenticate
updates independently of OS publisher code signing. Signed `0.1.2` feeds have
been generated and verified against the embedded public key. Live website
redirects and all five feeds have been checked, including CDN cache bypass.
**Native old-to-new installation tests remain necessary for release acceptance.**
See [Releasing](docs/RELEASING.md) for artifact formats, signing keys, feeds,
and release acceptance checks.

## Development setup

The Go SDK, project-local Wails CLI, and frontend runtime use
**Wails v3.0.0-beta.34**. Install the CLI version pinned in `go.mod`; keep all
three versions aligned when upgrading.

Requirements:

| Dependency | Requirement |
| --- | --- |
| Go | 1.26.2 or newer |
| Node.js | 24 LTS, with npm |
| Browser | Installed Chrome or Chromium |
| Linux | C compiler, `pkg-config`, GTK4 and WebKitGTK 6.0 development libraries |
| macOS | Xcode Command Line Tools |
| Windows | WebView2 runtime; NSIS on PATH when creating installers |

Linux uses the current GTK4 backend. No legacy GTK3 build is configured. On
Ubuntu 24.04 or newer, install the native development dependencies:

```sh
sudo apt-get update
sudo apt-get install -y build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev
```

From the repository root on Linux or macOS, with Make installed:

```sh
make install-cli
export PATH="$PWD/.tools/bin:$PATH"
npm ci --prefix frontend
make doctor
make dev
```

`make install-cli` installs the pinned CLI in `.tools/bin`. Make adds that
directory to PATH for its tasks; the export also makes it available to direct
`wails3` commands and their child tasks. `make run` is an alias for `make dev`.

`make dev` can run alongside installed Postika, including while posting queues
are active. Development builds run as **Postika Dev** with a separate
single-instance ID and account storage under `postika-dev` in the user's config
directory (`~/.config/postika-dev` on Linux). Sign in separately in dev; installed
accounts, browser sessions, groups, settings, and queues are not copied or shared.
Linux development webviews also use separate XDG data/cache directories; Windows
uses a separate WebView2 data directory. The dev frontend uses its own local
development origin. Updates are disabled in development builds, including manual
checks and installation. Reopening dev focuses its own existing instance.

The `production` Go build tag selects the installed app identity and storage;
normal release tasks already set it. A successful build alone does not replace
or restart an installed app.

On Windows, use PowerShell to install and run the same CLI without Make:

```powershell
$env:GOBIN = Join-Path $PWD '.tools\bin'
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
$env:Path = "$env:GOBIN;$env:Path"
npm ci --prefix frontend
wails3 doctor
wails3 dev
```

Wails handles frontend development and Go rebuilds through `build/config.yml`
and the Taskfiles. For frontend-only work, `npm --prefix frontend run dev`
starts Vite; native service calls require the Wails runtime.

## Validation

Run from the repository root with the pinned CLI on PATH. Generate bindings
first on a clean checkout, then build the frontend before running Go tests
because `main.go` embeds `frontend/dist`:

```sh
wails3 generate bindings -f '-tags server' -ts -i -obfuscated
npm --prefix frontend run check
npm --prefix frontend run i18n:check
npm --prefix frontend run i18n:test
npm --prefix frontend run themes:test
npm --prefix frontend run logs:test
npm --prefix frontend run build
go test -tags server ./...
```

The `server` tag lets backend tests run without a desktop display or GTK
integration. For race checks with a C compiler available, use
`go test -race -tags server ./...`.

Browser integration tests require Chrome/Chromium and the built frontend. They
use local fixtures and do not publish Facebook posts:

```sh
go test -tags server . -args -browser-integration
```

Release readiness also requires native installation, browser login, and
old-to-new update tests for each distributed platform and format.

## Builds and packages

### Local releases from Linux

GitHub can be used only for source control. `make release-local` builds Linux
x64, Windows x64, macOS Apple Silicon, and macOS Intel on this Linux computer.
It does not use GitHub Actions, artifact storage, `gh`, commits, or pushes.
It includes current local source edits. Frontend, tour, release, and Go checks
run before building. Builds run sequentially because platform tasks share the
frontend and `bin` outputs.

Install Docker Engine, NSIS (`makensis`), the Go/Node/Wails toolchain, and the
GTK4/WebKit development dependencies listed by `make doctor`. Also install
[Apple Codesign's `rcodesign`](https://gregoryszorc.com/docs/apple-codesign/stable/apple_codesign_getting_started.html)
on your PATH (or in `.tools/bin`) for portable ad-hoc Mac app signing. With Rust
installed, `cargo install --locked apple-codesign --version 0.29.0` installs the
pinned signer. Prebuilt Linux binaries are also available from that project's
releases.

On Ubuntu, install and start Docker once:

```sh
sudo apt-get install -y docker.io docker-buildx
sudo systemctl enable --now docker
sudo usermod -aG docker "$USER"
```

Log out and back in after adding yourself to the Docker group. `docker info`
must succeed as your normal user before continuing. Then, from the repository
root:

```sh
make setup-cross                    # One-time Docker image setup
make release-local-check            # Check tools without bumping or building
make release-local                  # Bump patch locally and build all targets
```

The Dockerfile uses heredocs and requires BuildKit. `make setup-cross` invokes
`docker buildx build --load` so the resulting `wails-cross` image is available to
the packaging tasks. If setup reports `COPY failed: no source files were specified`
at a `COPY <<...` step, install `docker-buildx` and rerun setup; the legacy Docker
builder cannot parse that syntax.

Choose the next command according to what you need; each new release command
bumps the patch version unless `BUMP=none` is supplied:

```sh
make release-local PUBLISH=1          # New release: build and publish to Bunny
make publish-local                   # Publish the release already built locally
make release-local BUMP=none PUBLISH=1 # Resume a failed build of unchanged source
```

`BUMP=minor`, `BUMP=major`, and `VERSION=X.Y.Z` select the version policy.
Version metadata is updated locally; commit it with your changes when ready.
Failures preserve the selected version and completed, checksum-verified targets.
Retries require unchanged source; edits require a new version or moving the
recorded staging folders aside. Completed releases stay in
`bin/releases/<version>/`. After successful publication, older local releases
and generated staging copies up to that version are removed. The current
release's packages, update feeds, and build record remain for retries. Build-only
runs and failed publications preserve previous packages. Small build records
and feed manifests from older versions may remain. Bunny keeps the latest two
completed versioned releases, as described under publishing below.
Local build records
and staging live in `bin/local-release/<version>/` and
`bin/releases/ci-local-<version>/`. Release and publish locks prevent concurrent
local release commands.

Mac downloads use **`.app.zip` instead of `.dmg`** because the pinned Wails
version's DMG task requires macOS. Extract the ZIP and move Postika.app to
Applications. Both architectures are ad-hoc signed with `rcodesign`; these are
not Apple Developer ID signatures or notarization. Gatekeeper first-launch
approval is still required, as with the current unsigned distribution. Signed
update feeds continue using the existing `.app.zip` payloads and existing
`.update-keys/updater.key`. The website metadata automatically links to the ZIPs.
The existing Homebrew cask template assumes DMGs and must be adapted separately
before using it for a ZIP-based release.

`PUBLISH=1` reads the existing ignored `.env` Bunny settings and checks the
signing key and storage connection before changing the version or building.
Use `make publish-local` to retry uploading a completed release without rebuilding.
Publishing uploads to Bunny and updates local website metadata; deploying the
website remains a separate step. Docker cross-compilation uses the macOS SDK;
review Apple's SDK terms as described in the
[Wails cross-platform guide](https://v3.wails.io/guides/build/cross-platform/).

### Optional releases through GitHub Actions

The all-OS release command handles versioning automatically:

```sh
make release-all PUBLISH=1       # Bump patch, build and publish from GitHub to Bunny
make release-all PUBLISH=1 BUMP=minor   # Bump minor instead
make release-all PUBLISH=1 BUMP=major   # Bump major instead
make release-all PUBLISH=1 VERSION=0.3.2 # Explicit higher version
make release-all PUBLISH=1 BUMP=none    # Rebuild the current version
```

Start with committed, pushed application changes. The launcher checks the
checkout and GitHub authentication, synchronizes the version, commits only the
version metadata, pushes that commit to the current branch on `origin`, and
builds it. A failed CI build keeps the reserved version; use `BUMP=none` to retry.
Without `PUBLISH=1`, this builds packages and generates website metadata without
uploading to Bunny. It does not create a GitHub release.

For manual version preparation, the standalone helpers remain available:

```sh
make version-bump                  # 0.1.0 -> 0.1.1
make version-bump BUMP=minor       # 0.1.0 -> 0.2.0
make version-bump BUMP=major       # 0.1.0 -> 1.0.0
make version-set VERSION=0.2.0     # Explicit higher version
```

Run just one of those manual alternatives. The command synchronizes all Windows,
macOS, and Linux app/package metadata from `build/config.yml`, including the
embedded version used by automatic updates. After a manual bump, use
`make release-local BUMP=none` to build that version. The optional GitHub path
requires committing and pushing first, then `make release-all BUMP=none`.
`make version-check` validates the metadata; `make version-sync` repairs it
from the config without incrementing anything.
Release scripts, Wails frontend build tasks, and CI reject inconsistent versions.
Existing published packages are not renamed or modified by a version bump.

Run these commands with `.tools/bin` on PATH. Build tasks regenerate bindings
and frontend assets automatically.

```sh
wails3 build                              # Current platform executable
wails3 task package:all                   # Current platform distribution formats
wails3 build GOOS=windows GOARCH=amd64     # Windows executable from Linux/macOS
```

`make build` and `make package-all` provide the corresponding shortcuts.
Outputs are written to `bin/`.

For all four release targets with direct publishing, after committing and pushing
your application changes:

```sh
make release-all PUBLISH=1
```

This starts `.github/workflows/distribution.yml` on native Linux, Windows,
Apple Silicon and Intel Mac runners. Its publish job verifies and combines the
packages, signs update feeds and uploads directly to Bunny. The local command
waits and syncs only the small website metadata file. Full local copies are
optional through `make release-resume RUN_ID=<build-id>` without `PUBLISH=1`.
That optional path flattens packages into `bin/releases/<version>/`, verifies
checksums, merges licenses and prunes older complete local releases after success.
Failed local downloads retain the previous complete release. MSIX and universal
Mac bundles remain separate optional targets. No GitHub release is created.

Use `node scripts/release-all.mjs --plan` to see the plan without starting a run.
The complete setup and limitations are in [Releasing](docs/RELEASING.md#one-command-build-for-all-desktop-targets).

| Platform | Packaging task | Outputs |
| --- | --- | --- |
| Windows x64 | `wails3 task windows:package:all ARCH=amd64` | Per-user NSIS installer and portable ZIP |
| macOS Apple Silicon from Linux | `wails3 task darwin:package:cross ARCH=arm64` | Ad-hoc signed app bundle and app ZIP |
| macOS Intel from Linux | `wails3 task darwin:package:cross ARCH=amd64` | Ad-hoc signed app bundle and app ZIP |
| macOS on a Mac / CI | `wails3 task darwin:package:all ARCH=arm64` or `ARCH=amd64` | Ad-hoc signed app bundle, DMG, and updater ZIP |
| Linux x64 | `wails3 task linux:package ARCH=amd64` | AppImage, DEB, RPM, and Arch package |

### Artifact locations for hosting

The local release download set is:

| OS | Downloads to host |
| --- | --- |
| Windows x64 | Installer `.exe` and portable `.zip` |
| macOS Apple Silicon | arm64 `.app.zip` |
| macOS Intel | amd64 `.app.zip` |
| Linux x64 | `.AppImage`, `.deb`, `.rpm`, and Arch `.pkg.tar.zst` |

Optional native/CI releases also include macOS DMGs. The existing Homebrew cask
template uses those DMGs and needs adapting before publishing a ZIP-based cask.
For automatic updates, also host the raw Windows application `.exe` and the JSON
update manifests. The same architecture-specific macOS app ZIPs serve downloads
and automatic updates.
Linux updates use the same packages as the public downloads. Include release
checksums and the license alongside the artifacts.

MSIX, universal macOS builds, and standalone Linux ELF downloads are outside
the initial public release set.

The paths below are relative to the repository root, using the default `bin/`
output directory. They describe build outputs; a file's presence alone does not
mean it is the latest build or has passed release testing.

| Target | Format | Local build output | Hosting use |
| --- | --- | --- | --- |
| Windows x64 | NSIS setup EXE | `bin/postika-amd64-installer.exe` | First-install download |
| Windows x64 | Portable ZIP | `bin/postika-windows-amd64-portable.zip` | First-install download |
| Windows x64 | Raw application EXE | `bin/postika.exe` | In-app update payload for both Windows distributions |
| macOS Apple Silicon | DMG | `bin/postika.dmg` | Optional macOS/CI output; used by the existing Homebrew cask template |
| macOS Apple Silicon | App bundle | `bin/postika.app/` | Input to app ZIP packaging; also DMG packaging on macOS |
| macOS Apple Silicon | App-bundle ZIP | `bin/postika-darwin-arm64.app.zip` | Local release download and in-app update payload |
| macOS Intel | DMG | `bin/postika.dmg` | Optional macOS/CI output; used by the existing Homebrew cask template |
| macOS Intel | App bundle | `bin/postika.app/` | Input to app ZIP packaging; also DMG packaging on macOS |
| macOS Intel | App-bundle ZIP | `bin/postika-darwin-amd64.app.zip` | Local release download and in-app update payload |
| Linux x64 | AppImage | `bin/postika-x86_64.AppImage` | First-install download and in-app update payload |
| Linux x64 | DEB | `bin/postika.deb` | First-install download and in-app update payload |
| Linux x64 | RPM | `bin/postika.rpm` | First-install download and in-app update payload |
| Linux x64 | Arch package | `bin/postika.pkg.tar.zst` | First-install download and in-app update payload |
| Linux x64 | Standalone ELF | `bin/postika` | Optional raw-binary download and its separate update feed |

Additional targets are separate from the unsigned release set:

- **Windows MSIX:** `wails3 task windows:create:msix:package ARCH=amd64`
  produces `bin/postika-amd64.msix`. It is excluded from `package:all` and the
  current public hosting plan because normal distribution requires signing.
- **macOS universal app:** `wails3 task darwin:package:universal` produces
  `bin/postika.app/` containing both architectures. The current download/update
  plan uses separate arm64 and amd64 builds.
- **Homebrew:** `build/homebrew/postika.rb.tmpl` is the release template, not a
  generated installer. Fill its version and DMG checksums, then publish it as
  `Casks/postika.rb` in a tap. It points to the same hosted, architecture-specific
  DMGs listed above.

For distribution, use the verified, versioned files in `bin/releases/<version>/`
created by `make release-local` (or downloaded by the optional `make release-all`
path). The raw build outputs above can be overwritten by the next architecture
or platform build. `make package-all` packages only the
current OS; it is not needed again after a successful all-OS release.

### Publishing a release to Bunny

The main release flow is **local Linux build → Bunny**. The computer builds all
four targets, verifies checksums, signs update feeds with the existing key, and
uploads directly to Bunny. GitHub is only needed if you choose the optional
Actions workflow above.

#### First-time publishing setup

1. Complete the Docker and signer setup in [Local releases from Linux](#local-releases-from-linux).
2. If `.env` does not exist, copy `.env.example` to `.env`. Keep an existing `.env`;
   do not overwrite its credentials. Configure the Frankfurt Storage HTTP API:

   ```dotenv
   BUNNY_STORAGE_ZONE=postika-desktop-application
   BUNNY_STORAGE_HOST=storage.bunnycdn.com
   BUNNY_STORAGE_PASSWORD=your-storage-zone-password
   ```

   Use the Storage Zone password. `de-s3.storage.bunnycdn.com` is the separate S3
   endpoint; this uploader uses the HTTP endpoint without an `https://` prefix.
3. Restore and privately back up the existing `.update-keys/updater.key`. Keep
   `internal/updateauth/public-key.pem` paired with it. Do not generate a
   replacement key for already-distributed apps. `.env`, `.update-keys/`, and
   local build outputs remain ignored by Git.
4. Run `make release-local-check`. It checks build prerequisites without
   changing a version, building packages, or uploading. Publishing commands also
   check the local signing files and Bunny connection. `make configure-publishing`
   is only for GitHub Actions; local publishing does not require it.

#### Which command to use

Run these commands from the **repository root**:

| Situation | Command |
| --- | --- |
| New release: bump patch, build all targets locally, and publish | `make release-local PUBLISH=1` |
| Build all targets locally for testing without uploading | `make release-local` |
| Publish or retry uploading an already-completed local release | `make publish-local` |
| Resume a failed local build with the same version and unchanged source, then publish | `make release-local BUMP=none PUBLISH=1` |
| Start a minor or major release | `make release-local BUMP=minor PUBLISH=1` or `BUMP=major` |
| Select an explicit higher version | `make release-local VERSION=0.3.2 PUBLISH=1` |
| Inspect the local release plan without building | `node scripts/release-local.mjs --plan` |

For a normal release:

```sh
make release-local PUBLISH=1
```

Local edits are included; pushing to GitHub is not required. The command checks
tools and publishing credentials, runs validation, updates version metadata
locally, and builds all four targets. It verifies and combines their packages
in `bin/releases/<version>/`, generates website metadata, signs the five update
feeds, and uploads directly to Bunny. Commit version and website metadata with
your source changes when ready.

A local build stops if its terminal or computer stops. Keep the computer running
and connected. A failed build preserves its version and verified targets; retry
with `BUMP=none` and unchanged source. Source changes are rejected when resuming
so packages from different code cannot be combined. A completed package set does
not need rebuilding to retry an upload: use `make publish-local`.

#### Optional GitHub publishing

To use Actions instead, authenticate with `gh auth login`, commit and push your
changes, then run `make configure-publishing` once. It validates the existing
signing key and saves the Bunny credentials and signing key as repository secrets.
`make release-all PUBLISH=1` builds and publishes from GitHub, then syncs the small
website metadata file locally. No GitHub Release is created.

| GitHub situation | Command |
| --- | --- |
| New release through Actions | `make release-all PUBLISH=1` |
| Retry publication of the last recorded Actions build | `make publish-release` |
| Publish a specific successful build | `make publish-release RUN_ID=<distribution-run-id>` |
| Retry a failed Actions build without another version bump | `make release-all BUMP=none PUBLISH=1` |
| Download packages from an existing successful build | `make release-resume RUN_ID=<distribution-run-id>` |

GitHub jobs continue if the local monitoring terminal disconnects. Check the run
before starting another one. The last build is recorded in ignored
`bin/release-build.json`. Publishing requires successful packages from all four
targets; if the checkout has advanced, `SOURCE_COMMIT=<full-sha>` explicitly
acknowledges the older built source. GitHub artifacts are retained for 30 days,
and its artifact quota can block these builds. This limit does not affect the
local release path.

#### What gets published

The publisher uploads packages to `downloads/<version>/`, using the same
versioned filenames as before. It supplies SHA-256 checksums to Bunny and fetches
every public package URL to verify its bytes. Only after all packages pass does
it replace the five stable update JSON files. It refuses downgrades and different
payloads under a published version.

After the new packages and all five stable feeds pass public verification, the
publisher keeps the latest two completed releases on Bunny: the current version
and the newest older completed version. It deletes older completed
`downloads/<version>/` folders, including their packages, checksums, and release
metadata. It rechecks the stable feeds before each deletion and skips cleanup if
they changed or a newer remote version appeared. Missing or malformed completion
metadata leaves a folder untouched. Legacy flat downloads, unrelated directories,
and `/updates/` files are preserved.

This retention policy runs automatically with both `make release-local PUBLISH=1`
and `make publish-local`, as well as the optional GitHub publisher. Failed
publications do not run cleanup. Cleanup errors are reported separately and do
not invalidate an already verified publication; retry `make publish-local` for
the same version to retry cleanup. Deploy the updated website metadata so its
download buttons point to the current retained release.

The Bunny base URL is `https://postika-desktop-application.b-cdn.net`:

| Hosted path | Contents |
| --- | --- |
| `/downloads/<version>/` | Packages, `SHA256SUMS`, `LICENSE.txt` and `release.json` |
| `/updates/stable.json` | Windows raw EXE, both macOS app ZIPs and Linux AppImage |
| `/updates/stable-deb.json` | Linux DEB |
| `/updates/stable-rpm.json` | Linux RPM |
| `/updates/stable-arch.json` | Linux Arch package |
| `/updates/stable-binary.json` | Standalone Linux ELF |

For example, a `0.1.8` Windows installer is
`https://postika-desktop-application.b-cdn.net/downloads/0.1.8/postika-0.1.8-windows-amd64-installer.exe`.
Feed URLs stay constant; their version, payload URLs, hashes and signatures change.
Keep legacy `0.1.2` files at their already-published flat `/downloads/` paths.

Bunny's `/updates/*.json` rule must continue serving `application/json` with
`Cache-Control: no-cache` or `no-store` and bypassing caching. The publisher verifies
these headers. Once bypass is configured, replacing feeds needs no cache-purge
API key. Uploads and public verification retry interrupted requests; a partial
feed failure may briefly leave platforms on different versions. Retry the same
build to complete publication.

#### Local packages and update feeds

`make update-feeds` signs and verifies a complete local package set without
uploading. `make website-release` regenerates website metadata from those
packages. `make publish-local` loads `.env` and the existing signing key, verifies
packages and feeds, and retries publishing the current version without a build
or bump.

Windows first-install downloads and updater payloads differ: the update feed
uses the raw application EXE, not the NSIS installer. macOS uses the app ZIP for
updates in both local and native/CI releases. Paid OS publisher signing and
notarization remain deferred; ad-hoc Mac signing is separate from the Ed25519
signatures on update feeds.

### Website downloads and update routing

Already-built apps request feeds at
`https://postika.softwrhq.com/updates/postika/stable*.json`. The website has five
exact GET/HEAD routes that issue temporary HTTPS redirects to Bunny's
`/updates/stable*.json`, preserving the query string. The app follows these
redirects. Deploy the website handler behind HTTPS and configure DNS for
`postika.softwrhq.com` before expecting installed apps to reach the feeds.
These routes remain the same for future releases; only Bunny's JSON contents
need replacing.

Download-page metadata is separate from the signed update feeds. The website
embeds `website/releases/current.json`, so download buttons work without
environment variables:

```sh
cd website
make run
```

`make release-local` generates `website/releases/<version>.json` and
`website/releases/current.json` with all eight versioned Bunny download URLs.
Mac URLs use `.app.zip` for local Linux releases. The optional
`make release-all PUBLISH=1` path syncs equivalent metadata from Bunny after
GitHub publishes its DMG-based release. No source-code embed path or URLs need
editing.
Release notes start empty; add actual notes to the version JSON and
`current.json` together before rebuilding the website. Existing release
dates and notes are preserved when regenerating metadata. Commit generated
metadata with your next source changes; the GitHub workflow requires a clean,
pushed checkout.

After publishing the packages and feeds, rebuild and deploy the website binary
so it embeds the new metadata. This automation does not deploy to the VPS;
the running website keeps its embedded release until replaced. The published
`downloads/<version>/release.json` is uploaded by the publisher and is also used
by the optional GitHub launcher to sync metadata. The running website uses its
embedded copy. Release notes appear when the metadata includes notes.

An optional `RELEASE_FILE` overrides the embedded metadata at startup. Use a path
relative to the website process's working directory, or an absolute path. When
using an override, deploy that JSON file alongside the executable and restart
the website to load changes. For example:

```sh
RELEASE_FILE=releases/0.1.2.json make run
```

After a successful publication, build the website for your Linux x64 VPS from
the repository root:

```sh
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 make -C website build
```

Upload `website/postika-website` to the VPS, replace the deployed binary, and
restart its website service. The binary includes the generated download links;
there is no separate JSON file to upload unless you explicitly use `RELEASE_FILE`.
Check `/downloads` for the new version after restarting. The existing update
redirect routes and DNS do not need changing for each release.

Configure the server's address using `ADDR` (default `127.0.0.1:8080`) behind
your HTTPS reverse proxy.

Before declaring auto updates available, verify the deployed website routes,
Bunny feeds, and native old-to-new installation flows. An app already running
the published version correctly reports no newer update.
See [Releasing](docs/RELEASING.md#signing-and-payloads) for verification and
release acceptance requirements.

macOS DMG packaging requires macOS. Preserve each architecture's output before
building the next, because some output paths are reused. Windows NSIS packaging
requires `makensis`, which also runs on Linux. macOS cross-compilation uses the
Docker toolchain; Linux x64 builds natively on Linux x64. For other hosts or
architectures, see [Wails cross-platform builds](https://v3.wails.io/guides/build/cross-platform/).

To create only a Linux DEB and install it after packaging succeeds:

```sh
wails3 task linux:create:deb ARCH=amd64
sudo apt install ./bin/postika.deb
```

Stop posting queues and fully restart Postika after a manual installation.
Building or installing a package does not replace an already running process.

Publisher signing and notarization are deferred. macOS bundles retain local
ad-hoc signing, which needs no paid certificate. MSIX is a separate build target
and is excluded from unsigned release packaging. A Homebrew cask template exists
in `build/homebrew/`; no public tap or working install command is published yet.

Optional obfuscation uses `make install-garble` to install Garble v0.16.0, followed
by `wails3 build --obfuscated`. Keep `wails_obfuscated.gen.go` in source control
and add explicit JSON tags to serialized Go fields. See the
[release guide](docs/RELEASING.md#development-and-validation) for the Go toolchain
setup required by Garble.

## Project layout

| Path | Purpose |
| --- | --- |
| `main.go`, `profiles.go`, `profile_operations.go` | Wails app setup and account service bindings |
| `groups_*.go`, `facebook_page_*.go` | Facebook browser sessions, groups, and page imports |
| `posting*.go`, `post_*.go` | Posting queues, request capture/replay, and formatting |
| `updates*.go`, `internal/updateauth/` | Update polling, verification, and platform installation |
| `frontend/src/` | Svelte interface, editor, drafts, themes, and localization |
| `frontend/bindings/` | Generated Wails service bindings and models |
| `build/`, `Taskfile.yml` | Native resources, platform packaging, and build tasks |
| `scripts/` | Release packaging, versioning, artifact verification, and signed feed generation |
| `website/` | Go website, download metadata, and update-feed redirects |
| `docs/` | Release instructions, translation guidance, and Wails v3 review |

## Ubuntu sandbox troubleshooting

If WebKit exits with `bwrap: setting up uid map: Permission denied`, Ubuntu's
AppArmor policy may be blocking user namespaces. The development profile grants
permission to this checkout's executable without disabling the WebKit sandbox:

```sh
sudo install -m 644 build/linux/postika-dev.apparmor /etc/apparmor.d/postika-dev
sudo apparmor_parser -r /etc/apparmor.d/postika-dev
```

Check the executable path inside the profile before loading it; update it if
the checkout has moved, then restart the app. Installed system packages include
a separate profile for the installed executable. AppImage installations may
need their own path-specific profile on affected systems.

## Documentation and license

- [Release and update guide](docs/RELEASING.md)
- [Wails v3 implementation review](docs/WAILS_V3_REVIEW.md)
- [Translation guide](docs/I18N.md)
- [Wails v3 build guide](https://v3.wails.io/guides/build/building/)

Postika is distributed under the [MIT License](LICENSE).
