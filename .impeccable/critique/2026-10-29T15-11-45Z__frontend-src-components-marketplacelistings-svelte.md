---
target: Marketplace saved listings table
total_score: 27
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 0
target_identity: "file:/home/kibet/Projects/Postika-WailsV3/frontend/src/components/MarketplaceListings.svelte"
target_fingerprint: "sha256:383c6b963a96e5f5a7d5b754bd0c26f25dee77591f567766121aa45bfc216f85"
target_path: /home/kibet/Projects/Postika-WailsV3/frontend/src/components/MarketplaceListings.svelte
timestamp: 2026-10-29T15-11-45Z
slug: frontend-src-components-marketplacelistings-svelte
closed: true
---
Method: dual-agent (A: /root/table_design_review · B: /root/table_evidence_review)

# Marketplace saved listings review — before refinement

Target: frontend/src/components/MarketplaceListings.svelte. Operate surface.
Reviewed the production component in a browser fixture at 1200px and 800px,
including light/dark themes and RTL. This is a baseline assessment, not a score
for the subsequent implementation.

## Design specificity and overall impression

The table fits Postika: it manages account-specific item, vehicle, and home
listings as drafts, with explicit selection and visible edit/delete actions.
The existing app identity is coherent. Precise table hierarchy, rather than a
new visual identity, is the right next step. The strongest opportunity is to
make listing identity and price easier to compare while supporting details recede.

## Design health

| Heuristic | Score /4 | Key evidence |
|---|---:|---|
| Visibility of system status | 3 | Loading, selection counts, saved feedback, draft status |
| Match with real world | 3 | Familiar labels; Publish selected currently means review only |
| User control and freedom | 3 | Clear selection, cancel deletion, return from editor |
| Consistency and standards | 3 | Theme tokens; row font differs from app typography |
| Error prevention | 3 | Account boundaries, deletion confirmation, disabled empty selection |
| Recognition rather than recall | 3 | Visible identity, location, type; compact actions need horizontal discovery |
| Flexibility and efficiency | 2 | Search and selection; no user-controlled ordering |
| Aesthetic and minimalist design | 2 | Filled actions and repeated timestamps compete with listing identity |
| Error recovery | 3 | Plain errors and retry; native runtime recovery not exercised |
| Help and documentation | 2 | Guidance exists; publishing limitation appears after action |
| **Total** | **27/40** | **Acceptable baseline; targeted refinements needed** |

## What's working

- Titles and thumbnails identify mixed listing types; missing images have meaningful type icons.
- The supplied checkbox retains a native input, associated label, mixed state, focus styling, and reduced-motion support.
- Saved drafts remain account-specific. Deletion is confirmed and selected IDs survive filtering.

## Priority issues

1. **P2 — Row hierarchy and state clarity.** Repeated filled Edit buttons and full timestamps distract from titles; prices start-align; supporting metadata has similar weight. Hover also overrides the selected-row background because its selector is more specific. Improve title/thumbnail hierarchy and supporting labels, end-align prices with tabular figures, shorten the timestamp presentation with a full accessible value, use quieter semantic button treatments, and preserve selection on hover. Suggested commands: impeccable typeset, quieter, polish.
2. **P2 — Comparison and compact navigation.** Ordering is implicit, Actions falls outside the initial 800px viewport, and repeated Edit/Delete labels lack listing context. Add sorting with visible direction and aria-sort, keep action access available during horizontal scrolling, and give repeated actions listing-specific accessible names. Preserve ID-based selection and native keyboard interaction. Suggested commands: impeccable adapt, harden, polish.

## Cognitive load and emotional journey

Eight columns plus repeated colored actions flatten the distinction between
identity, comparison information, and secondary metadata. There is no decision
point with more than four alternatives. Saved-draft feedback reassures users;
the current Publish selected label may overpromise to a first-time user. The
owner explicitly retains a test-only review flow, with group selection and live
submission deferred. That known limitation is outside this table refinement.

## Persona red flags

- Power user: cannot sort by price, listing title, or last update.
- First-time user: publishing capability is intentionally unfinished and explained in review.
- Keyboard/screen-reader user: compact scrolling and repeated action names need stronger support; checkbox semantics are a strength.

## Deterministic and browser evidence

Source detector: 0 findings, JSON []. Live CDP detector: 0 findings, JSON [].
Manual checks still found start-aligned prices and selected-hover state loss.
Cell padding: 14px vertical / 12px horizontal. Rows: approximately 76.5px.
Action targets: 33px high. Checkbox labels: 32px square. No contrast violation
was established. An empty detector result does not certify usability.

## Minor observations and boundaries

Keep capitalized headers, supplied checkbox shape/animation, Edit and Delete
only, current themes, draft storage, and review-only publishing. Do not invent
currency information or published state. Native shell and screen-reader behavior
remain untested; browser measurements are a fixture fallback.

Questions skipped: 2 grouped priority issues; the user already confirmed refinement priorities and scope.
