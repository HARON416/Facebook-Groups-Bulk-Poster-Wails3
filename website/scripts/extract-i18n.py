"""Extract website template copy; retain existing translations by source message."""
import json, re, sys
from pathlib import Path
from html.parser import HTMLParser
ROOT = Path(__file__).resolve().parents[1]
messages = set()
class Copy(HTMLParser):
    def handle_data(self, value):
        value = value.strip()
        if value and re.search(r'[A-Za-z]', value) and not value.startswith(('postika', '©')):
            messages.add(value)
    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        for name in ('alt', 'aria-label', 'placeholder'):
            if attrs.get(name): messages.add(attrs[name])
for path in (ROOT / 'templates').glob('*.html'):
    Copy().feed(re.sub(r'{{.*?}}', '', path.read_text(), flags=re.S))
messages.update(json.loads((ROOT / 'src/i18n-messages.json').read_text()))
# Brand and platform/package names are product identifiers, not prose.
messages.difference_update({'Postika Light', 'Windows, macOS & Linux', 'Download ', 'Version  ()', 'English'})
messages.add('Windows, macOS & Linux')
output = json.dumps(sorted(messages), ensure_ascii=False, indent=2) + '\n'
if '--check' in sys.argv:
    if (ROOT / 'locales/en.json').read_text() != output:
        sys.exit('Website copy changed. Run npm run i18n:extract and translate new PO entries.')
else:
    (ROOT / 'locales/en.json').write_text(output)
    # Keep all existing translations and add new strings as untranslated entries.
    for path in (ROOT / 'locales').glob('*.po'):
        contents = path.read_text()
        known = {json.loads(key) for key in re.findall(r'^msgid (".*")$', contents, re.M)}
        for message in sorted(messages - known):
            value = message if path.stem == 'en' else ''
            contents += '\nmsgid ' + json.dumps(message, ensure_ascii=False) + '\nmsgstr ' + json.dumps(value, ensure_ascii=False) + '\n'
        path.write_text(contents)
print(f'{len(messages)} website messages')
