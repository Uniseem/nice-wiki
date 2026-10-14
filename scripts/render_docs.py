"""从各域 implementation.json 生成 docs/functions.md 与 docs/files.md，保证人类文档与机器合同一致。"""

import argparse
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
KINDS = {'handwritten': '手写', 'ui': 'UI', 'generated': '生成', 'config': '配置', 'migration': '迁移'}
LEVELS = {'binding': '约束', 'reference': '参考'}


def cell(value):
    if isinstance(value, list):
        value = '；'.join(str(v) for v in value)
    return str(value).replace('|', '\\|').replace('\n', ' ')


def text(value):
    if isinstance(value, dict):
        return '；'.join(str(k) + '：' + str(v) for k, v in value.items())
    if isinstance(value, list):
        return '；'.join(str(v) for v in value) if value else '无'
    return str(value)


def functions_of(impl):
    return [(f, fn) for f in impl.get('files', []) for fn in f.get('functions', [])]


def render_function(file, fn):
    lang = 'go' if fn['signature'].startswith('func ') else 'ts'
    lines = ['## ' + fn['id'], '', '- 级别：' + LEVELS.get(fn.get('contract'), '未标注'), '- 文件：`' + file['path'] + '`', '- 作用：' + fn['purpose'], '', '```' + lang, fn['signature'], '```', '']
    for title, group in (('输入', 'inputs'), ('输出', 'outputs')):
        lines += ['**' + title + '**', '']
        if fn[group]:
            lines += ['| 名称 | 类型 | 说明 | 约束 |', '|---|---|---|---|']
            lines += ['| `' + p['name'] + '` | `' + cell(p['type']) + '` | ' + cell(p['description']) + ' | ' + cell(p['constraints']) + ' |' for p in fn[group]]
        else:
            lines.append('无')
        lines.append('')
    lines += ['**步骤**', '']
    lines += [str(i) + '. ' + re.sub(r'^\d+[.、]?\s+', '', step) for i, step in enumerate(fn['steps'], 1)]
    lines += ['', '**错误**', '']
    for error in fn['errors'] or ['无']:
        if isinstance(error, dict):
            status = ' / HTTP ' + str(error['httpStatus']) if 'httpStatus' in error else ''
            retry = '可重试' if error.get('retryable') else '不可重试'
            extra = '；' + error['rollback'] if error.get('rollback') else ''
            lines.append('- `' + error['code'] + '`' + status + '：' + str(error.get('when') or error.get('condition', '')) + '（' + retry + '）' + extra)
        else:
            lines.append('- ' + str(error))
    deps = '、'.join('`' + d + '`' for d in fn['dependencies']) or '无'
    lines += ['', '- 副作用：' + text(fn['sideEffects']), '- 授权：' + text(fn['authorization']), '- 并发：' + text(fn['concurrency']), '- 幂等：' + text(fn['idempotency']), '- 依赖：' + deps]
    if fn.get('resources'):
        lines.append('- 其他依赖说明：' + text(fn['resources']))
    return '\n'.join(lines) + '\n'


def render_functions(slug, impl):
    items = functions_of(impl)
    binding = sum(1 for _, fn in items if fn.get('contract') == 'binding')
    head = [
        '# ' + slug + ' 函数合同', '',
        '本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改；先改合同再重新生成。类型字段见 [types.json](../contracts/types.json)，公共类型见 [common.schema.json](../../../contracts/common.schema.json)。', '',
        '共 ' + str(len(items)) + ' 个函数：约束 ' + str(binding) + ' 个，参考 ' + str(len(items) - binding) + ' 个。约束函数（HTTP 处理、事件转发/订阅/处理、任务执行与启动入口）按合同的名称、签名和行为实现；参考函数是建议拆分，实现时可增删改名，模块交付时同步合同。', '',
    ]
    return '\n'.join(head) + '\n' + '\n'.join(render_function(f, fn) for f, fn in items)


def render_files(slug, impl):
    files = impl.get('files', [])
    counts = {}
    for f in files:
        counts[f['kind']] = counts.get(f['kind'], 0) + 1
    items = functions_of(impl)
    binding = sum(1 for _, fn in items if fn.get('contract') == 'binding')
    summary = '、'.join(KINDS.get(k, k) + ' ' + str(counts[k]) for k in KINDS if k in counts)
    lines = [
        '# ' + slug + ' 未来文件目录', '',
        '本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。仅登记未来实现位置，尚未创建这些源码；函数行为见 [functions.md](functions.md)。', '',
        '文件 ' + str(len(files)) + ' 个：' + summary + '。函数 ' + str(len(items)) + ' 个：约束 ' + str(binding) + '，参考 ' + str(len(items) - binding) + '。入口、契约、配置和部署文件按清单建立；internal/ 与界面 src/ 下的拆分属于参考，实现时可调整并在交付时同步合同。', '',
        '| 未来路径 | 类别 | 职责 | 函数 |', '|---|---|---|---|',
    ]
    for f in files:
        names = '、'.join('`' + fn['id'] + '`' + ('' if fn.get('contract') == 'binding' else '（参考）') for fn in f.get('functions', [])) or '无'
        lines.append('| `' + f['path'] + '` | ' + KINDS.get(f['kind'], f['kind']) + ' | ' + cell(f['purpose']) + ' | ' + names + ' |')
    return '\n'.join(lines) + '\n'


def expected_docs():
    modules = json.loads((ROOT / 'contracts/modules.json').read_text(encoding='utf-8')).get('modules', [])
    result = {}
    for module in modules:
        folder = ROOT / module['root']
        impl = json.loads((folder / 'contracts/implementation.json').read_text(encoding='utf-8'))
        result[folder / 'docs/functions.md'] = render_functions(module['slug'], impl)
        result[folder / 'docs/files.md'] = render_files(module['slug'], impl)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--check', action='store_true', help='只报告过期文档，不写入')
    args = parser.parse_args()
    stale = []
    for path, content in expected_docs().items():
        current = path.read_text(encoding='utf-8') if path.exists() else None
        if current != content:
            stale.append(str(path.relative_to(ROOT)))
            if not args.check:
                path.write_text(content, encoding='utf-8')
    for name in stale:
        print(('过期 ' if args.check else '已生成 ') + name)
    return 1 if args.check and stale else 0


if __name__ == '__main__':
    raise SystemExit(main())
