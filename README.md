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
this device. Reopen it from **How to use → Start guided tour**, or choose
**Read the guide** for the existing written instructions. The tour has Previous,
Next, Close and Finish controls, supports Escape and keyboard navigation, and
preserves the account's draft, workflow step and scroll position. It does not
fetch data, create accounts or start posting. Start it when no posting or group
loading operation is active. Tour copy follows the selected interface language.

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
embedded version used by automatic updates. Commit and push the changes before
`make release-all BUMP=none` to avoid incrementing it again. `make version-check` validates the metadata;
`make version-sync` repairs it from the config without incrementing anything.
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
| macOS Apple Silicon | `wails3 task darwin:package:all ARCH=arm64` | App bundle, DMG, and updater ZIP |
| macOS Intel | `wails3 task darwin:package:all ARCH=amd64` | App bundle, DMG, and updater ZIP |
| Linux x64 | `wails3 task linux:package ARCH=amd64` | AppImage, DEB, RPM, and Arch package |

### Artifact locations for hosting

Our initial public download set is:

| OS | Downloads to host |
| --- | --- |
| Windows x64 | Installer `.exe` and portable `.zip` |
| macOS Apple Silicon | arm64 `.dmg` |
| macOS Intel | amd64 `.dmg` |
| Linux x64 | `.AppImage`, `.deb`, `.rpm`, and Arch `.pkg.tar.zst` |

Homebrew will use the same hosted macOS DMGs through the published cask.
For automatic updates, also host the raw Windows application `.exe`, separate
macOS app-bundle `.zip` files for arm64 and amd64, and the JSON update manifests.
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
| macOS Apple Silicon | DMG | `bin/postika.dmg` | First-install download; also used by the Homebrew cask |
| macOS Apple Silicon | App bundle | `bin/postika.app/` | Input to DMG and updater ZIP packaging |
| macOS Apple Silicon | App-bundle ZIP | `bin/postika-darwin-arm64.app.zip` | In-app update payload |
| macOS Intel | DMG | `bin/postika.dmg` | First-install download; also used by the Homebrew cask |
| macOS Intel | App bundle | `bin/postika.app/` | Input to DMG and updater ZIP packaging |
| macOS Intel | App-bundle ZIP | `bin/postika-darwin-amd64.app.zip` | In-app update payload |
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
created by `make release-all`. The raw build outputs above can be overwritten by
the next architecture or platform build. `make package-all` packages only the
current OS; it is not needed again after a successful all-OS release.

### Publishing a release to Bunny

The recommended flow is **GitHub Actions → Bunny**. GitHub builds all four OS
and architecture targets, then a GitHub-hosted publish job downloads their
artifacts internally, verifies them, signs the update feeds and uploads to
Bunny. Large release files no longer pass through your laptop. Your laptop
starts and monitors the job, then downloads only the small website release JSON.

#### First-time setup

1. Authenticate GitHub CLI with `gh auth login`. Push the workflows and release
   scripts to the repository's default branch before using them.
2. If `.env` does not exist, copy `.env.example` to `.env`. Keep an existing `.env`;
   do not overwrite its credentials. Configure the Frankfurt Storage HTTP API:

   ```dotenv
   BUNNY_STORAGE_ZONE=postika-desktop-application
   BUNNY_STORAGE_HOST=storage.bunnycdn.com
   BUNNY_STORAGE_PASSWORD=your-storage-zone-password
   ```

   Use the Storage Zone password. `de-s3.storage.bunnycdn.com` is the separate S3
   endpoint; this uploader uses the HTTP endpoint without an `https://` prefix.
3. Restore and privately back up the existing `.update-keys/updater.key`. Do not
   generate a replacement key for already-distributed apps.
4. Run once from the repository root:

   ```sh
   make configure-publishing
   ```

   This validates that the private key matches the app's embedded public key,
   then configures GitHub repository secrets `BUNNY_STORAGE_PASSWORD` and
   `POSTIKA_UPDATE_PRIVATE_KEY`, plus repository variables `BUNNY_STORAGE_ZONE`
   and `BUNNY_STORAGE_HOST`. Secret values use stdin and GitHub's secret storage;
   they are not put in Git, command arguments or logs. The publish job creates
   a temporary key file and removes it after use.

`.env` and `.update-keys/` remain ignored by Git. Normal cloud publishing uses
GitHub's stored configuration and does not load your local `.env` or private key.
Keep local backups for recovery. The GitHub publish job has read-only repository
permissions and is serialized to prevent concurrent feed publication.

#### Which command to use

Run these commands from the **repository root**:

| Situation | Command |
| --- | --- |
| New release: bump version, build all OS targets and publish directly to Bunny | `make release-all PUBLISH=1` |
| Retry publication of the last recorded build without rebuilding or bumping | `make publish-release` |
| Publish a specific successful build, including one started before this automation | `make publish-release RUN_ID=<distribution-run-id>` |
| Build failed: rebuild the same version and publish | `make release-all BUMP=none PUBLISH=1` |
| Optional local package copy from an existing build | `make release-resume RUN_ID=<distribution-run-id>` |
| Advanced fallback: publish packages already on this computer | `make publish-local` |

For a normal release, commit and push your application changes, then run:

```sh
make release-all PUBLISH=1
```

The launcher checks the pushed checkout and repository publishing configuration,
reserves the next version, commits and pushes version metadata, then dispatches
the distribution workflow with publishing enabled. It waits for all build and
publication jobs. Successful publication syncs `website/releases/<version>.json`
and `website/releases/current.json` locally from Bunny. No packages are downloaded
to local `bin/releases/` in this mode. `BUMP=minor`, `BUMP=major`, and `VERSION=X.Y.Z`
select the version policy; the default is the next patch.

If your terminal loses its connection, GitHub continues working independently.
Check the run URL before starting another release. `make publish-release` reads
the run recorded in ignored `bin/release-build.json`; an explicit `RUN_ID` also
works on a fresh checkout. A saved publish-only run is reconnected to instead of
creating duplicate uploads. A completed distribution publish job only needs its
small metadata file synced again. Do not bump a new version just to recover a
monitoring or metadata-sync failure.

A publish retry never compiles or changes the app version. It verifies that all
four package jobs passed, and that the built version and embedded public key
match the publishing checkout. If your checkout has advanced beyond the built
commit, acknowledge the exact built source with `SOURCE_COMMIT=<full-sha>` or
build a fresh release to include later app changes. GitHub artifacts expire
after 30 days, so publish successful builds while their artifacts are available.

#### What gets published

The publish job uploads packages to `downloads/<version>/`, using the same
versioned filenames as before. It supplies SHA-256 checksums to Bunny and fetches
every public package URL to verify its bytes. Only after all packages pass does
it replace the five stable update JSON files. It refuses downgrades and different
payloads under a published version. Older remote releases are never deleted.

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

#### Optional local files

Plain `make release-all` without `PUBLISH=1` still builds and downloads packages
for local testing. `make release-resume RUN_ID=<run-id>` downloads an existing
build without rebuilding. These optional transfers still use your local network;
they are separate from cloud publishing. Local staging verifies checksums and
keeps the latest complete `bin/releases/<version>/` while preserving the previous
complete release if a download fails.

`make publish-local` retains the previous laptop-to-Bunny fallback and loads
local `.env` and the existing signing key. `make update-feeds` signs local
packages without uploading; `make website-release` generates website metadata
from a complete local package set. The normal cloud flow needs none of these.
Initial installers and updater payloads differ for Windows and macOS: feeds use
the raw Windows EXE or macOS app ZIP, never the NSIS installer or DMG. Paid OS
publisher signing and notarization remain deferred.

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

After `make release-all PUBLISH=1`, the small metadata file generated in GitHub
Actions is synced into `website/releases/<version>.json` and
`website/releases/current.json` with all eight versioned Bunny download URLs. No source-code embed path or URLs need editing.
Release notes start empty; add actual notes to the version JSON and
`current.json` together before rebuilding the website. Existing release
dates and notes are preserved when regenerating metadata. Commit generated
metadata with your next source changes before starting another all-OS build.

After publishing the packages and feeds, rebuild and deploy the website binary
so it embeds the new metadata. This automation does not deploy to the VPS;
the running website keeps its embedded release until replaced. The published
`downloads/<version>/release.json` is the metadata source synced by the local
release command, but the running website uses its embedded copy. Release notes appear when the metadata includes notes.

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
requires `makensis`. Linux and macOS cross-compilation use the Docker toolchain;
see [Wails cross-platform builds](https://v3.wails.io/guides/build/cross-platform/).

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
