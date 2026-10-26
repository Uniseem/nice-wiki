#!/usr/bin/env python3
"""把 modules/<域>/contracts/v2-ops.json 里的紧凑接口说明合并进该域的 openapi.json 与 types.json。

新架构不再维护函数级登记，所以 v2 接口只写这一份紧凑说明，由本脚本展开成标准的 OpenAPI 操作和 JSON Schema：
- 重复运行结果一致（按 x-v2 标记整体替换）；生成后再跑 make gen。
- 类型表达式：string、string:最大长度、int、int:最小:最大、bool、id、ts、hash、any、enum:a|b|c、[表达式]（数组）、
  类型名（引用同域类型）；末尾加 ? 表示可选。

用法：python3 scripts/v2_contract.py [域...]（缺省处理所有有 v2-ops.json 的域）
"""
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
COMMON = "../../../contracts/common.schema.json#/$defs/"

PROBLEM = {"description": "", "content": {"application/problem+json": {"schema": {"$ref": COMMON + "Problem"}}}}
ERRORS = {
    "400": "VALIDATION：字段、枚举、范围、格式或状态前置条件不合法；不记录敏感原值",
    "401": "UNAUTHENTICATED：会话、委托或服务证明缺失、失效或被撤销",
    "403": "FORBIDDEN：主体没有所需操作权限，或站点/资源作用域不匹配",
    "404": "NOT_FOUND：对象不存在或不可见",
    "409": "CONFLICT：状态冲突、唯一约束冲突或版本已改变",
    "429": "RATE_LIMITED：频率或容量限制",
    "503": "DEPENDENCY_UNAVAILABLE：权威依赖、存储或密钥不可用；不可把未知状态当允许",
}


def schema_of(expr):
    """类型表达式 -> (schema, required)。"""
    optional = expr.endswith("?")
    expr = expr.rstrip("?")
    if expr.startswith("["):
        inner, _ = schema_of(expr[1:-1])
        return {"type": "array", "items": inner, "maxItems": 1000}, not optional
    parts = expr.split(":")
    head = parts[0]
    if head == "string":
        s = {"type": "string"}
        s["maxLength"] = int(parts[1]) if len(parts) > 1 else 65536
        if len(parts) > 2:
            s["minLength"] = int(parts[2])
        return s, not optional
    if head == "int":
        s = {"type": "integer"}
        if len(parts) > 1 and parts[1] != "":
            s["minimum"] = int(parts[1])
        if len(parts) > 2:
            s["maximum"] = int(parts[2])
        return s, not optional
    if head == "bool":
        return {"type": "boolean"}, not optional
    if head == "id":
        return {"$ref": COMMON + "Identifier"}, not optional
    if head == "ts":
        return {"$ref": COMMON + "Timestamp"}, not optional
    if head == "hash":
        return {"$ref": COMMON + "Hash"}, not optional
    if head == "any":
        return {"type": "object", "additionalProperties": True}, not optional
    if head == "enum":
        return {"type": "string", "enum": parts[1].split("|")}, not optional
    return {"$ref": "#/$defs/" + head}, not optional


def build_type(name, spec):
    props, required = {}, []
    for field, expr in spec["props"].items():
        schema, req = schema_of(expr)
        props[field] = schema
        if req:
            required.append(field)
    out = {"type": "object", "description": spec.get("description", name), "additionalProperties": False, "properties": props}
    if required:
        out["required"] = required
    out["x-v2"] = True
    return out


def build_op(module, op):
    perm = op["perm"]
    security = [] if perm == "anonymous" else [{"serviceAssertion": []}] if perm == "service" else [{"serviceAssertion": [], "userDelegation": []}]
    parameters = []
    for name in op.get("path_params", []):
        parameters.append({"name": name, "in": "path", "required": True, "schema": {"$ref": COMMON + "Identifier"} if not op.get("path_free") else {"type": "string", "maxLength": 253}})
    for name, expr in op.get("query", {}).items():
        schema, req = schema_of(expr)
        parameters.append({"name": name, "in": "query", "required": req, "schema": schema})
    responses = {"200": {"description": op["summary"] + "的成功回执", "content": {"application/json": {"schema": {"$ref": "types.json#/$defs/" + op["out"]}}}}}
    for code, text in ERRORS.items():
        item = json.loads(json.dumps(PROBLEM))
        item["description"] = text
        responses[code] = item
    result = {
        "operationId": op["id"],
        "summary": op["summary"],
        "description": op.get("description", op["summary"]),
        "x-handler": f"{module}.{op['id'][0].upper()}{op['id'][1:]}",
        "x-permission": perm,
        "x-v2": True,
        "security": security,
        "parameters": parameters,
        "responses": responses,
    }
    if op.get("in"):
        result["requestBody"] = {"required": True, "content": {"application/json": {"schema": {"$ref": "types.json#/$defs/" + op["in"]}}}}
    return result


def apply(slug):
    folder = ROOT / "modules" / slug / "contracts"
    spec_path = folder / "v2-ops.json"
    if not spec_path.is_file():
        return False
    spec = json.loads(spec_path.read_text(encoding="utf-8"))
    types = json.loads((folder / "types.json").read_text(encoding="utf-8"))
    openapi = json.loads((folder / "openapi.json").read_text(encoding="utf-8"))
    defs = types["$defs"]
    for key in [k for k, v in defs.items() if isinstance(v, dict) and v.get("x-v2")]:
        del defs[key]
    for name, tspec in spec.get("types", {}).items():
        defs[name] = build_type(name, tspec)
    for path, item in list(openapi["paths"].items()):
        for method in list(item):
            if isinstance(item[method], dict) and item[method].get("x-v2"):
                del item[method]
        if not item:
            del openapi["paths"][path]
    for op in spec.get("ops", []):
        openapi["paths"].setdefault(op["path"], {})[op["method"]] = build_op(slug, op)
    (folder / "types.json").write_text(json.dumps(types, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    (folder / "openapi.json").write_text(json.dumps(openapi, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(f"{slug}: {len(spec.get('types', {}))} 个类型、{len(spec.get('ops', []))} 个接口")
    return True


if __name__ == "__main__":
    slugs = sys.argv[1:] or sorted(p.parent.parent.name for p in (ROOT / "modules").glob("*/contracts/v2-ops.json"))
    for s in slugs:
        apply(s)
