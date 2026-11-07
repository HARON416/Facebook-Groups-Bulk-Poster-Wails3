"""One-time translation bootstrap for missing frontend messages.

Uses the same public-copy translation endpoint as website/scripts/translate-i18n.py.
Only empty/fuzzy translations and long English fallbacks are replaced. Existing
translations are preserved. Not used in builds or at runtime. Review the resulting
catalog changes; placeholders are protected and verified before writing.
"""

import argparse
import ast
import concurrent.futures
import json
from pathlib import Path
import re
import time
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
TOKENS = re.compile(r"\{\d+\}|<\d+/>")
# Disambiguate housing and ad-posting terms without changing the source catalog.
SOURCE_HINTS = {
    "Cat friendly": "Cats allowed",
    "Dog friendly": "Dogs allowed",
    "Property square feet": "Property area (square feet)",
    "This is a private room in a shared property.": "This listing is for a private room in a shared home.",
    "Listing": "Ad",
    "Saved listings": "Saved ads",
    "Listing details": "Ad details",
    "Listing preferences": "Ad preferences",
    "Publish selected": "Post selected ads",
    "Publish selected<0/>": "Post selected ads<0/>",
    "Publishing listing {0} of {1}: {2}": "Posting Marketplace ad {0} of {1}: {2}",
    "Publish sent to Facebook. Review the result in Chrome before continuing.": "The request to post the ad was sent to Facebook. Review the result in Chrome before continuing.",
    "Could not confirm whether Publish was sent. Check Facebook before continuing or retrying this listing.": "Could not confirm whether the request to post the ad was sent. Check Facebook before continuing or retrying this ad.",
    "Publishing stopped. Check Facebook before retrying this listing.": "Posting stopped. Check Facebook before retrying this ad.",
}
# Button verbs, rather than school-exam nouns returned for the isolated word Test.
TEST_LABELS = dict(zip(
    "id cs da de es fr hr it sw hu nl pl pt-BR ro sk sv vi tr el bg ru uk ar he mr hi bn te th ko zh-CN ja".split(),
    ["Uji", "Vyzkoušet", "Test", "Testen", "Probar", "Tester", "Isprobaj", "Prova", "Jaribu", "Tesztelés", "Testen", "Przetestuj", "Testar", "Testează", "Vyskúšať", "Testa", "Thử", "Test et", "Δοκιμή", "Тествай", "Проверить", "Перевірити", "اختبار", "בדיקה", "चाचणी करा", "परीक्षण करें", "পরীক্ষা করুন", "పరీక్షించండి", "ทดสอบ", "테스트", "测试", "テスト"],
))


def translation_source(source):
    if source in SOURCE_HINTS:
        return SOURCE_HINTS[source]
    # "Listing" alone is otherwise translated as a list, directory, or inventory.
    return re.sub(r"\blistings?\b", lambda match: {
        "listing": "ad", "listings": "ads", "Listing": "Ad", "Listings": "Ads",
    }.get(match[0], "ad"), source)


def entries(text):
    for block in text.split("\n\n"):
        if "#:" not in block or re.search(r"^#~", block, re.M):
            continue
        values = {}
        current = None
        for line in block.splitlines():
            match = re.match(r"^(msgid|msgstr) (.*)$", line)
            if match:
                current = match[1]
                values[current] = ast.literal_eval(match[2])
            elif line.startswith('"') and current:
                values[current] += ast.literal_eval(line)
            else:
                current = None
        if values.get("msgid"):
            yield block, values["msgid"], values.get("msgstr", "")


def missing(block, source, target):
    return not target.strip() or bool(re.search(r"^#,.*\bfuzzy\b", block, re.M)) or (len(source) > 20 and source == target)


def protect(source):
    tokens = []
    def replace(match):
        tokens.append(match[0])
        return f"ZXQPH{len(tokens)-1}QXZ"
    return TOKENS.sub(replace, source), tokens


def restore(target, tokens):
    for index, token in enumerate(tokens):
        target, count = re.subn(rf"ZXQPH\s*{index}\s*QXZ", lambda _: token, target, flags=re.I)
        if count != 1:
            raise ValueError(f"Lost translation placeholder: {target}")
    return target


def translate(code, cache_dir, review_context=False):
    path = ROOT / f"src/locales/{code}.po"
    text = path.read_text()
    needed = [(block, source) for block, source, target in entries(text) if missing(block, source, target) or (review_context and (translation_source(source) != source or source == "Test"))]
    cache_path = cache_dir / f"{code}{'-context' if review_context else ''}.json"
    cache = json.loads(cache_path.read_text()) if cache_path.exists() else {}
    hints = cache.setdefault("_source_hints", {})
    for _, source in needed:
        if hints.get(source) != translation_source(source):
            cache.pop(source, None)
    cache['Test'] = TEST_LABELS[code]
    messages = [source for _, source in needed if source not in cache]
    batches, batch, size = [], [], 0
    for source in messages:
        if batch and size + len(source) > 2300:
            batches.append(batch)
            batch, size = [], 0
        batch.append(source)
        size += len(source)
    if batch:
        batches.append(batch)
    for batch in batches:
        protected = [protect(translation_source(source)) for source in batch]
        query = urllib.parse.urlencode([("client", "dict-chrome-ex"), ("sl", "en"), ("tl", code), *[("q", source) for source, _ in protected]])
        for retry in range(5):
            try:
                request = urllib.request.Request("https://clients5.google.com/translate_a/t?" + query, headers={"User-Agent": "Mozilla/5.0"})
                with urllib.request.urlopen(request, timeout=20) as response:
                    result = json.load(response)
                if len(result) != len(batch) or not all(isinstance(value, str) for value in result):
                    raise ValueError("Unexpected translation response")
                targets = [restore(target, tokens) for target, (_, tokens) in zip(result, protected)]
                for source, target in zip(batch, targets):
                    if sorted(TOKENS.findall(source)) != sorted(TOKENS.findall(target)) or not target.strip() or (len(source) > 20 and source == target):
                        raise ValueError(f"Invalid translation for {code}: {source!r} -> {target!r}")
                cache.update(zip(batch, targets))
                hints.update((source, translation_source(source)) for source in batch)
                cache_path.write_text(json.dumps(cache, ensure_ascii=False, indent=2) + "\n")
                break
            except Exception:
                if retry == 4:
                    raise
                time.sleep(2 ** retry)
    for block, source in needed:
        target = cache[source]
        if sorted(TOKENS.findall(source)) != sorted(TOKENS.findall(target)):
            raise ValueError(f"Cached placeholder mismatch: {source}")
        translated = re.sub(r'^msgstr .*?(?=\n[^"\n]|\Z)', lambda _: 'msgstr ' + json.dumps(target, ensure_ascii=False), block, flags=re.M | re.S)
        translated = re.sub(r'^#,.*\bfuzzy\b.*\n', '', translated, flags=re.M)
        text = text.replace(block, translated, 1)
    path.write_text(text)
    return f"{code}: translated {len(needed)} messages"


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--write", action="store_true", required=True, help="Write missing translations to locale catalogs")
    parser.add_argument("--cache-dir", type=Path, default=Path("/tmp/postika-marketplace-translations"))
    parser.add_argument("--languages", nargs="+")
    parser.add_argument("--review-context", action="store_true", help="Refresh ambiguous ad/housing labels with contextual wording")
    args = parser.parse_args()
    args.cache_dir.mkdir(parents=True, exist_ok=True)
    codes = args.languages or re.findall(r"code: '([^']+)'", (ROOT / "src/locales/languages.js").read_text())
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        futures = {pool.submit(translate, code, args.cache_dir, args.review_context): code for code in codes if code != "en"}
        for future in concurrent.futures.as_completed(futures):
            print(future.result(), flush=True)
