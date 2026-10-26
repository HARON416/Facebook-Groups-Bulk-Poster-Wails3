"""One-time machine translation bootstrap for public marketing copy.
Not used during builds or at runtime. Review translations in locales/*.po.
"""
import concurrent.futures, json, re, time, urllib.parse, urllib.request
from pathlib import Path
ROOT = Path(__file__).resolve().parents[1]
messages = json.loads((ROOT / 'locales/en.json').read_text())
languages = re.findall(r"code: '([^']+)'", (ROOT.parent / 'frontend/src/locales/languages.js').read_text())
def write_po(code, values):
    lines = ['# Website copy. Initial non-English translations are machine generated; review before publication.', 'msgid ""', 'msgstr ""', '"Content-Type: text/plain; charset=UTF-8\\n"', f'"Language: {code}\\n"', '']
    for source, value in zip(messages, values):
        lines.extend(['msgid ' + json.dumps(source, ensure_ascii=False), 'msgstr ' + json.dumps(value, ensure_ascii=False), ''])
    (ROOT / f'locales/{code}.po').write_text('\n'.join(lines))
write_po('en', messages)
def translate(code):
    path = ROOT / f'locales/{code}.po'
    if path.exists(): return code + ' already complete'
    values = []
    batches, batch, size = [], [], 0
    for message in messages:
        if size + len(message) > 2300:
            batches.append(batch); batch, size = [], 0
        batch.append(message); size += len(message)
    if batch: batches.append(batch)
    for batch in batches:
        query = urllib.parse.urlencode([('client','dict-chrome-ex'), ('sl','en'), ('tl',code), *[('q',m) for m in batch]])
        for retry in range(5):
            try:
                request = urllib.request.Request('https://clients5.google.com/translate_a/t?' + query, headers={'User-Agent':'Mozilla/5.0'})
                result = json.load(urllib.request.urlopen(request, timeout=45))
                if len(result) != len(batch) or not all(isinstance(v,str) for v in result): raise ValueError(str(result)[:200])
                for source, target in zip(batch,result):
                    if sorted(re.findall(r'\{\d+\}', source)) != sorted(re.findall(r'\{\d+\}', target)):
                        raise ValueError('Placeholder mismatch: ' + target)
                values.extend(result)
                break
            except Exception:
                if retry == 4: raise
                time.sleep(2 ** retry)
    write_po(code, values)
    return code + ' complete'
with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
    for result in pool.map(translate, [c for c in languages if c != 'en']): print(result, flush=True)
