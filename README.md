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

Open **How to use** inside the app for the detailed workflow.

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

Close any installed Postika instance before running `make dev`. Installed and
development builds share the same single-instance ID: if Postika is already
running, the development build focuses that instance and exits instead of
opening a new window. A successful build alone does not replace the running app.

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
make release-all                 # Bump patch, commit/push metadata, then build
make release-all BUMP=minor      # Bump minor instead
make release-all BUMP=major      # Bump major instead
make release-all VERSION=0.3.2   # Use an explicit higher version
make release-all BUMP=none       # Retry the current version without a bump
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

For all four release targets with one command, after committing and pushing
your application changes:

```sh
make release-all
```

This starts `.github/workflows/distribution.yml` on native Linux, Windows,
Apple Silicon, and Intel Mac runners, waits for completion, verifies the downloads,
and flattens all packages into `bin/releases/<version>/`, ready to upload.
It keeps one license, combines the SHA-256 checksums, and removes the nested
download folder only after verification. After the new release is complete,
older local versions and obsolete CI download folders are removed, leaving the
latest release. Failed downloads retain the previous complete release.
Install and authenticate GitHub CLI (`gh auth login`)
first. The workflow must already exist on the repository's default branch, and
the selected remote branch must match your local commit. It builds artifacts;
it generates website metadata but only uploads packages and signs/publishes
update feeds when `PUBLISH=1` is supplied. It does not create a GitHub release.
MSIX and universal Mac bundles remain separate optional targets.

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

Run the release commands below from the **repository root**.

#### First-time setup

1. Install the development tools described above, including Node.js 24, the
   project-local Wails CLI (`make install-cli`), and GitHub CLI. Authenticate GitHub
   CLI with `gh auth login`.
2. If `.env` does not exist, copy `.env.example` to `.env`. Keep an existing `.env`;
   do not overwrite its credentials. For this Frankfurt storage zone, configure:

   ```dotenv
   BUNNY_STORAGE_ZONE=postika-desktop-application
   BUNNY_STORAGE_HOST=storage.bunnycdn.com
   BUNNY_STORAGE_PASSWORD=your-storage-zone-password
   ```

   Use the **Storage Zone password**, not the global Bunny account API key.
   `de-s3.storage.bunnycdn.com` is Frankfurt's S3 endpoint; this uploader uses the
   HTTP API at `storage.bunnycdn.com`. The hostname has no `https://` prefix.
3. Restore the existing update signing key at `.update-keys/updater.key` and keep
   it backed up privately. Do not generate a replacement key for an app already
   distributed with the existing public key.
4. Commit and push the application and release-automation changes. The all-OS
   build command requires a clean checkout matching the remote branch.

The Make commands load `.env` automatically. `.env` and `.update-keys/` are
ignored by Git; never commit either. `.env.example` contains placeholders only.

#### Which command to use

| Situation | Command |
| --- | --- |
| New release: bump version, build every OS, and upload packages and feeds | `make release-all PUBLISH=1` |
| Current version is already built and downloaded: publish it or retry a failed upload | `make publish-release` |
| Build failed: retry the same version, then publish it | `make release-all BUMP=none PUBLISH=1` |
| Build a new release for testing before uploading | `make release-all` |

For the normal new-release workflow, after committing and pushing changes:

```sh
make release-all PUBLISH=1
```

This checks Bunny credentials and signing-key availability before reserving the
next version, bumps patch by default, builds all four targets using GitHub
Actions, downloads and verifies the packages, generates website metadata, signs
and verifies the five feeds, and publishes to Bunny. For example, if
`build/config.yml` contains `0.1.3`, the next patch release is `0.1.4`.
`BUMP=minor`, `BUMP=major`, and `VERSION=X.Y.Z` select another version policy.
`BUMP=none` keeps the already reserved version.

To test packages before publishing:

```sh
make release-all
# Install and test the generated packages, then:
make publish-release
```

Do not run `make release-all PUBLISH=1` again just to retry an upload: it starts
another version. Use `make publish-release` instead. It uses the complete
`bin/releases/<current-version>/` folder and never bumps, commits, pushes, or
rebuilds the app. It cannot publish a version whose build/download is incomplete.
Existing feeds are reused only after signature verification and validation
against the expected versioned URLs. `make package-all` is not needed after a
successful all-OS build.

The private signing key stays local. Run only one publisher at a time, including
across machines. A local lock prevents two publishers in the same checkout;
after a crash, check it stopped before removing `bin/.publish.lock`.

The publisher uploads packages to `downloads/<version>/`, creating the versioned
folder through the Storage API upload paths. It sends SHA-256 checksums with each
upload and downloads each public CDN URL to verify the bytes before touching any
stable feed. This public verification downloads roughly one release's size in
addition to the upload. It uploads `SHA256SUMS` after the packages, stores website
metadata as `downloads/<version>/release.json`, then replaces the five JSON feeds
in `updates/`. Feeds are checked publicly for matching bytes, JSON content type,
and no-cache headers. Transient connection errors and HTTP 429/5xx responses are retried up to three
times with fresh upload streams. Persistent errors report safe network codes
without credentials. CDN replication/cache failures stop publishing and give a
retry command. A failure partway through publishing feeds can leave some platforms
on the previous version; retry the same release to finish.

The Bunny base URL is `https://postika-desktop-application.b-cdn.net`:

| Hosted path | Contents |
| --- | --- |
| `/downloads/<version>/` | Versioned packages, `SHA256SUMS`, `LICENSE.txt`, and `release.json` |
| `/updates/stable.json` | Windows raw EXE, macOS app-bundle ZIPs for both architectures, and Linux AppImage |
| `/updates/stable-deb.json` | Linux DEB |
| `/updates/stable-rpm.json` | Linux RPM |
| `/updates/stable-arch.json` | Linux Arch package |
| `/updates/stable-binary.json` | Standalone Linux ELF |

For example, a `0.1.4` installer URL will be
`https://postika-desktop-application.b-cdn.net/downloads/0.1.4/postika-0.1.4-windows-amd64-installer.exe`.
The feed URLs stay the same; their version, payload URLs, hashes, and signatures
change. Existing `0.1.2` files at the old flat `/downloads/` URLs remain valid
and should stay in place. The publisher never deletes old remote releases and
refuses newer-to-older feed changes or different payloads under a published
version. Local release cleanup does not delete anything from Bunny.

Bunny's `/updates/*.json` rule must continue serving `application/json` with
`Cache-Control: no-cache` or `no-store` and bypassing CDN caching. Purge old cached
feeds when first configuring that rule. Normal publishing uses the storage
password only and does not need an account API key to purge caches. The already
configured cache bypass makes future feed replacements visible without a purge.

`make update-feeds` remains available to sign and verify feeds without uploading.
`make website-release` regenerates download metadata for a complete downloaded
release. Neither command changes the app version. The build and upload commands
can be previewed without network writes using `node scripts/release-all.mjs --plan`
and `node scripts/publish-release.mjs --plan`.

Initial-download archives and updater payloads differ for Windows and macOS:
feeds reference the raw Windows EXE or macOS app-bundle ZIP, never the NSIS
installer or DMG. Update signing is separate from OS publisher code signing;
paid publisher signing and notarization remain deferred.

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

After a successful `make release-all`, `website/releases/<version>.json` and
`website/releases/current.json` are generated automatically with all eight
versioned Bunny download URLs. No source-code embed path or URLs need editing.
Release notes start empty; edit the version's `notes` array and run
`make website-release` to copy the notes into `current.json`. Existing release
dates and notes are preserved when regenerating metadata. Commit generated
metadata with your next source changes before starting another all-OS build.

After publishing the packages and feeds, rebuild and deploy the website binary
so it embeds the new metadata. This automation does not deploy to the VPS;
the running website keeps its embedded release until replaced. The published
`downloads/<version>/release.json` is a metadata copy, not a live source for the
website. Release notes appear when the metadata includes notes.

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
