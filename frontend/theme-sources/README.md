# Desktop palette sources

`colorhunt.json` records the 24 curated Color Hunt palettes added on 26 October
2026, including source links, original colors, observed likes, and selection
rationale. Theme names are Postika's own descriptive names.

The review fetched 20 feed pages (10 newest, 5 monthly popular, 5 annual popular),
returning 630 entries and 400 unique palettes. Monthly popular results ended
after the first page. Likes are a snapshot, used as a secondary signal rather
than a live ranking.

Selection favors quiet backgrounds, readable foregrounds, and colors distinct
from the existing library. Each source contributes one theme, with 12 light and
12 dark additions. All four original colors are retained as background, accent,
secondary, and text seeds. The existing token generator adjusts derived UI
colors to meet contrast requirements; it does not rewrite the original swatches.

The source registry is `src/lib/theme-palettes.json`. Run `npm run themes:generate`
after editing it, then `npm run themes:test`. Existing saved theme IDs remain
valid. The new IDs use the `palette:colorhunt-` prefix. No network requests are
made to Color Hunt at runtime.

This batch applies only to the desktop application. Website theme assets were
not synchronized.

## New-feed desktop additions

`colorhunt-new.json` records twelve further palettes selected from 200 unique
results across `sort=new` steps 0–4. The requested `timeframe=30` was retained,
but this feed also returned older palettes. Six light and six dark themes use
original source colors with at least 7:1 background/text contrast. Derived UI
tokens are checked by `npm run themes:test`. Source codes and URLs are recorded;
clearance cookies are not stored.

All desktop theme display names are lowercase. Existing IDs are unchanged so
saved selections continue working. Website theme files are not regenerated.
