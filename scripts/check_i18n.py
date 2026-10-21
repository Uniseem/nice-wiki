"""后台界面译文检查：源码里的中文都要包在 t('…') 里，每条都要有 14 种语言的译文，占位符一致，不留没人用的译文。

用法：python3 scripts/check_i18n.py            检查（make check 调用）
      python3 scripts/check_i18n.py --missing  按模块列出缺译文的中文原文（JSON），供补译文
"""

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
LANGUAGES = ['zh-Hant', 'en', 'ja', 'ko', 'fr', 'de', 'es', 'pt', 'ru', 'it', 'vi', 'th', 'id', 'ar']
CJK = re.compile(r'[　-〿㐀-鿿＀-￯]')
PLACEHOLDER = re.compile(r'\{\d+\}')
# 公开页交互岛有自己的界面文字机制（服务端下发），不在后台译文范围内；语言清单里的本族语名称原样显示。
EXCLUDED = ('modules/public-site/web/', 'modules/qa/web/src/island/', '/gen/', 'libs/platform/web/ui/languages.ts')


def admin_sources():
    files = sorted(ROOT.glob('libs/platform/web/ui/**/*.ts*')) + sorted(ROOT.glob('modules/*/web/src/**/*.ts*'))
    return [path for path in files if not any(part in path.as_posix() for part in EXCLUDED)]


def owner(path):
    """译文放在哪个文件：模块源码用模块的 web/i18n.json，共用件用 libs/platform/web/i18n.json。"""
    relative = path.relative_to(ROOT).as_posix()
    if relative.startswith('modules/'):
        return ROOT / 'modules' / relative.split('/')[1] / 'web' / 'i18n.json'
    return ROOT / 'libs/platform/web/i18n.json'


def unescape(text):
    return re.sub(r'\\(.)', lambda m: {'n': '\n', 't': '\t'}.get(m.group(1), m.group(1)), text)


def scan(path):
    """返回 (t() 的中文原文列表, 没包 t() 的中文位置列表)。按 TS 词法跳过注释，正则字面量按常见写法识别。"""
    text = path.read_text(encoding='utf-8')
    lines = text.split('\n')
    keys, raw = [], []

    def line_of(index):
        return text.count('\n', 0, index)

    def ignored(line):
        return 'i18n-ignore' in lines[line] or (line > 0 and 'i18n-ignore' in lines[line - 1]) or 'console.' in lines[line]

    def report(start, end):
        line = line_of(start)
        if not ignored(line):
            raw.append((line + 1, text[start:end].strip()[:60]))

    i, n = 0, len(text)
    previous = ''
    while i < n:
        c = text[i]
        if text.startswith('//', i):
            i = text.find('\n', i)
            i = n if i < 0 else i
            continue
        if text.startswith('/*', i):
            end = text.find('*/', i + 2)
            i = n if end < 0 else end + 2
            continue
        if c in '\'"`':
            start = i
            i += 1
            depth = 0
            literal, inner = [], []
            while i < n:
                if text[i] == '\\':
                    (inner if depth else literal).append(text[i:i + 2])
                    i += 2
                    continue
                if c == '`' and text.startswith('${', i):
                    depth += 1
                    i += 2
                    continue
                if c == '`' and depth and text[i] == '}':
                    depth -= 1
                    i += 1
                    continue
                if text[i] == c and not depth:
                    break
                (inner if depth else literal).append(text[i])
                i += 1
            literal = ''.join(literal)
            # 模板里 ${…} 中的 t('…') 也算
            keys.extend(unescape(m.group(1)) for m in re.finditer(r"(?<![\w.$])t\(\s*'((?:[^'\\\n]|\\.)*)'", ''.join(inner)))
            i += 1
            if CJK.search(literal):
                before = text[max(0, start - 40):start].rstrip()
                if c != '`' and re.search(r'(?<![\w.$])t\($', before):
                    keys.append(unescape(literal))
                elif c != '`' and line_of(start) < len(lines) and 'i18n-key' in lines[line_of(start)]:
                    keys.append(unescape(literal))
                else:
                    report(start, i)
            previous = c
            continue
        if c == '/' and previous in '(,=:[!&|?{};' + '\n':
            # 正则字面量
            i += 1
            in_class = False
            while i < n and text[i] != '\n':
                if text[i] == '\\':
                    i += 2
                    continue
                if text[i] == '[':
                    in_class = True
                elif text[i] == ']':
                    in_class = False
                elif text[i] == '/' and not in_class:
                    break
                i += 1
            i += 1
            previous = '/'
            continue
        if CJK.match(c):
            start = i
            while i < n and text[i] not in '<{\n':
                i += 1
            report(start, i)
            continue
        if not c.isspace():
            previous = c
        i += 1
    return keys, raw


def catalogs():
    files = [ROOT / 'libs/platform/web/i18n.json'] + sorted(ROOT.glob('modules/*/web/i18n.json'))
    return {path: json.loads(path.read_text(encoding='utf-8')) for path in files if path.exists()}


def dynamic_keys():
    """不在源码字面量里、运行时才传给 t() 的中文：后台菜单名来自 contracts/permission-groups.json。"""
    groups = json.loads((ROOT / 'contracts/permission-groups.json').read_text(encoding='utf-8'))
    return {(ROOT / 'modules/portal/web/i18n.json'): [route['labelZH'] for route in groups['adminRoutes']]}


def main():
    used = {}
    issues = []
    for path in admin_sources():
        keys, raw = scan(path)
        for line, snippet in raw:
            issues.append(f'{path.relative_to(ROOT)}:{line}: 中文没有包在 t() 里：{snippet}')
        if keys:
            used.setdefault(owner(path), set()).update(keys)
    for path, keys in dynamic_keys().items():
        used.setdefault(path, set()).update(keys)
    loaded = catalogs()
    if '--missing' in sys.argv:
        missing = {}
        for path, keys in used.items():
            entries = loaded.get(path, {})
            todo = sorted(key for key in keys if any(not entries.get(key, {}).get(language) for language in LANGUAGES))
            if todo:
                missing[str(path.relative_to(ROOT))] = todo
        print(json.dumps(missing, ensure_ascii=False, indent=1))
        return
    for path in set(used) | set(loaded):
        entries = loaded.get(path, {})
        relative = path.relative_to(ROOT)
        for key in sorted(used.get(path, set())):
            translations = entries.get(key)
            if not translations:
                issues.append(f'{relative}: 缺译文：{key}')
                continue
            for language in LANGUAGES:
                value = translations.get(language, '')
                if not value.strip():
                    issues.append(f'{relative}: {language} 缺译文：{key}')
                elif sorted(PLACEHOLDER.findall(value)) != sorted(PLACEHOLDER.findall(key)):
                    issues.append(f'{relative}: {language} 占位符和原文不一致：{key} → {value}')
            extra = set(translations) - set(LANGUAGES)
            if extra:
                issues.append(f'{relative}: 未知语言 {sorted(extra)}：{key}')
        for key in sorted(set(entries) - used.get(path, set())):
            issues.append(f'{relative}: 没人用的译文：{key}')
    for issue in issues[:200]:
        print(issue)
    if issues:
        print(f'后台译文检查失败：{len(issues)} 项')
        sys.exit(1)
    print('后台译文检查通过')


if __name__ == '__main__':
    main()
