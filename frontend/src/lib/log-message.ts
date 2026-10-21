import messages from './log-messages.json';

const exact = messages as Record<string, string>;
type ParsedLog = { key: string; args: string[]; prefix: string; code: string; url: string };
const patterns: [string, RegExp][] = [
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
  let value = raw.replace(/ · Item time \d+s$/, '');
  let code = '', url = '';
  const link = value.match(/ (https?:\/\/\S+)$/);
  if (link) { url = link[1]; value = value.slice(0, -link[0].length); }
  const errorCode = value.match(/ \(Code ([^)]+)\)$/);
  if (errorCode) { code = errorCode[1]; value = value.slice(0, -errorCode[0].length); }
  const result = (key = '', args: string[] = [], prefix = '') => ({ key, args, prefix, code, url });
  if (Object.hasOwn(exact, value)) return result(exact[value]);
  if (value.startsWith('API confirmation:')) return result(exact['Facebook response received']);
  if (value.endsWith(' Using the original text for this post only. Gemini remains enabled for the next post.')) {
    return result(exact['Gemini rewrite failed. Using the original text for this post; Gemini will be tried again for the next post.']);
  }
  for (const [key, pattern] of patterns) {
    const match = value.match(pattern);
    if (match) return result(key, match.slice(1));
  }
  // Outcomes prefix an exact message with a group name, which may contain ': '.
  for (const [message, key] of Object.entries(exact)) {
    const ending = ': ' + message;
    if (value.endsWith(ending)) return result(key, [], value.slice(0, -ending.length));
  }
  return result();
}
