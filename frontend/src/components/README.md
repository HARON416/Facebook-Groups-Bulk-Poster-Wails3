# Icons

Use official [Phosphor Icons](https://phosphoricons.com/) SVGs for interface icons.
Use regular weight to match the existing interface. Add the official SVG path to
`PhosphorIcon.svelte` and render it with `name` and an optional pixel `size`.
Keep `viewBox="0 0 256 256"` and `fill="currentColor"` so icons follow the theme.
Do not substitute hand-drawn approximations or emoji.

`EditorIcon.svelte` already contains regular-weight Phosphor editor icons.
The license is bundled at `frontend/public/licenses/phosphor-icons.txt`.

# Listing fields

Use `ListingFieldSelect.svelte` for custom listing dropdowns. The shared option
registry is `src/lib/marketplace-fields.ts`. Category labels, groups, icons, and
shipping hints follow the supplied menu in `MarketplaceCategories.svelte`, where
Wuchale translates them. `MarketplaceConditions.svelte` supplies the four
owner-provided conditions. `MarketplaceAvailability.svelte` supplies the two
availability choices and their descriptions. Location choices come from the
selected account’s live Marketplace search rather than a static option list. Stored option values are local draft keys, not Facebook field IDs.
Keep stored values stable and translate display labels through the interface
translation pipeline when adding options. Do not guess Facebook values.

`ItemListingEditor.svelte` implements a local draft and preview. Meetup choices,
promotion preference, friend visibility, and product tags are saved per account.
Saved drafts without those preferences receive the defaults in
`emptyItemListing()`. Promotion preferences do not launch ads, and the editor
does not publish listings or transfer photos from a phone. Those integrations
belong to later steps. Never display a posted timestamp for an unpublished draft.

Listing photos use pointer capture for rearranging inside desktop WebViews, matching
PostComposer. Native HTML drag events remain responsible for external file drops.
Reorder existing photo records only on release, retaining object URLs and the
active preview by ID. Alt + Left / Right offers the same action by keyboard
(and follows the visual direction in RTL). Escape cancels a drag.

Location search begins after a nonempty query, with an explanation before opening
Facebook. The query survives Cancel and browser setup. A dedicated account tab
uses the shared login detector and waits through login/verification without a
time limit. More details expansion and location capture each have bounded waits.

Capture listeners are armed before focusing Facebook’s Location combobox and
entering the initial query. Capture validates the full POST body, correlated wire
headers/cookies, authenticated account, HTTP status, and address response. Session
values stay in memory in Go and are never exposed in frontend status or drafts.
Later queries preserve captured fields while replacing params.query and advancing
the request sequence. New queries cancel older HTTP work; the UI also ignores
stale results. An invalid template requires a new consent/capture cycle.

Drafts store the selected place’s page ID, label, subtitle, coordinates, city, and
postal code in locationData. Unselected search text is not a valid location.
Stored place IDs are not a substitute for validating the final publish payload.
Profile shutdown cancels active capture and search work. Publishing is separate.


Reusable notifications use `notify({ kind, message, description?, duration?, actionLabel?, onaction? })` from `lib/notifications.svelte.ts`. Supported kinds are `info`, `success`, `warning`, and `error`; the globally mounted `NotificationHost` renders up to three bars in each window. Pass an `id` to replace a notification, or use the returned ID with `dismissNotification(id)`. `NotificationBar` can also be rendered inline. Success and information messages default to six seconds; warnings and errors remain until dismissed. Timers pause while hovered, focused, or the window is hidden. Closing a bar returns focus to the element active when it appeared. Native code can dispatch `postika:notification` with the same options as the event detail.

Location setup verifies a replay before closing its dedicated browser. When the account browser has other work or tabs, it closes only the Marketplace tab. A successful setup brings the listing window forward, restores the Location caret, and displays the shared success bar. Captured request/session values remain in memory for subsequent searches.


Shared motion is defined in `src/motion.css`, loaded by every window. Use `--motion-fast` (120ms) for control feedback, `--motion-state` (200ms) for state changes, `--motion-surface` (180ms) for surface reveals, and `--motion-ease`. Name transitioned properties explicitly; keep focus outlines, pointer-driven dragging, and layout changes immediate. Switch thumbs use transforms and respect right-to-left direction. Reduced-motion preferences override CSS transitions and decorative animation globally; JavaScript animation must also check the preference.


Location capture matches the typed query, ignoring the empty request from focusing the Facebook field. Capture and initial replay verification stay cancellable without a connection-speed deadline; transient failures retry with backoff while the UI remains in its searching state. Browser closure, Cancel, authentication failures, and app shutdown stop setup. Normal typeahead replays have a 45-second bound and newer queries still cancel older searches. Switches use a separate track and thumb element for consistent desktop webview animation, with a 260ms easing curve that spreads movement across the transition.


The shared listing editor supports items and vehicles. Vehicles have Vehicle type, Location, Year, Make, Model, Price, Description, and the promotion preference, with up to 20 photos. Item-only category, condition, tags, SKU, availability, meetup, and hide-from-friends controls stay in the item form. Vehicle titles derive from year, make, and model; the preview shows only provided vehicle attributes. `MarketplaceVehicles.svelte` owns the supplied Vehicle type options and the descending Year list from 2027 through 1901. Neither field is preselected. Location capture/replay is shared per account. Selected locations use the shared real static-map preview; publishing actions are not simulated.

Vehicle drafts use the compound IndexedDB key `['vehicles', profileID]`; existing item drafts keep their original profile key and database version. Account deletion removes both keys in one transaction. Switching listing kinds preserves unsaved form fields, location text, image order, and the active photo in the create window. Selecting Vehicles from the item Category dropdown opens the Vehicle for sale editor through the same navigation, preserving the prior item category and restoring the separate vehicle draft. Object URLs are recreated on remount and released when the previous editor closes.


Selecting Car/Truck reveals the make dropdown, mileage, appearance fields (body style, exterior/interior color), clean-title declaration, vehicle condition, fuel type, and transmission. Transmission defaults to the supplied Automatic transmission value; the supplied make names are in `vehicle-makes.ts`, and body style, colors, condition, fuel type, and transmission options are in `listingCarTruckOptions()`. Exterior and interior colors share the supplied palette but keep independent selections. Brand spellings and order are preserved in every locale. Previously entered free-text makes remain available as their draft’s current make. Motorcycle shows mileage, exterior color directly after price, fuel type, and transmission, with the same automatic transmission default. Car-only body style, interior color, clean-title declaration, and vehicle condition stay hidden for motorcycles. Motorcycle Make uses the supplied manufacturer list in `motorcycle-makes.ts`, preserving spelling and order; an existing draft make remains available. The supplied motorcycle exterior color, fuel type, and transmission lists match Car/Truck and share those localized options through `listingMotorcycleOptions()`. Powersport and RV/Camper keep Make and Model as text fields and show exterior color, interior color, and fuel type using the supplied lists, which match the shared vehicle options. Neither has a mileage or transmission field, and those attributes stay hidden from their previews. Trailer keeps Make and Model as text fields, shows the shared exterior and interior colors, and omits fuel type, mileage, transmission, and the Vehicle details section. Boat follows the supplied Powersport/RV layout with Make and Model as text fields, exterior/interior colors, fuel type, and video support. It currently uses the established shared color and fuel lists; the Boat form capture does not contain opened option lists. Commercial/Industrial follows the same supplied layout as Boat, using shared Year, exterior/interior colors, and fuel options, text Make/Model, and video support. Its form and preview omit mileage and transmission. Other shows the base vehicle form with text Make/Model, shared Year, and photo/video uploads. It has no color, fuel, mileage, or transmission fields, and hides those draft values in its preview. Vehicle details remain in the draft when another type is selected; only attributes relevant to the selected type appear in its preview.

`ListingVideo` accepts one locally playable video of at most 60 seconds. It checks actual metadata before replacing an accepted file, supports browsing and dropping, offers removal and cancellation, and releases temporary media/URLs on cancellation or unmount. Videos join the preview gallery and are stored with the vehicle draft as an optional binary record without changing the database version. Items, homes, and all eight vehicle types show the video uploader and preview gallery, and pass the video to the test/submission callback. Hidden or unfocused editors pause playback, and starting one preview pauses other players in that editor. Test-only WebM clips in `frontend/testdata` cover short, exactly 60-second, and over-limit video durations.

Photo reordering in both the listing editor and post composer uses the dragged image rectangle against visible target tiles, including edge contact. The largest overlap wins, with centre distance breaking ties; the source tile is excluded. Insertion follows the relative image order so touching an adjacent tile moves the image in that direction. Targets are scoped to the current gallery and clipped to its scroll viewport. Geometry and adjacent reordering regressions are covered by `node --test scripts/drag-target.test.mjs`.

Home for sale or rent opens the shared listing editor with up to 50 photos, one video of at most 60 seconds, offer/property dropdowns, bedroom/bathroom counts, price, address, and property description. Advanced Details is always visible, with an Optional label beside its heading, and contains square feet, laundry, parking, air conditioning, and heating. The six supplied home dropdown lists are defined in `MarketplaceHomes.svelte` and exposed through `homeListingFieldOptions` in the active locale. Rent/sale keys are `rent` and `sale` so price and location labels follow the offer. None is an explicit option in every advanced dropdown; leaving a field empty also remains valid. Address uses the same Facebook location capture/replay and searchable pin-icon combobox as item and vehicle locations, including result labels/subtitles, keyboard selection, loading, error, and setup states. Selecting a result stores its metadata and address label; editing invalidates the old result, and only a selected result satisfies location validation. Earlier typed-only home addresses are restored as search text awaiting selection. The preview shows only home attributes and excludes vehicle/item preferences. Selected home locations use `ListingLocationPreview`, shared with items and vehicles. Home drafts use `["homes", profileID]`, keep all form fields and media separate from item/vehicle records, and are removed on account deletion. `node --test scripts/marketplace-drafts.test.mjs` checks field/media round trips, complete photo selection, isolation, and account cleanup with an in-memory IndexedDB adapter.

`ListingLocationPreview.svelte` renders the selected location label and a compact Facebook static-map image using its latitude/longitude and the supplied Marketplace URL parameters, including a 2 km approximate-area circle. It appears for all listing kinds, updates after a new selection, uses Phosphor info/pin icons, and links to an external coordinate map through the shared browser-opening handler. Map-image failure shows a readable fallback; invalid coordinates do not request an image. The info control exposes the image link. Coordinate validation and URL parameters are covered by `node --test scripts/listing-map.test.mjs`. Map imagery requires the remote Facebook CDN to accept the image request.

Selecting Rent changes Property type to Rental type (Apartment, House, Townhouse), Price to Price per month, and Property description to Rental description. Rent also shows a private-room checkbox, optional Date available with the custom calendar picker, and Cat friendly/Dog friendly checkboxes. These values are saved in the home draft and appear in the rental preview; switching to For Sale hides them while preserving them for a switch back. Advanced Details remains always visible and optional. Earlier drafts receive empty/false defaults for the added fields.

The private-room rental control uses the shared compact `ListingSwitch`, with the label on the left and switch on the right. Enabling it reveals How many people live here?, Room Details (monthly price, Bathroom Type, furnished switch), and Property Details (bedrooms, bathrooms, address, description). Monthly price moves into Room Details without losing its value. Bathroom Type uses the supplied Private and Shared options, in that order. Disabling private room or switching to For Sale hides room-only fields and preview attributes, preserves draft values, and removes their validation requirement. Rental pet preferences also use compact switches.

`ListingDatePicker.svelte` provides the rental availability calendar: a Phosphor calendar icon, localized month/day labels, Sunday-first dates, previous/next month navigation, past-date disabling, selected/today states, an optional Clear date action, and a popover positioned above or below to fit the form scroll area. Keyboard support includes arrows, Home/End, Page Up/Down (Shift for years), Escape, and focus restoration after selection. Date values remain local `YYYY-MM-DD` strings in drafts. Date-only parsing, leap years, calendar grids, and month-end navigation are covered by `node --test scripts/calendar-date.test.mjs`.

All listing editor footers show Test and Save side by side. Save stores a per-account listing only when all required fields and media are valid. Item Test opens the selected account browser and adds media to the create-item page through Rod. Vehicle/home Test retain the existing submission callback and stay disabled when no handler is connected. The header Save listing action uses the same validation and save function.

Marketplace image selection retains and displays every supported unique image, even when over the limit (10 items, 20 vehicles, 50 homes). The count and inline warning are derived from the current selection and clear automatically at or below the limit. Extra images stay available to preview, reorder, and remove; adding remains available. Test and both Save actions are disabled while over the limit, with a matching save guard. Draft restoration never truncates images. Removing an earlier photo preserves the active preview image.

Items support the shared one-video uploader (maximum 60 seconds), drag/drop, inspection, removal, preview gallery, and binary draft restoration. Item Description is always visible immediately after Condition, above More details. The public notice includes the supplied prohibited-item summary and Commerce Policies link. Upload from your phone is not implemented.

Photos and video now share one media grid and a combined photo/video counter across item, vehicle, and home editors. The video occupies the first tile, can be selected for the main preview, and has its own Remove control. While local video validation is pending, the tile shows Loading video, an indeterminate progress bar appears above the grid, the grid has a danger border, and an inline message explains that loading must finish before proceeding. Save/Test remain disabled through the existing busy state. Removing a loading tile cancels validation. Grid drops route video files to the same inspector and images to the photo handler, including mixed batches. No server upload progress is simulated.

MP4/M4V/MOV validation reads local ISO movie metadata instead of requiring embedded-webview playback support. The reader skips media payloads, supports movie metadata at the end of a file and version 0/1 durations, and checks video-track dimensions and the existing 60-second limit. Other containers continue through webview metadata inspection. A valid attached video remains usable independently of webview playback. Video tiles and the gallery use still JPEG thumbnails rather than video players. `node --test scripts/mp4-metadata.test.mjs` covers container layouts, durations, invalid files, and cancellation. The reported 4096×2160 H.264 file was verified at 11.9467 seconds using ffprobe and accepted by the updated validator with browser decoding disabled.

Video media is thumbnail-only: no inline video player or playback controls are rendered in the editor or gallery. `video-thumbnail.ts` calls the stable native thumbnail binding directly. Webview canvas capture is intentionally excluded because it can report success while returning black or corrupted pixels. The native path uses FFmpeg from PATH, a private temporary file, a 30-second context deadline, bounded 640-pixel JPEG output, and cleanup on completion/cancellation. It never uploads or modifies the original. FFmpeg from PATH is required for still-thumbnail generation; it is not bundled. Thumbnails are regenerated on draft restoration. The stable binding ID is explicitly registered for obfuscated builds and kept in the frontend adapter; generated binding files are not edited. `go test media_thumbnail.go media_thumbnail_test.go` exercises generation and invalid input; the reported 4K MP4 was used to verify a 640×338 JPEG thumbnail.

The complete thumbnail task (file read plus native IPC) has a 15-second frontend deadline and immediate cancellation on removal, even if the native promise never settles. Timeout/failure clears the busy state and offers Try again without rejecting the video. `node --test scripts/thumbnail-task.test.mjs` covers success, stalled IPC, removal cancellation, and native failures. Video thumbnails use contain-fit with a themed background so the whole frame remains visible.

`createVideoPoster` isolates thumbnail lifecycle tracking to the selected File, a derived primitive account ID, and explicit retry count. Replacing the polled profile object or changing poster busy/image/error state does not restart generation. Removing/replacing the file, switching account IDs, and unmounting cancel stale work. `node --conditions=browser scripts/video-poster.test.mjs` compiles the production Svelte reactive helper and verifies repeated profile refreshes both during and after generation, explicit retries, cancellation, and stale-result suppression.

Item Test requires at least one image or accepted video (maximum 10 photos and 1 video), Title, a valid whole-number Price (including zero), Category, and Condition. Location is optional for items, but typed location text must be selected before testing. It opens `https://www.facebook.com/marketplace/create/item` in a separate per-account tab, reuses that tab on subsequent tests, waits for positive login detection without an elapsed login deadline, and supports Cancel/unmount/shutdown cancellation. Photos and video are staged locally and assigned to the hidden image/video inputs using Rod `Element.SetFiles`; translated text and generated CSS classes are not used as selectors. The video input is reacquired after adding photos to tolerate React replacing controls. After media insertion, the action fills Title, Price, Category, and Condition, then only supplied optional fields, and leaves the listing for review. Staged files remain available while Chrome may read them and are cleaned up on the next successful test or browser shutdown. Temporary files from failed/cancelled tests are removed. `go test . -run 'TestMarketplaceTestMediaValidation|TestMarketplaceItemMediaBrowserIntegration' -browser-integration` verifies local fixture uploads, photo-only/video-only/mixed selection, hidden inputs, control replacement, tab reuse, staging cleanup, login cancellation, and no Publish click. No live Facebook listing is used by these tests.

`MarketplaceItemFields` carries canonical category/condition keys and optional Description, Availability, Product tags, SKU, selected Location, meetup preferences, promotion, and hide-from-friends flags. Native validation rejects missing/invalid required values before launching Chrome. `marketplace_selectors.go` resolves fields from the supplied composer structure: input attributes, roles, associated popup relationships, and scoped control groups. Localized accessible names and generated CSS classes are not selectors. Dropdown choices map canonical values to the supplied menu positions (26 category, 4 condition, 2 availability options), following numeric option-ID suffixes when present; location matches use the selected label and subtitle, rejecting ambiguous matches. Product tags are committed with Enter and checked in the chip list. More details is expanded only when a supplied optional field requires it, including either explicit Availability choice. Empty optional values and false preferences leave Facebook defaults untouched. Checked preferences are clicked only when not already enabled. Item and vehicle form filling use caller cancellation without an overall elapsed-time cap. After filling, the action waits for Next to be enabled and clicks it once; it never clicks Publish. Once media has been assigned, staged files stay available even if later field filling fails so Chrome can finish reading them. `go test . -run TestMarketplaceItemFields -browser-integration` checks required-only and populated-optional forms, controlled input events, descriptive dropdown labels, tag chips, ambiguous city-label disambiguation, untouched defaults, and idempotent preferences using local fixtures.

Category, Condition, and Availability automation first clicks the labeled combobox and waits for its open menu. Category uses the supplied Dropdown menu dialog and role=button choices; Condition uses its named listbox and role=option choices; Availability follows aria-controls/aria-owns or the generic Select an option listbox. While a linked popup is not yet rendered, automation waits instead of falling back to another menu. Hidden/stale popups and unrelated listboxes are excluded. After choosing, the live field is reacquired and its selected label is verified, including when React replaces the control. Local dropdown tests include delayed menus, shipping/availability descriptions, stale hidden dialogs, unrelated visible options, control replacement, and both Availability choices.

Dropdown opening now targets the supplied `label[role="combobox"][aria-labelledby]` directly and checks `aria-expanded` before option lookup. If Facebook ignores a click while rendering, it reacquires the live label and retries after a grace interval; expanded controls are not clicked again. Disabled controls wait without a fixed opening deadline; browser closure, caller cancellation, account shutdown, or a caller-supplied deadline stop the wait. Internal two-second inspection probes are retried when slow rather than ending the dropdown operation. The local browser regression ignores and replaces the first label click for Category, Condition, and Availability, then verifies exactly one retry per field and no accidental re-closing.

Facebook Price verification compares exact numeric amounts rather than raw strings, accepting currency symbols/codes, grouping separators, decimal formatting, and Free for zero. A requested 29999 therefore matches Facebook’s $29,999. Different amounts, negative values, and malformed grouping are rejected. Input verification reacquires the live field and allows a brief bounded wait for React updates. The browser regression formats 29999 as $29,999, replaces the input node, and verifies automation continues through Category and Condition. Test browser cleanup waits for Chrome to exit before removing its temporary profile.

All listing price inputs—items, vehicles, home sales, whole-property rentals, and private-room rentals—accept ASCII digits only, including zero. Numeric keyboards and integer patterns are used; invalid typing/pasted edits are rejected without concatenating decimal parts or changing the previous amount. Existing fractional draft prices are preserved for correction but cannot pass Test validation. Native item validation also rejects fractional prices. Facebook display formatting such as $29,999.00 remains compatible with the integer amount 29999.

Location suggestion selection follows the Location input’s aria-controls/aria-owns or active-descendant listbox, with the supplied visible suggestion-list structure as a fallback. Facebook nests a role=option div inside each role=option li; only top-level rows count as candidates, so a single city is not falsely ambiguous. Matching uses the exact primary label plus the supplied subtitle, rather than neighbourhood-description substrings. Distinct rows with identical labels/subtitles remain ambiguous and are not chosen arbitrarily. After clicking, the input value, collapsed state, and popup dismissal are verified. Local browser tests cover nested Nairobi city/neighbourhood results, same-label cities with different subtitles, and truly duplicate rows.

Saved Marketplace listings now use individual `['listing', profileID, listingID]`
keys in the existing IndexedDB store. Opening Marketplace moves legacy single
item, vehicle, and home drafts into the collection in one transaction, preserving
photos, videos, and field defaults. Editing retains the listing ID; creating
a listing makes a new ID. Table rows expose Edit and Delete directly. Account deletion removes the collection as well as
legacy draft keys. Save refreshes the table through a window event and
BroadcastChannel, opens Marketplace, and resets the create window for another
listing. Search and listing-type filters compose; select-all applies to visible
item rows while preserving selections outside the filter.

`Publish selected` uses the same `testMarketplaceListingMedia` dispatcher as the listing
editor's Test button. It uploads the saved photos/video and fills the saved item
fields in the selected account's Facebook form, then clicks the enabled Next button once. It never clicks Publish.
Items, all eight supplied vehicle types, and homes for sale are selectable for testing; home rentals remain disabled. Multiple
listings are tested one at a time: after reviewing the filled form, Next listing starts
the next selected item so forms are never silently overwritten. The queue supports
Cancel and retry, locks selection/edit/delete during the run, and aborts on account
switch or window destruction. All saved listings remain Draft.

Item Test resolves navigation through the supplied two-segment progress structure
and its adjacent action row. The first step has one action; the second has two
(Previous and Publish), a visible level-one heading, and no visible required text
fields. No translated heading, button caption, or progress label is matched.
Readiness respects disabled state and inert ancestors, reacquires replaced controls,
and has a cancellable two-minute deadline. The first action is clicked once;
arrival detection then waits up to 30 seconds without clicking another action.
Group rows and warning text are not required for arrival. After arrival, the shared
`waitMarketplacePublishReady` check waits up to two minutes for the second footer
action to be enabled. It respects `aria-disabled="true"`, native disabled state,
inert/busy ancestors, and visible loading-state elements; explicit false or an
absent disabled attribute permits readiness once loading ends. The control is
reacquired through replacements without matching a translated label. This check
is shared by items and supported vehicle types and is available to future home automation; home
form filling remains unsupported. No Publish click or focus is dispatched.

These selectors are language-independent within the captured Facebook layout,
not a stable Facebook API. Menu order and grouped preference order remain explicit
assumptions because the supplied markup has no semantic value IDs. A unique visible popup with the expected option schema confirms a dropdown is
open even when Facebook leaves `aria-expanded="false"` on its control; an already
visible menu is consumed without another control click. Unexpected counts,
duplicate fields, mixed option indices, and ambiguous popups stop the flow.
A reorder with unchanged counts cannot be detected reliably and requires an updated
mapping. Optional-field layouts also need live verification across accounts.
Condition and Availability are separated by their positions around Description
and the tags/SKU/location section, rather than by mutable required-state attributes.
Briefly duplicated dropdown controls are allowed to settle during the
caller-cancellable wait; no ambiguous control is clicked.
SKU resolution requires its `aria-describedby` relationship to resolve inside the
composer, excluding unrelated optional text inputs. The anonymized expanded-form
capture in `testdata/marketplace/item-expanded.html` verifies the supplied control
structure with additional text inputs and changed SKU/help IDs.
Local Chrome fixtures cover altered labels, RTL layout, regenerated IDs, reversed
indexed options, populated fields, location selection, disabled/replaced controls,
cancellation, exactly one Next click, and zero Publish clicks. These fixtures do
not establish compatibility with every live Facebook variant.

The saved-listing table sorts by Listing, Price, or Updated (newest first by
initial default). Sorting and filtering retain selections by listing ID; missing
prices sort after numeric prices, and zero remains a numeric value. Supporting
listing details use translated option labels from the shared field registries.
Dates use the selected locale and local timezone, with the full timestamp in the
accessible name and hover title. At compact desktop widths, Edit and Delete stay
pinned to the table's trailing edge while the other columns scroll. The scroll
region is keyboard-focusable so keyboard users can reach offscreen columns.

Car/Truck Test opens `/marketplace/create/vehicle`, validates selected location,
year, supplied make, model, whole-number mileage/price, and transmission before
launching Chrome. Car/Truck is selected before uploading because subtype changes can reset media.
Up to 20 photos and one accepted video are staged through the shared upload helper. A checked nine-combobox layout identifies the supplied
Car/Truck fields; Type precedes media, model/mileage follow Make, and Price is the
non-combobox input with autocomplete disabled. Clean title uses `name="title_status"`.
Optional fields are filled only when supplied. Dropdown choices follow the supplied
option slots and counts; Year matches the live numeric caption, including localized
digits, without fixing the first year. Translated labels and generated ID prefixes
are not field selectors. Layout/count changes stop the flow; same-count reorderings
remain a mapping assumption. The existing location search setup is reused per
account, and the chosen location is selected in the visible vehicle form.
The shared Next/step-two checks wait for Publish readiness and stop without clicking it. The local Car/Truck browser
fixture uses the anonymized supplied markup, RTL captions, reversed indexed menus,
control replacement, required-only/all-optional input, video, and 20 photos. It does
not certify every live Facebook variant or other vehicle types.

Motorcycle Test uses the supplied six-combobox schema (type, year, make, exterior
color, fuel, transmission) and the existing motorcycle manufacturer slots copied
from `motorcycle-makes.ts`. It shares location, whole-number mileage/price, 20-photo
and video uploads, Next navigation, and read-only Publish readiness with Car/Truck.
Car-only body style, interior color, vehicle condition, and clean-title values
retained in drafts are ignored during validation and filling for motorcycles.
`testdata/marketplace/motorcycle.html` is the anonymized supplied form capture;
the Motorcycle browser test covers required-only and all-optional fields, RTL
captions, localized year digits, indexed menu reordering, replaced controls,
20 photos/video, and zero Publish clicks. Manufacturer menu order remains the
same explicit contract as the existing editor list; schema/count changes stop
selection rather than guessing another field.

After Next confirms the second step, the shared runner automatically selects up to
20 available Marketplace groups before waiting for Publish readiness. Existing
checked groups count toward the limit and are never unchecked. Disabled rows are
skipped; rows are reacquired and checked-state increases verified after every
click. Group checkboxes are scoped to the supplied second-step form and identified
by role/state plus their avatar/name structure, without matching translated UI
labels. The scroll container is advanced when currently rendered rows are
exhausted, with bounded settling time for appended groups. Empty/unavailable group
lists are allowed; persistent loading, unconfirmed clicks, unsupported rows, or a
preexisting over-limit selection stop with an error. Selection is shared by items,
all supported vehicle types and is reusable for future house filling. Publish is
never clicked. The anonymized `testdata/marketplace/groups.html` capture backs local
Chrome tests for the cap, existing selections, disabled rows, replacements, delayed
and lazy-loaded rows, cancellation, repeat runs, and zero Publish clicks.

Powersport Test uses the supplied five-combobox layout (type, year, exterior color,
interior color, fuel), with free-text Make and Model between Year and Price.
Mileage, transmission, body style, vehicle condition, and clean-title draft values
are ignored for this subtype. The supplied form is retained as an anonymized
`testdata/marketplace/powersport.html` fixture. It shares selected-location handling,
whole-number price validation, 20 photos/video, automatic group selection, Next,
and read-only Publish readiness with the other supported vehicle subtypes.

RV/Camper Test uses the verified five-dropdown/basic-text layout shared with
Powersport: type/year/exterior color/interior color/fuel, plus free-text Make and
Model. Validation and filling ignore retained mileage, transmission, body style,
vehicle condition, and clean-title values. Its supplied markup is retained in
`testdata/marketplace/rv-camper.html`. Test and Publish selected use the same media,
location, Next, optional suggested-group selection, and read-only Publish-readiness
flow as the other supported vehicle subtypes. Local Chrome coverage includes
required-only/all-optional fields, 20 photos/video, localized captions/year digits,
replaced controls, and zero Publish clicks.

Trailer Test uses the supplied four-dropdown layout: type, year, exterior color,
and interior color. Make and Model are text fields. Its supplied capture has new
generated IDs and supports 20 photos plus a video; selectors continue to use
structural attributes and relationships. Fuel, mileage, transmission, body style,
vehicle condition, and clean-title draft values are ignored for Trailer.
`testdata/marketplace/trailer.html` backs required-only/all-optional Chrome tests,
including photo/video uploads, Next, skipped empty group suggestions, Publish
readiness, and zero Publish clicks. The shared group tests cover nonempty lists.

Boat Test uses the supplied five-dropdown/basic-text layout with text Make/Model,
Year, both colors, and Fuel type. Its new generated field IDs are captured in
`testdata/marketplace/boat.html`; selectors continue to use structural attributes.
Validation ignores retained mileage, transmission, body style, vehicle condition,
and clean-title values. Test and Publish selected share 20 photos/video, selected
location, Next, automatic suggested-group selection, and read-only Publish readiness.
The Boat fixture covers required-only/all-optional values with localized captions,
reordered indexed menus, control replacements, and zero Publish clicks.

Commercial/Industrial Test uses its supplied five-dropdown/basic-text form:
Vehicle type, Year, both colors, Fuel type, and text Make/Model. It reuses the
existing year/color/fuel lists and ignores retained mileage, transmission,
body style, condition, and clean-title fields. The anonymized supplied form is
`testdata/marketplace/commercial-industrial.html`. Test and Publish selected share
media upload, selected location, Next, suggested-group selection when available,
and Publish readiness without clicking Publish. Local fixtures cover required-only
and all-optional input, 20 photos/video, localized captions, replaced controls,
and zero Publish clicks; closed-form markup does not independently verify menu
contents or order.

Other Test uses the supplied two-dropdown layout (Vehicle type and Year) and text
Make/Model. Colors, fuel, mileage, transmission, body style, condition, and clean-title
values retained in the draft are ignored. The anonymized supplied capture is
`testdata/marketplace/other.html`. All eight supplied vehicle types are now enabled
in Test and Publish selected, retaining required-field validation, media limits,
location selection, Next, optional suggested-group selection, and read-only Publish
readiness. Other browser fixtures cover required-only and all-optional input,
20 photos/video, localized year digits/captions, replacements, and zero Publish clicks.

The shared dropdown opener has no fixed elapsed-time deadline, so it can wait for
slow Facebook rendering across item/vehicle fields and future home integrations.
The enclosing item and vehicle filling functions also impose no overall time cap.
Other stage-specific checks (input retention, Next, arrival, Publish readiness,
group confirmation, browser startup, and location search) keep their own bounds.
A local Chrome regression enables Vehicle type after 26 seconds (past the former
25-second cap), then verifies caller cancellation and browser-close detection.

Home-for-sale Test opens `/marketplace/create/rental` and selects For Sale before
uploading media. The home binding has stable ID 4270654620; the shared media runner
accepts exactly one fields variant and permits up to 50 home photos plus one accepted
video, retaining the existing item/vehicle limits. Native validation requires Sale,
a supplied property type, whole-number bedrooms/price, integer or half bathrooms,
a selected address matching its location hit, and a description. Optional square
feet and advanced dropdown values are checked before launching Chrome.
The supplied six-combobox sale schema identifies Home offer, Property type,
Laundry, Parking, Air conditioning, and Heating structurally; bedroom/bathroom
inputs precede Price, and square feet follows Description. The existing home option
lists supply canonical menu slots/counts; closed-form HTML alone does not establish
that every locale/account retains those menu orders. The current selected-location
search is reused for Address, matching the chosen live suggestion and subtitle;
live address suggestions still depend on Facebook's page and shared search source.
Rent/private-room automation is not enabled. Item/vehicle/Home sale flows share
Next, automatic selection of up to 20 suggested groups, and read-only Publish
readiness. Home form/dropdown filling has no added elapsed-time cap and inherits
caller cancellation. `testdata/marketplace/home-sale.html` is the anonymized capture;
local Chrome tests cover required/all-optional fields, 50 photos/video, duplicate
address captions disambiguated by subtitle, RTL/altered captions, indexed menu
reordering, replaced controls, and zero Publish clicks.
