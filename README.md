# Postika

A desktop app for posting text and images to Facebook groups across multiple
accounts. Built with Wails v3, Go, Svelte 5, and TypeScript.

## Workflow

1. **Create post** — write and format your message, then browse or drop images.
   Drag thumbnails to reorder them, or use Alt + Left / Right.
2. **Select groups** — load your Facebook groups, search, and select where to post.
   Sign in through Chrome when prompted. Saved groups remain available between runs.
3. **Preferences & post** — choose posting intervals, optionally enable Gemini
   rewriting, then post now or schedule the queue.

Use **Add account** and the profile switcher for separate Facebook logins,
groups, drafts, settings, and queues. Switching accounts leaves existing queues
running. **Posting logs** opens one shared window with its own profile switcher
and pause, resume, stop, progress, and response-copy controls.

The interface supports **32 languages** and **189 themes**, including Postika
Light and Dark. Language and theme choices sync across windows. App-generated
logs are translated; original diagnostics remain under **Debug details**.
Open **Features** in the top bar for a scrollable overview of the app’s features.
Choose **How to use** in the same card for a detailed guide to accounts,
composing posts, images, groups, intervals, Gemini, scheduling, logs, and troubleshooting.
On first launch, the interface follows the browser or system’s preferred
languages, falling back to English when none are supported. Choosing a language
manually saves it for future launches and all windows. No location access is used.

## Saved data

- Each account keeps its own browser profile, groups, and settings in the OS
  configuration directory under `postika/`. The default account retains its
  existing paths; additional accounts use `profiles/<profile-id>/`.
- Draft text and independent copies of image bytes are stored per account in
  IndexedDB. Restored images do not depend on the original files. Unreadable
  older attachments must be replaced or removed before posting.
- Drafts survive updates or reinstalls when application data is retained.
  Scheduled queues require Postika to stay open and the computer awake;
  queues and logs are not restored after quitting.
- Gemini receives the post text and rewrite instructions, without images.
  Failed rewrites fall back to the original text. Saved API keys stay in Go;
  leaving the key field blank preserves the existing key.

Results distinguish published, awaiting approval, failed, skipped, and
unconfirmed posts. Check unconfirmed posts on Facebook before retrying: a lost
response can leave a submission uncertain. Facebook changes can require updates
to browser automation and response parsing.

## Development

Requires **Go 1.25+**, **Node.js 24 LTS**, npm, and **Chrome/Chromium**.
On Ubuntu 24.04, install the native build dependencies:

```sh
sudo apt-get update
sudo apt-get install -y build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev
```

From the project root:

```sh
make install-cli          # Install the Wails CLI pinned to go.mod
npm ci --prefix frontend
make dev                  # Desktop app with hot reload
```

`make run` is an alias for `make dev`. Use `make doctor` to check dependencies.
For frontend-only development, run `npm --prefix frontend run dev`; native Go
calls require the desktop runtime.

## Checks and builds

```sh
make check
npm --prefix frontend run i18n:check
npm --prefix frontend run i18n:test
npm --prefix frontend run themes:test
npm --prefix frontend run logs:test
npm --prefix frontend run build
go test ./...
make build
```

The Linux executable is `bin/postika`. Browser integration checks require a
built frontend and Chrome; they use local fixtures without publishing to Facebook:

```sh
go test . -args -browser-integration
```

To build and install a Linux DEB, run from the project root:

```sh
PATH="$PWD/.tools/bin:$PATH" .tools/bin/wails3 task linux:create:deb
sudo dpkg -i bin/postika.deb
```

Install only after packaging succeeds. Stop active queues and restart Postika
after updating.

To build a Windows amd64 executable from Linux, run from the project root:

```sh
PATH="$PWD/.tools/bin:$PATH" .tools/bin/wails3 build GOOS=windows GOARCH=amd64
```

The executable is `bin/postika.exe`. To build a per-user Windows installer
(requires NSIS, with `makensis` on `PATH`):

```sh
PATH="$PWD/.tools/bin:$PATH" .tools/bin/wails3 package GOOS=windows GOARCH=amd64 INSTALL_SCOPE=user
```

The unsigned installer is `bin/postika-amd64-installer.exe`. These commands
rebuild the current version; they do not increment the release version metadata.

See the [release guide](docs/RELEASING.md) for platform packaging
and [translation guide](docs/I18N.md) for localization.

## Ubuntu sandbox troubleshooting

If WebKit exits with `bwrap: setting up uid map: Permission denied`, load the
included development AppArmor profile:

```sh
sudo install -m 644 build/linux/postika-dev.apparmor /etc/apparmor.d/postika-dev
sudo apparmor_parser -r /etc/apparmor.d/postika-dev
```

Restart `make dev`. The profile contains this checkout's absolute path; adjust
it if the project moves. Installed packages need a profile matching their
installed path.
# Facebook-Groups-Bulk-Poster-Wails3
