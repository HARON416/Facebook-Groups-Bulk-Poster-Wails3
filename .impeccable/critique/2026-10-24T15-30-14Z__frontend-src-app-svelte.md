---
target: Postika frontend posting workflow
total_score: 25
max_score: 40
na_heuristics: 
p0_count: 0
p1_count: 3
target_identity: "file:/home/kibet/Projects/Postika-WailsV3/frontend/src/App.svelte"
target_fingerprint: "sha256:41242d5c1901c29fddae5841f4c621fd2731759b5a02a07cf48cefd6a6f96f9e"
target_path: /home/kibet/Projects/Postika-WailsV3/frontend/src/App.svelte
timestamp: 2026-10-24T15-30-14Z
slug: frontend-src-app-svelte
closed: true
---
Method: dual-agent (A: /root/design_review · B: /root/detector_review)

Postika's workflow is specific to Facebook group posting; its visual structure is a conventional wizard. The biggest opportunity is a better final review before publishing, rather than a wholesale redesign.

The review used source inspection and fresh Chrome screenshots at 1280×900 and 1200×1000. These are fixture states, not verification of native WebKitGTK, all languages/themes, or screen-reader behavior.

| Heuristic | Score | Main observation |
|---|---:|---|
| System status | 3/4 | Clear progress; invalid settings lack an explanation. |
| Real-world language | 3/4 | Familiar labels; pacing needs interpretation. |
| User control | 2/4 | Group loading disables unrelated navigation. |
| Consistency | 3/4 | Coherent controls; celebration precedes success. |
| Error prevention | 2/4 | No final content/destination recap. |
| Recognition over recall | 2/4 | Earlier choices disappear before publishing. |
| Efficiency | 3/4 | Search, bulk selection, shortcuts; broad loading locks. |
| Minimalist design | 3/4 | Calm layout; repeated headings and sparse final step. |
| Error recovery | 2/4 | Interval errors silently disable actions. |
| Help | 2/4 | Useful instructions are under “Features.” |
| **Total** | **25/40** | **Acceptable; targeted improvements needed.** |

What works: three stages mirror the real task; consistent controls and a stable footer keep actions predictable. Attachment handling includes keyboard reordering and actionable errors. Per-account drafts and background queue indicators support ongoing work.

Priority issues:

1. **P1 — Add a review before publishing.** The final step shows timing fields without the post, attachments, or selected destinations. Users must remember earlier choices before an external action. Add account identity, an excerpt or imported-post count, image count, destination count with expandable names, and direct edit links. Explain keep-open/awake and queue non-restoration constraints for immediate and scheduled queues. Evidence: frontend/src/App.svelte:454 and :488; frontend/src/Workspace.svelte:86. Suggested command: $impeccable clarify.

2. **P1 — Explain invalid posting intervals inline.** Minimum 180 / maximum 120 disables Post now and Schedule without a visible explanation. Existing oninvalid handlers do not provide reactive feedback in this layout. Associate a specific correction with the field using aria-describedby and aria-invalid, and disclose the 86400-second upper bound. Evidence: frontend/src/App.svelte:28, :474, :488, :491; frontend/src/components/PostingOptionsForm.svelte:3, :19, :22. Suggested command: $impeccable harden.

3. **P1 — Narrow the lock while Facebook groups load.** Waiting for login disables Previous, account switching, background queue buttons, Posting logs, and the menu. No cancellation action appears in this view. A prolonged wait prevents unrelated work; a permanent hang was not observed. Preserve monitoring/navigation where safe, scope the lock to conflicting browser operations, and provide cancel/recovery with selections retained. Evidence: frontend/src/App.svelte:129, :476; frontend/src/Workspace.svelte:56, :65, :86, :89, :105. Suggested command: $impeccable harden.

4. **P2 — Tie celebration to outcomes.** Confetti fires on step navigation and immediately after Post now, before the result. A fixture screenshot shows celebration with no groups selected. Reserve celebration for an accurately named confirmed outcome; scheduling acceptance, published, pending, and unconfirmed submissions must stay distinct. Preserve reduced-motion support. Evidence: frontend/src/App.svelte:244, :350; frontend/src/lib/confetti.ts:37. Suggested command: $impeccable animate.

5. **P2 — Make help discoverable.** “Features” opens a feature inventory; “How to use” requires another selection. Expose Help or How to use directly and put concise guidance beside login waits and posting decisions. Evidence: frontend/src/components/FeaturesCard.svelte:13, :30, :40, :89; frontend/src/components/UsageGuide.svelte:28, :42. Suggested command: $impeccable clarify.

Cognitive load is moderate. Progressive steps help, but the seven title-bar utilities and nine editor buttons compete with the task. The more consequential burden is recalling content and destinations on the final screen. The emotional low point is the external login wait; premature celebration offers reassurance before the uncertain work has finished.

Persona checks: Alex benefits from bulk tools but loses multitasking during loading. Jordan encounters disabled publishing without an explanation and help named Features. Sam gets semantic controls and keyboard attachment moves, but interval errors lack associated feedback; actual screen-reader behavior and scaling remain unverified.

The detector found three warning-level side-tab accent-border matches: frontend/src/app.css:175 (blockquote), frontend/src/components/PostingLogView.svelte:307 (current activity), and :453 (outcome cards). The blockquote match is a false positive for generic decorative treatment. The two log treatments can communicate useful status and are at most P3 review candidates. No matches occurred in App.svelte. These syntactic matches establish no P0–P2 usability defect and provide no rendered contrast assessment.

Minor observations: repeated step descriptions occupy space that could support the recap; quiet draft-save reassurance could reinforce account continuity; showing the affected total beside Select all groups would clarify its scope while searching. Weekday shortcuts and the calendar begin on different weekdays.

Questions worth resolving: Can users verify the account, post, and destinations without going backward? Which independent actions should stay available while Facebook requests verification? Which confirmed outcome deserves celebration?
