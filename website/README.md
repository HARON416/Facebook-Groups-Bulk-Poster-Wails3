# Postika website

An independent Go website for DigitalOcean, using Go HTML templates, vendored
HTMX 2.0.10, TypeScript, and native nested CSS. The desktop application is unchanged.
The website does not require a database or external Go modules.

## Run

From `website/`:

```sh
make run
```

Run `make help` to list commands. `make tidy` tidies Go modules, `make deps`
installs frontend dependencies, `make frontend` compiles TypeScript, and
`make build` compiles both browser assets and the Go executable. Use `make check`,
`make test`, and `make fmt` for checks, tests, and Go formatting. Run `make deps`
once before commands that compile or check TypeScript. Plain `go run .` also works.

From the repository root, use `make -C website run` or `make -C website tidy`.
Environment settings work as usual, for example `ADDR=:8090 make run`.

Visit <http://127.0.0.1:8080> for the landing page and `/downloads` for downloads.
Set `ADDR=0.0.0.0:8080` in a container or hosted environment. Static files and
HTML templates are embedded into the executable; rebuild after editing them.

The compiled browser script is checked in, so Node is optional for running Go.
To change the TypeScript:

```sh
npm install
npm run check
npm run build
go test ./...
```

From this monorepo, the existing frontend compiler can also build the script:
`node website/scripts/sync-themes.mjs`, followed by
`frontend/node_modules/.bin/tsc -p website/tsconfig.theme.json` and
`frontend/node_modules/.bin/tsc -p website/tsconfig.json`.

## Website themes

The header selector shows the palette icon and current theme name, and opens a compact, scrollable dropdown with
the same searchable selection of 189 palettes as
Postika's desktop app. Use the arrow keys and Enter to select a result, or
Escape or a click outside to close the picker. Swatches are rectangular.

The site follows the OS color scheme until a visitor chooses a theme. Choices
are stored locally, apply to the home and downloads pages, and synchronize
across tabs. **System default** returns to the OS preference. A small external
script applies the saved palette before styles render, without inline scripts
or a change to the Content Security Policy. If storage is blocked, theme changes
still work for the current page.

Palette data and accessible color tokens come from the desktop source. Run
`npm run themes:sync` to refresh `src/theme-data.ts` and
`static/css/themes.css`; `npm run build` does this automatically and compiles
both browser scripts. Generated assets are checked in so the standalone Go
server and Docker image do not need Node at runtime. Run `npm test` for theme
preference, persistence, storage failure, and synchronization tests.

## Screenshots

Click a screenshot to open an enlarged image with a floating close button and
no surrounding title, footer, or panel. Escape, the close
button, or the backdrop dismisses it and returns focus to the screenshot link.
Without JavaScript, screenshot links open the original image directly.

The composer, accounts, languages, posting logs, posting intervals, page imports,
and scheduling use owner-supplied Bunny-hosted screenshots:

| Section | File | Aspect ratio |
| --- | --- | --- |
| Composer and groups | [Bunny-hosted WebP](https://postika-desktop-application.b-cdn.net/Screenshot%20From%202026-10-25%2011-40-59.webp) | 771:829 |
| Accounts | [Bunny-hosted WebP](https://postika-desktop-application.b-cdn.net/screenshot_highlight_accounts.webp) | 1188:797 |
| Posting logs | [Bunny-hosted WebP](https://postika-desktop-application.b-cdn.net/Screenshot%20From%202026-10-25%2013-14-06.webp) | 1188:797 |
| Languages | [Bunny-hosted WebP](https://postika-desktop-application.b-cdn.net/screenshot_highlight_languages.webp) | 1188:797 |
| Posting intervals | [Bunny-hosted WebP](https://postika-desktop-application.b-cdn.net/screenshot_highlight_interval.webp) | 1188:797 |
| Page imports | [Bunny-hosted WebP](https://postika-desktop-application.b-cdn.net/Screenshot%20From%202026-10-25%2013-31-04.webp) | 1188:797 |
| Scheduling | [Bunny-hosted WebP](https://postika-desktop-application.b-cdn.net/screenshot_highlight_schedule.webp) | 1188:797 |

When replacing screenshots, update the image and its original-image link in
`templates/home.html` and keep the real intrinsic dimensions. Each feature has
one full-width card in a vertical flow: copy on the left, image on the right on
desktop; copy followed by the image on phones. Screenshots have no visible
captions, colored frames, or padded wrappers, and remain clickable for an
image-only modal preview. Use owner-approved captures for publication. The
image policy permits the supplied Bunny hostname; other external image hosts
require an explicit policy entry.

## Downloads and releases

Downloads are disabled until release metadata is supplied. Copy
`release.example.json` to `release.json`, fill in verified version/date/notes
and the real HTTPS Bunny download URLs, then start with:

```sh
RELEASE_FILE=/absolute/path/to/release.json go run .
```

Metadata is loaded on startup; restart after changing it. Empty URLs remain
labeled Coming soon. No artificial versions, download counts, or testimonials
are displayed. The HTMX endpoint `/partials/release` refreshes release notes
from the server's current release metadata. All essential content is rendered
without JavaScript. TypeScript suggests a platform while keeping every format
visible.

The desktop updater feeds remain independently hosted on Bunny. This website
does not configure or publish them. The updates copy currently states that
native update delivery verification is in progress.

## Fonts

- Inter 4.1 variable WOFF2, upright and italic: body and controls. Its license
  is included in `static/fonts/inter/LICENSE.txt`.
- Satoshi Black OTF (weight 900): headings; Bold (700): wordmark. The supplied CDNFonts ZIP has no license; obtain
  the official webfont kit and license from <https://www.fontshare.com/fonts/satoshi>
  before publication, then replace its `@font-face` source with the supplied
  WOFF2. Do not convert the original font files.
- Originals remain in `font-sources/`, outside the publicly served directory.

The stylesheet uses rem sizing, a 62.5% root font size, 1.6rem body text, a reset
including pseudo-elements, custom scrollbars, and reduced-motion support.

## Deploy to DigitalOcean

Build browser assets before packaging if TypeScript has changed. From this
directory, `docker build -t postika-website .` creates a standalone image using
the checked-in browser assets. The image listens on port 8080 and runs as a
non-root user. Configure HTTPS through the hosting platform or reverse proxy.
Supply `RELEASE_FILE` and mount its JSON file if using external release metadata.
No hosting account changes or deployment have been performed.

## Website localization

The website uses **Wuchale 0.26.7**, matching the desktop application, with the
same 33 languages and native language names imported from
`frontend/src/locales/languages.js`. The header defaults to **🇺🇸 English**. Its compact, searchable, scrollable popup
matches the desktop picker, with each language's matching flag and native name.
Explicit choices persist across pages and synchronize across tabs.
Arabic and Hebrew set the document direction to RTL. Dates and theme counts use
locale-aware formatting. English HTML remains usable without JavaScript.

Catalogs live in `locales/*.po`. Initial non-English copy is machine translated,
with corrections to headlines and product names; native-language review is
still recommended. The selected catalog is fetched locally from
`/static/locales/`, compiled by Wuchale, and rendered without injecting HTML.
There is no external translation service at runtime. Release notes and unknown
future download format names remain as supplied in release metadata.

To update copy:

```sh
npm run i18n:extract
# Translate new blank msgstr entries in locales/*.po.
npm run i18n:check
npm run build
npm test
```

Extraction requires Python 3 and reads the Go templates plus dynamic messages
in `src/i18n-messages.json`. It preserves existing PO translations. The check
reports unextracted copy, missing translations, and broken placeholders or
inline markup. The browser runtime and all compiled catalogs are checked in
and embedded by Go, so production still needs only the standalone executable.
Node 22.12+ is recommended for building browser assets. Rebuild the Go binary
after changing templates or generated static assets.
