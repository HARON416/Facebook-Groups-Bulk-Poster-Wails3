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

All listing editor footers show Test and Save side by side. Save invokes the existing per-account draft save and is available for incomplete drafts. Item Test opens the selected account browser and adds media to the create-item page through Rod. Vehicle/home Test retain the existing submission callback and stay disabled when no handler is connected. The header Save draft action continues to use the same save function.

Marketplace image selection retains and displays every supported unique image, even when over the limit (10 items, 20 vehicles, 50 homes). The count and inline warning are derived from the current selection and clear automatically at or below the limit. Extra images stay available to preview, reorder, and remove; adding remains available. Test and both Save actions are disabled while over the limit, with a matching save guard. Draft restoration never truncates images. Removing an earlier photo preserves the active preview image.

Items support the shared one-video uploader (maximum 60 seconds), drag/drop, inspection, removal, preview gallery, and binary draft restoration. Item Description is always visible immediately after Condition, above More details. The public notice includes the supplied prohibited-item summary and Commerce Policies link. Upload from your phone is not implemented.

Photos and video now share one media grid and a combined photo/video counter across item, vehicle, and home editors. The video occupies the first tile, can be selected for the main preview, and has its own Remove control. While local video validation is pending, the tile shows Loading video, an indeterminate progress bar appears above the grid, the grid has a danger border, and an inline message explains that loading must finish before proceeding. Save/Test remain disabled through the existing busy state. Removing a loading tile cancels validation. Grid drops route video files to the same inspector and images to the photo handler, including mixed batches. No server upload progress is simulated.

MP4/M4V/MOV validation reads local ISO movie metadata instead of requiring embedded-webview playback support. The reader skips media payloads, supports movie metadata at the end of a file and version 0/1 durations, and checks video-track dimensions and the existing 60-second limit. Other containers continue through webview metadata inspection. A valid attached video remains usable independently of webview playback. Video tiles and the gallery use still JPEG thumbnails rather than video players. `node --test scripts/mp4-metadata.test.mjs` covers container layouts, durations, invalid files, and cancellation. The reported 4096×2160 H.264 file was verified at 11.9467 seconds using ffprobe and accepted by the updated validator with browser decoding disabled.

Video media is thumbnail-only: no inline video player or playback controls are rendered in the editor or gallery. `video-thumbnail.ts` calls the stable native thumbnail binding directly. Webview canvas capture is intentionally excluded because it can report success while returning black or corrupted pixels. The native path uses FFmpeg from PATH, a private temporary file, a 30-second context deadline, bounded 640-pixel JPEG output, and cleanup on completion/cancellation. It never uploads or modifies the original. FFmpeg from PATH is required for still-thumbnail generation; it is not bundled. Thumbnails are regenerated on draft restoration. The stable binding ID is explicitly registered for obfuscated builds and kept in the frontend adapter; generated binding files are not edited. `go test media_thumbnail.go media_thumbnail_test.go` exercises generation and invalid input; the reported 4K MP4 was used to verify a 640×338 JPEG thumbnail.

The complete thumbnail task (file read plus native IPC) has a 15-second frontend deadline and immediate cancellation on removal, even if the native promise never settles. Timeout/failure clears the busy state and offers Try again without rejecting the video. `node --test scripts/thumbnail-task.test.mjs` covers success, stalled IPC, removal cancellation, and native failures. Video thumbnails use contain-fit with a themed background so the whole frame remains visible.

`createVideoPoster` isolates thumbnail lifecycle tracking to the selected File, a derived primitive account ID, and explicit retry count. Replacing the polled profile object or changing poster busy/image/error state does not restart generation. Removing/replacing the file, switching account IDs, and unmounting cancel stale work. `node --conditions=browser scripts/video-poster.test.mjs` compiles the production Svelte reactive helper and verifies repeated profile refreshes both during and after generation, explicit retries, cancellation, and stale-result suppression.

Item Test requires at least one image or accepted video (maximum 10 photos and 1 video), Title, a valid whole-number Price (including zero), Category, and Condition. Location is optional for items, but typed location text must be selected before testing. It opens `https://www.facebook.com/marketplace/create/item` in a separate per-account tab, reuses that tab on subsequent tests, waits for positive login detection without an elapsed login deadline, and supports Cancel/unmount/shutdown cancellation. Photos and video are staged locally and assigned to the hidden image/video inputs using Rod `Element.SetFiles`; translated text and generated CSS classes are not used as selectors. The video input is reacquired after adding photos to tolerate React replacing controls. After media insertion, the action fills Title, Price, Category, and Condition, then only supplied optional fields, and leaves the listing for review. Staged files remain available while Chrome may read them and are cleaned up on the next successful test or browser shutdown. Temporary files from failed/cancelled tests are removed. `go test . -run 'TestMarketplaceTestMediaValidation|TestMarketplaceItemMediaBrowserIntegration' -browser-integration` verifies local fixture uploads, photo-only/video-only/mixed selection, hidden inputs, control replacement, tab reuse, staging cleanup, login cancellation, and no Publish click. No live Facebook listing is used by these tests.

`MarketplaceItemFields` carries canonical category/condition keys and optional Description, Availability, Product tags, SKU, selected Location, meetup preferences, promotion, and hide-from-friends flags. Native validation rejects missing/invalid required values before launching Chrome. `marketplace_item_fields.go` resolves the supplied English Facebook field labels via associated labels, aria-label, and aria-labelledby, without copied dynamic IDs or CSS classes. Dropdown choices use exact supplied option labels; location matches use the selected label and subtitle, rejecting ambiguous matches. Product tags are committed with Enter and checked in the chip list. More details is expanded only when a supplied optional field requires it, including either explicit Availability choice. Empty optional values and false preferences leave Facebook defaults untouched. Checked preferences are clicked only when not already enabled. Form filling has a cancellable two-minute deadline following the login wait. The action never clicks Next or Publish. Once media has been assigned, staged files stay available even if later field filling fails so Chrome can finish reading them. `go test . -run TestMarketplaceItemFields -browser-integration` checks required-only and populated-optional forms, controlled input events, descriptive dropdown labels, tag chips, ambiguous city-label disambiguation, untouched defaults, and idempotent preferences using local fixtures.

Category, Condition, and Availability automation first clicks the labeled combobox and waits for its open menu. Category uses the supplied Dropdown menu dialog and role=button choices; Condition uses its named listbox and role=option choices; Availability follows aria-controls/aria-owns or the generic Select an option listbox. While a linked popup is not yet rendered, automation waits instead of falling back to another menu. Hidden/stale popups and unrelated listboxes are excluded. After choosing, the live field is reacquired and its selected label is verified, including when React replaces the control. Local dropdown tests include delayed menus, shipping/availability descriptions, stale hidden dialogs, unrelated visible options, control replacement, and both Availability choices.

Dropdown opening now targets the supplied `label[role="combobox"][aria-labelledby]` directly and checks `aria-expanded` before option lookup. If Facebook ignores a click while rendering, it reacquires the live label and retries after a grace interval; expanded controls are not clicked again. Disabled controls wait, browser closure is detected, cancellation is respected, and opening has a 25-second deadline. The local browser regression ignores and replaces the first label click for Category, Condition, and Availability, then verifies exactly one retry per field and no accidental re-closing.

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
rows while preserving selections outside the filter.

`Publish selected` opens a review of the selected saved listings only. It never
submits to Facebook. Group selection and actual publishing remain future steps;
all saved listings are shown as Draft.

The saved-listing table sorts by Listing, Price, or Updated (newest first by
initial default). Sorting and filtering retain selections by listing ID; missing
prices sort after numeric prices, and zero remains a numeric value. Supporting
listing details use translated option labels from the shared field registries.
Dates use the selected locale and local timezone, with the full timestamp in the
accessible name and hover title. At compact desktop widths, Edit and Delete stay
pinned to the table's trailing edge while the other columns scroll. The scroll
region is keyboard-focusable so keyboard users can reach offscreen columns.
