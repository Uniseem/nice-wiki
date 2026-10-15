"""设计阶段的结构、引用与隐私校验，不运行或模拟业务。"""

import argparse
import ast
import hashlib
import ipaddress
import json
import re
import subprocess
import sys
from pathlib import Path
from urllib.parse import unquote, urlsplit

ROOT = Path(__file__).resolve().parents[1]
ISSUES = []
CACHE = {}
BUILTINS = {'string', 'bool', 'int', 'int32', 'int64', 'uint64', 'float64', 'byte', 'bytes', 'error', 'void', 'number', 'boolean', 'Context', 'Reader', 'Writer', 'ResponseWriter', 'Request', 'Time', 'Promise', 'Record', 'Map', 'Set', 'Uint8Array', 'AbortSignal', 'HTMLElement', 'Event', 'KeyboardEvent', 'MouseEvent', 'File', 'Blob', 'unknown'}


def fail(label, message):
    ISSUES.append(str(label) + ': ' + message)


def relative(path):
    return str(path.relative_to(ROOT)).replace('\\', '/')


def load_json(path):
    path = path.resolve()
    if path not in CACHE:
        try:
            CACHE[path] = json.loads(path.read_text(encoding='utf-8'))
        except (ValueError, OSError) as exc:
            fail(relative(path), 'JSON读取失败: ' + type(exc).__name__)
            CACHE[path] = {}
    return CACHE[path]


def walk_nodes(value):
    if isinstance(value, dict):
        yield value
        for item in value.values():
            yield from walk_nodes(item)
    elif isinstance(value, list):
        for item in value:
            yield from walk_nodes(item)


def check_ref(base, target):
    file_part, _, pointer = target.partition('#')
    if file_part.startswith(('https://', 'http://')):
        paths = [p for p, data in CACHE.items() if data.get('$id') == file_part]
        if not paths:
            fail(relative(base), '未登记远程schema引用 ' + target)
            return
        path = paths[0]
    else:
        path = (base.parent / unquote(file_part)).resolve() if file_part else base
    if not path.is_relative_to(ROOT) or not path.is_file():
        fail(relative(base), '不存在或越界schema引用 ' + target)
        return
    value = load_json(path)
    try:
        for part in pointer.lstrip('/').split('/') if pointer else []:
            key = part.replace('~1', '/').replace('~0', '~')
            value = value[int(key)] if isinstance(value, list) else value[key]
    except (KeyError, IndexError, ValueError, TypeError):
        fail(relative(base), '不存在JSON指针 ' + target)


def scan_privacy(label, text):
    for address in re.findall(r'[A-Za-z0-9_.+%-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}', text):
        domain = address.split('@')[1].lower()
        if domain not in ('users.noreply.github.com', 'example.com', 'example.org', 'example.net') and address not in ('noreply@openai.com', 'noreply@anthropic.com', 'noreply@z.ai'):
            fail(label, '不允许的邮箱，具体值不输出')
    if re.search(r'(?:[A-Za-z]:[\\/]Users[\\/][A-Za-z0-9_.-]+|(?:^|[\s\"\x27])/(?:Users|home)/[A-Za-z0-9_.-]+)', text):
        fail(label, '本机用户路径')
    if re.search(r'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----|gh[pousr]_[A-Za-z0-9]{20,}|sk-[A-Za-z0-9]{24,}', text):
        fail(label, '疑似密钥，具体值不输出')
    networks = [ipaddress.ip_network(n) for n in ('192.0.2.0/24', '198.51.100.0/24', '203.0.113.0/24')]
    for value in re.findall(r'(?<![\w.])(?:\d{1,3}\.){3}\d{1,3}(?![\w.])', text):
        try:
            address = ipaddress.ip_address(value)
        except ValueError:
            continue
        if not address.is_loopback and not address.is_unspecified and not any(address in n for n in networks):
            fail(label, '非文档IPv4地址，具体值不输出')


def check_links(path):
    text = path.read_text(encoding='utf-8')
    text = re.sub(r'(?ms)^[ \t]*(`{3,}|~{3,})[^\n]*\n.*?^[ \t]*\1[ \t]*$', '', text)
    text = re.sub(r'(`+)[^`]*?\1', '', text)
    for target in re.findall(r'\[[^\]]*\]\(([^)]+)\)', text):
        target = target.split(' "')[0].strip('<>')
        if urlsplit(target).scheme or target.startswith('#'):
            continue
        file_part = unquote(target.split('#')[0])
        destination = (path.parent / file_part).resolve()
        if not destination.is_relative_to(ROOT) or not destination.exists():
            fail(relative(path), '文档链接不存在 ' + target)


def check_modules(modules, permissions, codes, shared=()):
    functions = {}
    operations = {}
    planned = set()
    shared_slugs = {c['slug'] for c in shared}
    count = {'modules': len(modules), 'shared': len(shared), 'files': 0, 'functions': 0, 'operations': 0, 'types': 0}
    for module in list(modules) + list(shared):
        slug = module['slug']
        service = slug not in shared_slugs
        folder = ROOT / module['root']
        if service:
            required = ['README.md', 'AGENTS.md', *['docs/' + n + '.md' for n in ('requirements', 'architecture', 'data', 'api', 'workflows', 'files', 'functions', 'operations', 'acceptance')], *['contracts/' + n + '.json' for n in ('types', 'implementation', 'openapi', 'events')]]
        else:
            required = ['README.md', 'progress.json', 'docs/files.md', 'docs/functions.md', 'contracts/implementation.json']
        for name in required:
            if not (folder / name).is_file():
                fail(slug, '缺少模块文件 ' + name)
        impl = load_json(folder / 'contracts/implementation.json')
        types = load_json(folder / 'contracts/types.json').get('$defs', {}) if (folder / 'contracts/types.json').is_file() else {}
        common = load_json(ROOT / 'contracts/common.schema.json').get('$defs', {})
        count['types'] += len(types)
        if impl.get('module') != slug or impl.get('phase') != 'design':
            fail(slug, '模块标识或设计阶段不匹配')
        local = {}
        for file in impl.get('files', []):
            path = file.get('path', '')
            if not path.startswith(module['root'] + '/') or '..' in Path(path).parts:
                fail(slug, '未来文件不在本域 ' + path)
            if path in planned:
                fail(slug, '重复未来文件 ' + path)
            planned.add(path)
            count['files'] += 1
            if not file.get('purpose'):
                fail(path, '缺文件职责')
            for fn in file.get('functions', []):
                fid = fn.get('id', '')
                if fid in functions:
                    fail(slug, '重复函数ID ' + fid)
                functions[fid] = (slug, fn)
                local[fid] = fn
                count['functions'] += 1
                fields = ('id', 'name', 'signature', 'inputs', 'outputs', 'purpose', 'steps', 'errors', 'sideEffects', 'authorization', 'concurrency', 'idempotency', 'dependencies', 'contract')
                for key in fields:
                    if key not in fn:
                        fail(fid, '缺函数字段 ' + key)
                if not fn.get('steps') or not fn.get('purpose') or not fn.get('signature'):
                    fail(fid, '缺精确作用/签名/步骤')
                if fid != slug + '.' + fn.get('name', ''):
                    fail(fid, '函数ID必须为 模块slug.函数名')
                if fn.get('contract') not in ('binding', 'reference'):
                    fail(fid, '合同级别只能是binding或reference')
                if not all(isinstance(r, str) and r for r in fn.get('resources', [])):
                    fail(fid, 'resources只能是非空说明文字')
                signature = fn.get('signature', '')
                if 'RequestContext' in signature:
                    fail(fid, '签名不得使用RequestContext')
                if service and signature.startswith('func ') and 'OperationContext' in signature and '(ctx context.Context, op OperationContext' not in signature:
                    fail(fid, 'Go操作函数签名必须以(ctx context.Context, op OperationContext开头')
                if service and not signature.startswith('func ') and 'OperationContext' in signature:
                    fail(fid, '浏览器函数不得接收服务端OperationContext')
                for group in ('inputs', 'outputs'):
                    for param in fn.get(group, []):
                        if not all(key in param for key in ('name', 'type', 'description', 'constraints')):
                            fail(fid, '输入输出字段不完整')
                        type_names = re.findall(r'[A-Za-z_][A-Za-z0-9_]*', param.get('type', ''))
                        for name in type_names:
                            if name not in types and name not in common and name not in BUILTINS and name not in ('context', 'io', 'http', 'time', 'map'):
                                fail(fid, '未登记类型 ' + name)
                for error in fn.get('errors', []):
                    code = error.get('code') if isinstance(error, dict) else error
                    if code not in codes:
                        fail(fid, '未登记错误码 ' + str(code))
        if not service:
            continue
        api = load_json(folder / 'contracts/openapi.json')
        if api.get('openapi') != '3.1.0':
            fail(slug, 'OpenAPI版本不是3.1.0')
        for route, methods in api.get('paths', {}).items():
            for method, op in methods.items():
                if method.lower() not in ('get', 'post', 'put', 'patch', 'delete', 'head', 'options'):
                    continue
                oid = op.get('operationId', '')
                key = slug + ':' + oid
                if not oid or key in operations:
                    fail(slug, '缺少或重复operationId ' + route)
                operations[key] = op
                count['operations'] += 1
                if op.get('x-handler') not in local:
                    fail(key, '未登记处理函数 ' + str(op.get('x-handler')))
                elif local[op.get('x-handler')].get('contract') != 'binding':
                    fail(key, 'HTTP处理函数必须是binding级别')
                if op.get('x-permission') not in permissions:
                    fail(key, '未登记权限 ' + str(op.get('x-permission')))
                declared = {p.get('name') for p in op.get('parameters', []) if p.get('in') == 'path'}
                for parameter in re.findall(r'\{([^}]+)\}', route):
                    if parameter not in declared:
                        fail(key, '缺路径参数 ' + parameter)
                if not op.get('responses'):
                    fail(key, '缺响应契约')
        for dep in impl.get('dependencies', []):
            if dep.get('module') not in {m['slug'] for m in modules}:
                fail(slug, '依赖不存在模块 ' + str(dep.get('module')))
    for fid, (slug, fn) in functions.items():
        for dep in fn.get('dependencies', []):
            if ':' in dep and dep.split(':', 1)[0] in {m['slug'] for m in modules}:
                if dep not in operations:
                    fail(fid, '跨域operation不存在 ' + dep)
            elif dep in functions and functions[dep][0] != slug and functions[dep][0] not in shared_slugs:
                fail(fid, '直接依赖另域内部函数 ' + dep)
            elif dep not in functions:
                fail(fid, '依赖无法解析，须为本域函数ID或 模块:operationId ' + dep)
    return count


def check_wiring(modules, shared=()):
    groups = load_json(ROOT / 'contracts/permission-groups.json')
    definitions = load_json(ROOT / 'contracts/common.schema.json').get('$defs', {})
    locales = groups.get('interfaceLocales', [])
    routes = groups.get('adminRoutes', [])
    if locales != definitions.get('UILocale', {}).get('enum') or len(locales) != len(set(locales)):
        fail('后台偏好', '界面语言目录与公共schema不一致或重复')
    keys = [row.get('key') for row in routes]
    paths = [row.get('path') for row in routes]
    if keys != definitions.get('AdminRouteKey', {}).get('enum') or len(keys) != len(set(keys)) or len(paths) != len(set(paths)):
        fail('后台偏好', '页面目录与公共schema不一致或重复')
    permissions = set(load_json(ROOT / 'contracts/permissions.json').get('permissions', [])) - {'anonymous', 'service'}
    for row in routes:
        grants = row.get('permissions', [])
        if not grants or len(grants) != len(set(grants)) or not set(grants) <= permissions:
            fail('后台偏好', '页面权限必须为已登记且不重复的非空权限集合')
        if not re.fullmatch(r'/admin/[a-z-]+', row.get('path', '')) or not row.get('labelZH') or not row.get('labelEN') or not isinstance(row.get('navigation'), bool):
            fail('后台偏好', '页面路径、标签或导航声明不合法')
    functions, files, roots, ops, callers = {}, {}, set(), {}, set()
    permissions = set(load_json(ROOT / "contracts/permissions.json").get("permissions", []))
    management = load_json(ROOT / "contracts/permission-groups.json").get("jobManagementGrants", [])
    if not isinstance(management, list) or not management or len(management) != len(set(management)) or any(p not in permissions or p in ("anonymous", "service", "job.manage") for p in management):
        fail("任务管理权限", "jobManagementGrants须为非空、不重复的已登记具体业务权限")
    management = set(management)
    used_management = set()
    for component in shared:
        for f in load_json(ROOT / component['root'] / 'contracts/implementation.json').get('files', []):
            for fn in f.get('functions', []):
                functions[fn.get('id')] = fn
                files[fn.get('id')] = f.get('path', '')
    roots.add('platform.main')
    for module in modules:
        folder = ROOT / module['root']
        impl = load_json(folder / 'contracts/implementation.json')
        for f in impl.get('files', []):
            for fn in f.get('functions', []):
                functions[fn.get('id')] = fn
                files[fn.get('id')] = f.get('path', '')
        api = load_json(folder / 'contracts/openapi.json')
        for route, methods in api.get('paths', {}).items():
            for method, op in methods.items():
                if isinstance(op, dict) and op.get('operationId'):
                    key = module['slug'] + ':' + op['operationId']
                    ops[key] = op
                    roots.add(op.get('x-handler'))
                    if op.get('x-admission') and op.get('x-permission') != 'anonymous':
                        fail(key, 'x-admission只能标在匿名公开接口')
        for event in load_json(folder / 'contracts/events.json').get('events', []):
            if event.get('handler'):
                roots.add(event['handler'])
        slug = module['slug']
        roots.add(slug + '.main')
        local = {fn.get('id') for f in impl.get('files', []) for fn in f.get('functions', [])}
        job_types = impl.get('jobTypes')
        background = impl.get('backgroundTasks')
        if not isinstance(job_types, list) or not isinstance(background, list):
            fail(slug, 'implementation缺jobTypes或backgroundTasks清单')
            continue
        worker = slug + '.RunWorker'
        for event in load_json(folder / 'contracts/events.json').get('events', []):
            if event.get('direction') == 'publish':
                permission = load_json(ROOT / 'contracts/permission-groups.json').get('eventManagementPermissions', {}).get(slug)
                if permission not in management:
                    fail(slug, '事件生产方缺少已登记的具体管理权限')
                else:
                    used_management.add(permission)
        for job in job_types:
            permission = job.get("managementPermission")
            if permission not in management:
                fail(slug, "任务缺少已登记的具体managementPermission " + str(job.get("type")))
            else:
                used_management.add(permission)
            if not str(job.get('type', '')).startswith(slug + '.') or job.get('executor') not in local or not job.get('trigger'):
                fail(slug, 'jobTypes条目不合法 ' + str(job.get('type')))
            elif worker not in local or job['executor'] not in functions[worker].get('dependencies', []):
                fail(slug, '任务执行函数未挂在本域RunWorker ' + job['executor'])
            elif functions[job['executor']].get('contract') != 'binding':
                fail(job['executor'], '任务执行函数必须是binding级别')
        build = functions.get(slug + '.BuildApplication', {})
        for task in background:
            if task.get('function') not in local or not task.get('schedule'):
                fail(slug, 'backgroundTasks条目不合法 ' + str(task.get('function')))
            elif task['function'] not in build.get('dependencies', []):
                fail(slug, '后台任务未挂在BuildApplication ' + task['function'])
        if any(j.get('schedule') for j in job_types) and 'jobs:createSchedule' not in build.get('dependencies', []):
            fail(slug, '有系统定时任务但BuildApplication未登记jobs:createSchedule')
    if used_management != management:
        fail("任务管理权限", "jobManagementGrants必须与任务类型及事件生产方的具体管理权限集合一致")
    for fn in functions.values():
        callers.update(d for d in fn.get('dependencies', []) if ':' in d)
    seen, stack = set(), list(roots)
    while stack:
        fid = stack.pop()
        if fid in seen or fid not in functions:
            continue
        seen.add(fid)
        for dep in functions[fid].get('dependencies', []):
            stack.append(ops.get(dep, {}).get('x-handler') if ':' in dep else dep)
    for fid in functions:
        if fid not in seen and not files[fid].endswith(('.ts', '.vue')):
            fail(fid, '从任何入口都不可达（接口处理、事件处理、启动入口）')
    generic = ('acquireBarrier', 'releaseBarrier', 'exportSnapshot', 'stageSnapshot', 'discardSnapshot', 'health', 'ready')
    for key, op in ops.items():
        if op.get('x-permission') == 'service' and key.split(':', 1)[1] not in generic and key not in callers:
            fail(key, 'service接口没有任何调用方')


def check_identity():
    try:
        email = subprocess.check_output(['git', 'config', '--get', 'user.email'], cwd=ROOT).decode().strip()
        name = subprocess.check_output(['git', 'config', '--get', 'user.name'], cwd=ROOT).decode().strip()
    except subprocess.CalledProcessError:
        fail('git', '没有提交身份')
        return
    if not re.fullmatch(r'\d+\+[A-Za-z0-9-]+@users\.noreply\.github\.com', email):
        fail('git', '作者邮箱不是GitHub代用地址')
    scan_privacy('作者和提交者', name + ' <' + email + '>')


def check_events(modules):
    producers = {}
    consumers = []
    for module in modules:
        path = ROOT / module['root'] / 'contracts/events.json'
        data = load_json(path)
        impl = load_json(ROOT / module['root'] / 'contracts/implementation.json')
        local = {fn.get('id'): fn for f in impl.get('files', []) for fn in f.get('functions', [])}
        deps = {d for fn in local.values() for d in fn.get('dependencies', [])}
        directions = {event.get('direction') for event in data.get('events', [])}
        if 'consume' in directions and not any({'jobs:subscribeEvents', 'jobs:acknowledgeEvent'} <= set(fn.get('dependencies', [])) for fn in local.values()):
            fail(module['slug'], '消费事件但没有调用jobs订阅与确认的订阅循环')
        if module['slug'] != 'jobs' and 'publish' in directions and 'jobs:publishEvent' not in deps:
            fail(module['slug'], '生产事件但没有把outbox转交jobs的转发函数')
        if 'jobs:publishEvent' in deps and 'publish' not in directions:
            fail(module['slug'], '调用jobs:publishEvent但未在events.json登记生产事件')
        for event in data.get('events', []):
            if event.get('direction') == 'consume':
                handler = local.get(event.get('handler'))
                if not handler:
                    fail(module['slug'] + ':' + str(event.get('name')), '消费事件缺本域处理函数handler')
                elif handler.get('contract') != 'binding':
                    fail(module['slug'] + ':' + str(event.get('name')), '事件处理函数必须是binding级别')
            key = (event.get('name'), event.get('version'))
            label = module['slug'] + ':' + str(key[0])
            for field in ('name', 'version', 'direction', 'payloadSchema', 'purpose', 'delivery', 'ordering', 'idempotency'):
                if field not in event:
                    fail(label, '缺事件字段 ' + field)
            if event.get('delivery') != 'at-least-once':
                fail(label, '未登记事件交付语义')
            ref = event.get('payloadSchema', {}).get('$ref', '')
            file_part, _, pointer = ref.partition('#')
            target = ((path.parent / file_part).resolve(), pointer)
            if event.get('direction') == 'publish':
                if key in producers:
                    fail(label, '事件有多个生产方')
                producers[key] = (label, target)
            elif event.get('direction') == 'consume':
                consumers.append((key, label, target))
            else:
                fail(label, '事件方向不合法')
    for key, label, target in consumers:
        if key not in producers:
            fail(label, '消费事件没有生产方')
        elif producers[key][1] != target:
            fail(label, '消费载荷未引用唯一生产方schema')


def check_rendered():
    sys.path.insert(0, str(ROOT / 'scripts'))
    import render_docs
    for path, content in render_docs.expected_docs().items():
        if not path.is_file() or path.read_text(encoding='utf-8') != content:
            fail(relative(path), '与implementation.json生成结果不一致，运行make docs')


def check_framework():
    path = ROOT / 'docs/structure/framework.json'
    data = load_json(path)
    declared = {item.get('path', '') for item in data.get('files', [])}
    actual = {relative(p) for top in ('docs', 'contracts', 'modules', 'scripts', 'libs', 'deploy') if (ROOT / top).is_dir() for p in (ROOT / top).rglob('*') if p.is_file() and p.suffix != '.pyc'}
    actual.update(n for n in ('AGENTS.md', 'CLAUDE.md', 'README.md', 'STATUS.md', 'THIRD_PARTY_NOTICES.md', 'LICENSE', 'Makefile', '.gitignore', '.gitattributes', '.editorconfig') if (ROOT / n).is_file())
    for extra in sorted(actual - declared):
        fail('实际结构', '未登记文件 ' + extra)
    for missing in sorted(declared - actual):
        fail('实际结构', '登记文件不存在 ' + missing)


GO_SUM_LINE = re.compile(r'^\S+ v\S+ h1:[A-Za-z0-9+/]{43}=$')


def check_go_modules():
    """go.mod 有依赖时必须有工具生成的 go.sum；校验行格式，手写或空文件直接拒绝。"""
    for gomod in sorted(list((ROOT / 'modules').glob('*/go.mod')) + list((ROOT / 'libs').glob('*/go.mod'))):
        label = relative(gomod)
        text = gomod.read_text(encoding='utf-8')
        if not re.search(r'^require\b', text, re.M):
            continue
        gosum = gomod.with_name('go.sum')
        lines = gosum.read_text(encoding='utf-8').splitlines() if gosum.is_file() else []
        if not lines:
            fail(label, '有依赖但缺少 go.sum；只能由 go mod tidy 生成，不得手写')
            continue
        for number, line in enumerate(lines, 1):
            if not GO_SUM_LINE.match(line):
                fail(relative(gosum), f'第{number}行不是 go 工具生成的校验和格式')
                break


def check_staged():
    try:
        names = subprocess.check_output(['git', 'diff', '--cached', '--name-only', '--diff-filter=ACMR', '-z'], cwd=ROOT).decode().split('\0')
        for name in filter(None, names):
            if name == 'vps.txt' or name.startswith(('.env', '.local/', 'data/', '.cache/')):
                fail('暂存区', '包含私有配置或运行目录')
                continue
            raw = subprocess.check_output(['git', 'show', ':' + name], cwd=ROOT)
            try:
                scan_privacy('暂存:' + name, raw.decode('utf-8'))
            except UnicodeDecodeError:
                fail('暂存:' + name, '设计阶段不允许未审阅二进制')
    except subprocess.CalledProcessError:
        fail('暂存区', 'Git暂存扫描失败')


def check_progress(modules):
    states = {'planned', 'in_progress', 'implemented', 'verified', 'accepted', 'blocked'}
    for module in modules:
        folder = ROOT / module['root']
        impl = load_json(folder / 'contracts/implementation.json')
        ledger = load_json(folder / 'progress.json')
        if ledger.get('module') != module['slug']:
            fail(module['slug'], '交接台账模块不匹配')
        digest = hashlib.sha256(json.dumps(impl, ensure_ascii=False, sort_keys=True, separators=(',', ':')).encode('utf-8')).hexdigest()
        if ledger.get('contractSha256') != digest:
            fail(module['slug'], '交接台账契约摘要过期')
        for name in (() if module.get('kind') else ('STATUS.md', 'docs/handoff.md')):
            if not (folder / name).is_file():
                fail(module['slug'], '缺少交接记录 ' + name)
        expected_files = {f['path'] for f in impl.get('files', [])}
        expected_functions = {fn['id'] for f in impl.get('files', []) for fn in f.get('functions', [])}
        for group, key, expected in (('files', 'path', expected_files), ('functions', 'id', expected_functions)):
            rows = ledger.get(group, [])
            actual = {row.get(key) for row in rows}
            if actual != expected or len(rows) != len(actual):
                fail(module['slug'], '台账未覆盖唯一逐项' + group)
            for row in rows:
                label = str(row.get(key))
                state = row.get('state')
                if state not in states:
                    fail(label, '未知进度状态')
                if state in ('implemented', 'verified', 'accepted') and not re.fullmatch(r'[0-9a-f]{40}', row.get('implementationCommit') or ''):
                    fail(label, '已实现状态缺对应完整提交SHA')
                verification = row.get('verification', {})
                if state in ('verified', 'accepted') and (verification.get('state') != 'passed' or not verification.get('evidence')):
                    fail(label, '已验证状态缺通过证据')
                if state == 'blocked' and not row.get('blocker'):
                    fail(label, '阻塞状态缺具体原因')
                if ledger.get('phase') == 'design' and state != 'planned':
                    fail(label, '设计阶段不能冒充业务实现')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--privacy-only', action='store_true')
    parser.add_argument('--summary', action='store_true')
    parser.add_argument('--message-file')
    args = parser.parse_args()
    modules = load_json(ROOT / 'contracts/modules.json').get('modules', [])
    shared = load_json(ROOT / 'contracts/modules.json').get('shared', [])
    files = []
    for top in ('docs', 'contracts', 'modules', 'scripts', 'libs', 'deploy'):
        if (ROOT / top).is_dir():
            files.extend((ROOT / top).rglob('*'))
    files.extend([ROOT / n for n in ('AGENTS.md', 'README.md', 'STATUS.md', 'THIRD_PARTY_NOTICES.md', 'Makefile', '.gitignore')])
    files = [p for p in files if p.is_file() and p.suffix in ('.md', '.json', '.py') or p.is_file() and p.name in ('Makefile', '.gitignore')]
    for path in files:
        scan_privacy(relative(path), path.read_text(encoding='utf-8'))
        if not args.privacy_only and path.suffix == '.json':
            load_json(path)
        if not args.privacy_only and path.suffix == '.md':
            check_links(path)
    if args.message_file:
        scan_privacy('提交信息', Path(args.message_file).read_text(encoding='utf-8'))
    check_identity()
    check_staged()
    count = {}
    if not args.privacy_only:
        for path, data in list(CACHE.items()):
            for node in walk_nodes(data):
                if '$ref' in node:
                    check_ref(path, node['$ref'])
        permissions = load_json(ROOT / 'contracts/permissions.json').get('permissions', [])
        codes = load_json(ROOT / 'contracts/errors.json').get('codes', [])
        count = check_modules(modules, permissions, codes, shared)
        check_events(modules)
        check_wiring(modules, shared)
        check_rendered()
        check_framework()
        check_go_modules()
        check_progress(list(modules) + list(shared))
        if len(modules) != 15:
            fail('模块清单', '服务数必须为已登记15域')
        for name in ('check_docs.py', 'render_docs.py'):
            ast.parse((ROOT / 'scripts' / name).read_text(encoding='utf-8'))
    if ISSUES:
        for issue in ISSUES:
            print(issue)
        print('检查失败：' + str(len(ISSUES)) + ' 项')
        return 1
    print('设计契约/引用/链接/权限/隐私检查通过；不代表业务验收')
    if count:
        print(json.dumps(count, ensure_ascii=False))
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
