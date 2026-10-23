import messages from './log-messages.json';

const exact = messages as Record<string, string>;
type ParsedLog = { key: string; args: string[]; prefix: string; code: string; url: string; details: boolean };
const summarized = new Set(['facebook_fetch_error', 'facebook_photo_error', 'facebook_capture_error', 'groups_error', 'accounts_error']);
const patterns: [string, RegExp][] = [
  ['accounts_error', /^read account profiles: [\s\S]+$/],
  ['post_uncertain', /^(?:The connection failed; Facebook may have received the request|Could not read Facebook's confirmation; retained response is incomplete): [\s\S]+$/],
  ['groups_progress', /^Loaded (\d+) groups…$/],
  ['groups_restored', /^Restored (\d+) saved groups\. Load groups to refresh the list\.$/],
  ['groups_saved', /^Loaded and saved (\d+) groups\.$/],
  ['groups_unsaved', /^Loaded (\d+) groups, but could not save them\. Load groups again to retry saving\.$/],
  ['groups_error', /^(?:Facebook rejected the groups request \(HTTP \d+\)\. Check your login and try Load groups again|Facebook rejected the groups query(?: \(code [^)]+\))?\. Click Load groups to capture fresh values|The browser's groups request failed \(HTTP \d+\)\. Check Facebook in Chrome and try again\.|The browser's groups request succeeded, but Go replay returned HTTP \d+\. Click Load groups to try again|The browser's groups request succeeded, but the Go replay failed: [\s\S]+|The browser's own groups request failed: [\s\S]+)$/],
  ['no_composers', /^None of the (\d+) selected groups has an available post composer$/],
  ['image_read', /^Could not read (.+)\. Try browsing for it again\.$/s],
  ['image_unsupported', /^Unsupported file: (.+)\. Choose JPG, PNG, WebP, or GIF images\.$/s],
  ['image_not_file', /^(.+) is not an image file\. Drop individual images, not folders\.$/s],
  ['image_unsupported', /^Unsupported files: (.+)\. Choose JPG, PNG, WebP, or GIF images\.$/s],
  ['image_preview', /^Could not preview (.+)\. Please choose another image\.$/s],
  ['image_position', /^(.+) moved to position (\d+) of (\d+)\.$/s],
  ['opening', /^Login detected\. Opening the (Facebook) page…$/],
  ['facebook_progress', /^Fetched (\d+) of (\d+) posts…$/],
  ['facebook_end', /^Fetched (\d+) posts\. The page has no more posts available\.$/],
  ['facebook_complete', /^Fetched (\d+) posts\.$/],
  ['facebook_add_text', /^Add text to fetched post (\d+) before posting$/],
  ['rotate_prepare', /^Preparing post (\d+)\/(\d+) for (.+)\.\.\.$/s],
  ['rotate_uploads', /^Uploading images from post (\d+)\/(\d+) for (.+)\.\.\.$/s],
  ['rotate_submit', /^Posting (?:reusable )?post (\d+)\/(\d+) to (.+)\.\.\.$/s],
  ['facebook_download', /^Downloading full-size image (\d+) of (\d+) for post (\d+) of (\d+)…$/],
  ['facebook_fetch_error', /^Facebook(?: rejected the timeline request \(HTTP \d+\)\. Check your login and fetch again|'s timeline request failed \(HTTP \d+\))$/],
  ['facebook_photo_error', /^(?:Facebook rejected the photo query \(HTTP \d+\)\. Fetch again|Could not resolve full-size image \d+ for a fetched post\. Facebook did not include its photo ID)$/],
  ['published', /^Posted successfully to (.+)\.$/s],
  ['pending', /^(.+): Your post is pending\. Your post is awaiting admin approval\. If the admin team approves it, (?:it will|it'll) become visible in the group\.$/s],
  ['group', /^Group (\d+)\/(\d+): (.+)$/s],
  ['wait', /^Waiting (\d+)s before posting to (.+)\.$/s],
  ['countdown', /^Next post in (\d+:\d+) · (.+)$/s],
  ['upload', /^(.+): uploading image (\d+) of (\d+)…$/s],
  ['prepare', /^Preparing post 1\/1 for (.+)\.\.\.$/s],
  ['uploads', /^Uploading images from post 1\/1 for (.+)\.\.\.$/s],
  ['submit', /^Posting reusable post 1\/1 to (.+)\.\.\.$/s],
  ['captured', /^Fresh upload and post request values captured from (.+)\.$/s],
  ['scheduled', /^Queue scheduled for (\d{4}-\d\d-\d\dT[^ ]+)\. Keep Postika open until posting finishes\.$/],
  ['retry', /^Submission unconfirmed\. Retrying request (\d+)\/(\d+): [\s\S]+$/],
  ['finished', /^Finished: (\d+) published, (\d+) awaiting approval, (\d+) failed or skipped\.$/],
  ['opening', /^Opening (.+)…$/s],
  ['capture', /^Capturing image uploads \((\d+) of (\d+)\)…$/],
  ['gemini_http', /^Gemini rewrite failed \(HTTP (\d+)\)\. Check the key, model and account quota$/],
];

// Parse only Postika's known templates. Group names and external diagnostics
// remain data; never replace English words inside a user's group name.
export function parseLogMessage(raw: string): ParsedLog {
  // A known message can contain an example URL as part of its text.
  if (Object.hasOwn(exact, raw)) return { key: exact[raw], args: [], prefix: '', code: '', url: '', details: summarized.has(exact[raw]) };
  let value = raw.replace(/ · Item time \d+s$/, '');
  let code = '', url = '';
  const link = value.match(/ (https?:\/\/\S+)$/);
  if (link) { url = link[1]; value = value.slice(0, -link[0].length); }
  const errorCode = value.match(/ \(Code ([^)]+)\)$/);
  if (errorCode) { code = errorCode[1]; value = value.slice(0, -errorCode[0].length); }
  const result = (key = '', args: string[] = [], prefix = '', details = !key || summarized.has(key)) => ({ key, args, prefix, code, url, details });
  if (Object.hasOwn(exact, value)) return result(exact[value]);
  if (value.startsWith('API confirmation:')) return result(exact['Facebook response received'], [], '', true);
  if (value.endsWith(' Using the original text for this post only. Gemini remains enabled for the next post.')) {
    return result(exact['Gemini rewrite failed. Using the original text for this post; Gemini will be tried again for the next post.'], [], '', true);
  }
  for (const [key, pattern] of patterns) {
    const match = value.match(pattern);
    if (match) return result(key, match.slice(1), '', key === 'retry' || key === 'post_uncertain' || summarized.has(key));
  }
  const imageRead = value.match(/^Could not read (.+)$/s);
  if (imageRead) return result('image_read', [imageRead[1]]);
  // Outcomes prefix an exact message with a group name, which may contain ': '.
  for (const [message, key] of Object.entries(exact)) {
    const ending = ': ' + message;
    if (value.endsWith(ending)) return result(key, [], value.slice(0, -ending.length));
  }
  return result();
}

// A different language alone is not a reason to repeat an English status.
export function hasLogDetails(raw: string): boolean {
  return !!raw && parseLogMessage(raw).details;
}
