"""从各域合同生成 docs/files.md、docs/api.md 的接口清单和 docs/workflows.md 的入口流程，保证人类文档与机器合同一致。"""

import argparse
import json
import os
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
KINDS = {'handwritten': '手写', 'ui': 'UI', 'generated': '生成', 'config': '配置', 'migration': '迁移'}
API_START, API_END = '<!-- openapi:start -->', '<!-- openapi:end -->'
FLOW_START, FLOW_END = '<!-- flows:start -->', '<!-- flows:end -->'
METHODS = ('get', 'post', 'put', 'patch', 'delete', 'head', 'options')


def cell(value):
    if isinstance(value, list):
        value = '；'.join(str(v) for v in value)
    return str(value).replace('|', '\\|').replace('\n', ' ')


def render_files(slug, impl):
    files = impl.get('files', [])
    counts = {}
    for f in files:
        counts[f['kind']] = counts.get(f['kind'], 0) + 1
    summary = '、'.join(KINDS.get(k, k) + ' ' + str(counts[k]) for k in KINDS if k in counts)
    lines = [
        '# ' + slug + ' 文件目录', '',
        '本文件由 `make docs` 从 [implementation.json](../contracts/implementation.json) 生成，不手改。登记入口、契约、配置、部署和生成文件的位置，其余源码按需要建立，不逐个登记。', '',
        '文件 ' + str(len(files)) + ' 个：' + summary + '。', '',
        '| 路径 | 类别 | 职责 |', '|---|---|---|',
    ]
    for f in files:
        lines.append('| `' + f['path'] + '` | ' + KINDS.get(f['kind'], f['kind']) + ' | ' + cell(f['purpose']) + ' |')
    return '\n'.join(lines) + '\n'


def schema_name(content):
    for media, body in (content or {}).items():
        schema = body.get('schema', {})
        if '$ref' in schema:
            return schema['$ref'].split('/')[-1]
        if schema.get('format') == 'binary':
            return media
        return schema.get('type', media)
    return '—'


def render_api(api, codes):
    lines = [
        '## 接口清单', '',
        '本节由 `make docs` 从 [openapi.json](../contracts/openapi.json) 生成，不手改；节外说明为手写。', '',
        '| 方法与路径 | operationId | 处理函数 | 权限 | 请求 → 成功响应 | 说明 | 错误 |', '|---|---|---|---|---|---|---|',
    ]
    for route, methods in api.get('paths', {}).items():
        for method in METHODS:
            op = methods.get(method)
            if not isinstance(op, dict):
                continue
            responses = op.get('responses', {})
            ok = next((c for c in responses if str(c).startswith('2')), None)
            result = (str(ok) + ' `' + schema_name(responses[ok].get('content')) + '`') if ok else '—'
            request = schema_name(op.get('requestBody', {}).get('content'))
            errors = []
            for status, body in responses.items():
                if str(status).startswith('2'):
                    continue
                found = [c for c in codes if re.search(r'(?<![A-Z_])' + c + r'(?![A-Z_])', body.get('description', ''))]
                errors.extend('`' + c + '`' for c in found or ['HTTP ' + str(status)])
            lines.append('| ' + method.upper() + ' `' + route + '` | `' + op.get('operationId', '') + '` | `' + str(op.get('x-handler')) + '` | `' + str(op.get('x-permission')) + '` | `'
                         + request + '` → ' + result + ' | ' + cell(op.get('summary', '')) + ' | ' + ('、'.join(errors) or '—') + ' |')
    return '\n'.join(lines) + '\n'


def merge_block(current, block, start, end):
    if current is None or start not in current or end not in current:
        return (current or '').rstrip() + '\n\n' + start + '\n' + block + end + '\n'
    before, rest = current.split(start, 1)
    after = rest.split(end, 1)[1]
    return before + start + '\n' + block + end + after


def merge_api(current, block):
    return merge_block(current, block, API_START, API_END)


def render_flows(impl, events):
    lines = ['## 入口与后台流程', '', '本节由 `make docs` 从 implementation.json 的 jobTypes、backgroundTasks 与 events.json 生成，不手改；节外为手写的状态机与规则。', '']
    consumed = [e for e in events.get('events', []) if e.get('direction') == 'consume']
    lines += ['### 事件处理', '']
    if consumed:
        lines += ['| 事件 | 版本 | 处理函数 | 作用 |', '|---|---|---|---|']
        lines += ['| `' + e['name'] + '` | ' + str(e.get('version')) + ' | `' + str(e.get('handler')) + '` | ' + cell(e.get('purpose', '')) + ' |' for e in consumed]
    else:
        lines.append('本域不消费事件。')
    lines += ['', '### 异步任务', '']
    if not impl.get('jobTypes'):
        lines.append('本域没有登记的异步任务类型。')
    for job in impl.get('jobTypes', []):
        lines += ['#### `' + job['type'] + '`', '', '- 执行函数：`' + job['executor'] + '`', '- 触发：' + job['trigger']]
        if job.get('schedule'):
            lines.append('- 系统定时：`' + job['schedule'] + '`')
        lines.append('')
        lines += [str(i) + '. ' + re.sub(r'^\d+[.、]?\s+', '', st) for i, st in enumerate(job.get('steps', []), 1)]
        lines.append('')
    lines += ['', '### 后台任务', '']
    if impl.get('backgroundTasks'):
        lines += ['| 函数 | 运行时机 | 作用 |', '|---|---|---|']
        lines += ['| `' + t['function'] + '` | ' + cell(t['schedule']) + ' | ' + cell(t.get('purpose', '')) + ' |' for t in impl['backgroundTasks']]
    else:
        lines.append('本域没有后台任务。')
    return re.sub(r'\n{3,}', '\n\n', '\n'.join(lines)) + '\n'


def expected_docs():
    catalog = json.loads((ROOT / 'contracts/modules.json').read_text(encoding='utf-8'))
    modules = catalog.get('modules', [])
    codes = json.loads((ROOT / 'contracts/errors.json').read_text(encoding='utf-8')).get('codes', [])
    result = {}
    for module in modules:
        folder = ROOT / module['root']
        impl = json.loads((folder / 'contracts/implementation.json').read_text(encoding='utf-8'))
        result[folder / 'docs/files.md'] = render_files(module['slug'], impl)
        api_path = folder / 'docs/api.md'
        api = json.loads((folder / 'contracts/openapi.json').read_text(encoding='utf-8'))
        result[api_path] = merge_api(api_path.read_text(encoding='utf-8') if api_path.exists() else None, render_api(api, codes))
        flow_path = folder / 'docs/workflows.md'
        events = json.loads((folder / 'contracts/events.json').read_text(encoding='utf-8'))
        result[flow_path] = merge_block(flow_path.read_text(encoding='utf-8') if flow_path.exists() else None, render_flows(impl, events), FLOW_START, FLOW_END)
    for component in catalog.get('shared', []):
        folder = ROOT / component['root']
        impl = json.loads((folder / 'contracts/implementation.json').read_text(encoding='utf-8'))
        result[folder / 'docs/files.md'] = render_files(component['slug'], impl)
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
