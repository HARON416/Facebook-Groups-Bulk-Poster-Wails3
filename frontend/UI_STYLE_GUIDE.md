# App UI style reference

The desktop app uses InterVariable and the shared rules in `src/app.css`.
Use those rules for new screens and dialogs; the public website has its own
styles. Values below describe the existing app scale, not a new visual identity.

| Role | Token | Current value |
| --- | --- | --- |
| Screen, section, and dialog titles | `--ui-title-size` | 15px |
| Title weight | `--ui-title-weight` | 600 |
| Body, form fields, ordinary actions | `--ui-body-size` | 13px |
| Toolbar, hints, metadata | `--ui-small-size` | 12px |
| Form labels and action weight | `--ui-label-weight` | 550 |
| Content padding and section gaps | `--ui-space` | 16px |
| Related field and option gaps | `--ui-field-gap` | 8px |
| Dialog corners | `--ui-panel-radius` | 10px |
| Input and compact option corners | `--ui-input-radius` | 6px |

## Dialogs and controls

Use `.ui-dialog`, `.ui-dialog-heading`, `.ui-title`, `.ui-dialog-close`, and
`.ui-dialog-footer`. A component owns its width and scrolling behavior; let
the shared classes own type, border, shadow, backdrop, and header padding.
Contact is 520px wide, account and Facebook import forms are 480px, themes
are 520px, and the help reader is 880px. Width follows content, not text size.

Use `.ui-input` for standard fields and `.action-button` variants for workflow
actions. Compact toolbar actions use 12px secondary text, transparent surfaces,
no borders, and visible keyboard focus. Contact and How to use remain text-only.

Contact options retain real `mailto:` links in browser previews. In the desktop
app, `ContactService.OpenEmail` checks the native launcher result; Linux also
checks the default mail association. Failure reveals the address, translated
recovery text, and a Gmail compose link preserving the selected topic.

Use `--text-strong` for headings, `--text` for content, `--text-secondary` for
supporting text, and `--text-muted` for less prominent metadata. Surface,
border, warning, error, and focus colors come from theme tokens. Avoid literal
colors for states that must work across the app's palettes.

Post previews and the rich-text editor deliberately use larger content text.
Queue counters and hour/minute inputs also retain larger numeric text. Those
roles are distinct from dialog titles, descriptions, and ordinary buttons.

## Consistency review

- Contact's 24px title and 18px choices now use the shared 15px/13px scale.
  Its oversized frame and padding now follow the common dialog rules.
- Account creation, rename, deletion, Facebook import, themes, help, and
  Contact all use the same title and body sizes.
- Repeated sizes in help, themes, settings, toolbar controls, composer hints,
  account menus, scheduling, and logs now reference the shared tokens.
- Text previously below 12px in scheduling, log metadata, composer hints,
  account menus, and compact step descriptions now uses the small-text token.
- The Updates action keeps its requested borderless appearance. Its available
  state and panel shadow now use theme tokens.

Validation used browser previews of all seven dialogs, scheduling, and the
empty posting-log view, plus Contact at narrow width, in dark mode, and in
Arabic RTL. Native account creation, deletion, posting, and updating were not
executed. Source review covered the main workflow, composer, editor, settings,
queue review, logs, menus, and dialogs. This is a consistency review, not an
accessibility certification or a check of every palette and translated string.

The mechanical detector also flagged existing quote and log side borders.
The rich-text quotation border and log outcome markers were retained as content
and status conventions; they were not introduced or promoted into dialog styling.
