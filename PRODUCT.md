# Postika

<!-- impeccable:product-schema 1 -->

## Platform

web

Postika is a desktop application built with Wails v3, Go, Svelte 5,
TypeScript, and Lexical. Its interface uses web technologies inside a desktop
webview; it is not a public browser app or a native mobile interface.

## Users

People managing posts across multiple Facebook groups, including people who
need separate Facebook accounts and independent posting queues. This audience
and the constraints below were confirmed by the project owner.

## Product Purpose

Compose or import posts, choose Facebook group destinations, and publish or
schedule a posting queue while keeping account data and progress understandable.

## Operating Context

- Windows, macOS, and Linux desktop packaging is provided in the repository.
- An installed Chrome or Chromium browser handles Facebook login and verification.
- Users move through Create post, Select groups, and Post or schedule.
- Posting logs show queue progress and provide pause, resume, and stop controls.
- Multiple accounts can have queues running while the user changes workspaces.

## Capabilities and Constraints

- Preserve account separation for browser sessions, groups, drafts, settings,
  and queues.
- Preserve rich text composition, image attachments and reordering, page-post
  import and review, group search, and bulk selection.
- Drafts and local image copies are saved per account in IndexedDB. Imported
  image URLs can expire.
- Posting intervals range from 60 to 86400 seconds, with a minimum and maximum.
- Scheduled and active queues require Postika to remain open and the computer
  awake. Queues and posting logs are not restored after quitting.
- Preserve distinct published, pending approval, failed, skipped, and
  unconfirmed outcomes. An uncertain submission must be checked on Facebook
  before retrying to avoid duplicates.
- Preserve localization, right-to-left layouts, and user-selectable themes.
  The repository documents 33 languages and 189 themes.
- Facebook browser automation depends on account access and Facebook behavior.
  Do not promise uninterrupted posting or claim scheduling survives app exit.

## Brand Commitments

Preserve the Postika name and existing app identity during refinement. The owner
has authorized setup and critique, not a replacement visual identity.

## Evidence on Hand

- README.md documents workflows, saved data, platform support, and limitations.
- frontend/src/components/UsageGuide.svelte contains the in-app instructions.
- frontend/src/ contains the existing interface, themes, and locale catalogs.
- build/appicon.png is an existing app asset.
- Browser integration tests use local fixtures; they do not publish Facebook posts.
- Release feeds and public downloads are documented as pending publication.
  Do not invent release availability, testimonials, customer counts, or metrics.

## Product Principles

- Keep account identity and posting destinations clear before publishing.
- Preserve user drafts and distinguish saved content from transient queues.
- Make queue state, confirmation uncertainty, and recovery actions explicit.
- Keep frequent posting workflows efficient and usable across supported
  languages, directions, and themes.

## Open Decisions

Specific customer segments, commercial positioning, and a product-specific
accessibility conformance target have not been established.
