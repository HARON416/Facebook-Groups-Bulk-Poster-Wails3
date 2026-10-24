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

The **Themes** button opens the same searchable selection of 189 palettes as
Postika's desktop app. Use the arrow keys and Enter to select a result, or
Escape to close the picker. Swatches are rectangular.

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

The illustrations are explicitly labeled placeholders. Replace these images
with real, sanitized app captures:

| Section | File | Aspect ratio |
| --- | --- | --- |
| Composer and groups | `static/screenshots/compose.svg` | 900:650 |
| Accounts | `static/screenshots/accounts.svg` | 900:650 |
| Posting logs | `static/screenshots/logs.svg` | 1400:680 |
| Languages | `static/screenshots/languages.svg` | 900:650 |
| Scheduling | `static/screenshots/schedule.svg` | 900:650 |
| Page imports | `static/screenshots/imports.svg` | 900:650 |
| Automatic updates | `static/screenshots/updates.svg` | 900:650 |

For PNG/WebP captures, update the corresponding image paths in
`templates/home.html`, preserve the intrinsic dimensions/aspect ratio, and
remove the placeholder captions. The composer and updates use split screenshot-and-copy sections on desktop;
accounts/languages and scheduling/imports form paired rows, with posting logs
spanning the page. All sections stack screenshot first on phones. Screenshot frames contain
the complete image without cropping. The site uses white, black, and the
desktop app's light-theme teal (`#087f75`, hover `#096d65`). No personal account
names or actual customer content should appear in public screenshots.

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
