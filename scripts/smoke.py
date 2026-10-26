#!/usr/bin/env python3
"""主链路冒烟：注册/登录 → 初始化站点 → 新建文档 → 加锁 → 保存草稿 → 发布 → 读公开页。
需要先 `python3 scripts/dev.py up`；重复运行会复用已有账号与站点。"""
import json, os, sys, urllib.request, urllib.error, uuid

PORTAL = os.environ.get("NW_PORTAL", "http://127.0.0.1:8080")  # 对 docker 部署用 NW_PORTAL=https://localhost（自签证书，下面不校验）
import ssl
ssl._create_default_https_context = ssl._create_unverified_context
PUBLIC = os.environ.get("NW_PUBLIC", "http://127.0.0.1:8081")
ACCOUNT, PASSWORD = "admin", "Dev-Passw0rd-123456"
state = {"cookie": "", "csrf": ""}


def call(method, path, body=None, base=PORTAL, headers=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(base + path, data=data, method=method)
    req.add_header("Origin", PORTAL)
    if data is not None:
        req.add_header("Content-Type", "application/json")
    if state["cookie"]:
        req.add_header("Cookie", state["cookie"])
    if state["csrf"] and method != "GET":
        req.add_header("X-CSRF-Token", state["csrf"])
    if method != "GET":
        req.add_header("Idempotency-Key", uuid.uuid4().hex)
    for k, v in (headers or {}).items():
        req.add_header(k, v)
    try:
        resp = urllib.request.urlopen(req)
    except urllib.error.HTTPError as e:
        resp = e
    raw = resp.read().decode()
    cookies = [c.split(";")[0] for c in resp.headers.get_all("Set-Cookie") or []]
    if cookies:
        state["cookie"] = "; ".join(cookies)
    try:
        return resp.status, json.loads(raw)
    except ValueError:
        return resp.status, raw


def upload_png():
    import base64
    return upload_bytes(base64.b64decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg=="), "red.png", "image")


def upload_bytes(png, name, kind):
    import hashlib
    sha = hashlib.sha256(png).hexdigest()
    up = step("声明上传", call("POST", "/v1/uploads", {"sha256": sha, "byteSize": len(png), "originalName": name, "kind": kind}))["upload"]
    req = urllib.request.Request(PORTAL + f"/v1/uploads/{up['uploadId']}/chunks", data=png, method="PUT")
    for k, v in {"Origin": PORTAL, "Cookie": state["cookie"], "X-CSRF-Token": state["csrf"], "Idempotency-Key": uuid.uuid4().hex, "Content-Type": "application/octet-stream",
                 "X-Upload-Offset": "0", "X-Chunk-SHA256": sha, "If-Match": str(up["revision"])}.items():
        req.add_header(k, v)
    up = json.loads(urllib.request.urlopen(req).read())["upload"]
    return step("完成上传", call("POST", f"/v1/uploads/{up['uploadId']}:complete", {"expectedRevision": up["revision"]}))["asset"]["ref"]


def step(name, result):
    status, body = result
    print(f"[{status}] {name}: {json.dumps(body, ensure_ascii=False)[:300] if isinstance(body, (dict, list)) else str(body)[:300]}")
    if status >= 400:
        sys.exit(f"失败于：{name}")
    return body


def main():
    s, reg = call("GET", "/v1/registration")
    print("registration", reg)
    if reg.get("registrationOpen"):
        body = step("注册首个超管", call("POST", "/v1/registration", {"accountName": ACCOUNT, "password": PASSWORD, "passwordConfirmation": PASSWORD}))
    else:
        body = step("登录", call("POST", "/v1/sessions", {"accountName": ACCOUNT, "password": PASSWORD}))
    state["csrf"] = body["session"]["csrfToken"] if "session" in body else body["tokens"]["csrfToken"]
    status, init = call("GET", "/v1/initialization")
    print("initialization", init)
    if not (init.get("initialized") if isinstance(init, dict) else False):
        step("初始化站点", call("POST", "/v1/initialization", {"name": "示例文档站", "baseLanguage": "zh-Hans"}))
    status, langs = call("GET", "/v1/languages?includeDisabled=false")
    if not langs.get("languages"):
        key = uuid.uuid4().hex
        step("启用基准语言", call("PUT", "/v1/languages/zh-Hans", headers={"Idempotency-Key": key}, body={"expectedVersion": 0, "idempotencyKey": key, "language": {
            "tag": "zh-Hans", "version": 1, "enabled": True, "isDefault": True, "automaticTranslation": False, "direction": "ltr",
            "reflectionEnabled": False, "batchOutputTokens": 1000, "translationMemoryMinimumCharacters": 20}}))
    doc = step("新建文档", call("POST", "/v1/documents", {"title": "快速开始", "parentId": None, "beforeDocumentId": None}))
    doc_id = doc["draft"]["documentId"]
    slug = "quick-start" if len(sys.argv) < 2 else "page-" + uuid.uuid4().hex[:6]
    draft = doc["draft"]
    lock = step("获取编辑锁", call("POST", f"/v1/documents/{doc_id}/lock", {"tabId": uuid.uuid4().hex}))
    lease = lock["lease"]
    body = {"schemaVersion": 1, "type": "doc", "content": [
        {"type": "heading", "attrs": {"blockId": uuid.uuid4().hex, "level": 2}, "content": [{"type": "text", "text": "欢迎", "marks": []}]},
        {"type": "paragraph", "attrs": {"blockId": uuid.uuid4().hex}, "content": [{"type": "text", "text": "这是第一篇文档。", "marks": []}]}]}
    if "math" in sys.argv:
        body["content"].append({"type": "mathBlock", "attrs": {"blockId": uuid.uuid4().hex, "latex": "E = mc^2"}})
    for arg in sys.argv:
        if arg.startswith("video="):
            path = arg[6:]
            ref = upload_bytes(open(path, "rb").read(), os.path.basename(path), "video")
            body["content"].append({"type": "video", "attrs": {"blockId": uuid.uuid4().hex, "source": {"kind": "uploaded", "asset": ref}, "title": "演示视频", "description": "", "poster": None, "widthPercent": None}})
    if "img" in sys.argv:
        ref = upload_png()
        body["content"].append({"type": "image", "attrs": {"blockId": uuid.uuid4().hex, "asset": ref, "alt": "一个红点", "altSource": "manual", "caption": [], "widthPercent": None}})
    saved = step("保存草稿", call("PUT", f"/v1/documents/{doc_id}/draft", {
        "body": body, "expectedDraftVersion": draft["draftVersion"], "expectedDraftRevisionId": draft["revisionId"],
        "leaseId": lease["leaseId"], "fencingToken": lease["fencingToken"], "serverBootId": lease["serverBootId"]}))
    meta = call("GET", "/v1/tree")[1]
    print("tree", json.dumps(meta, ensure_ascii=False)[:400])
    node = next(n for n in meta["documents"] if n["documentId"] == doc_id)
    revision = node["revision"]
    new_draft = saved["draft"]
    pub = step("发布", call("POST", f"/v1/documents/{doc_id}:publish", {
        "draftRevisionId": new_draft["revisionId"], "expectedDraftVersion": new_draft["draftVersion"], "expectedRevision": revision,
        "manualPathSegment": slug, "manualSEO": "第一篇文档"}))
    step("释放编辑锁", call("DELETE", f"/v1/documents/{doc_id}/lock", {"leaseId": lease["leaseId"], "serverBootId": lease["serverBootId"]}))
    if "trash" in sys.argv:
        tree = call("GET", "/v1/tree")[1]
        node = next(n for n in tree["documents"] if n["documentId"] == doc_id)
        step("删除到回收站", call("POST", f"/v1/documents/{doc_id}:trash", {"expectedRevision": node["revision"], "expectedTreeRevision": tree["treeRevision"], "includeSubtree": False, "replacementDocumentId": None}))
        print("删除后公开页", call("GET", "/zh-Hans/" + slug)[0])
        trash = call("GET", "/v1/documents?state=trash&limit=100")[1]
        t = next(n for n in trash["documents"] if n["documentId"] == doc_id)
        step("恢复", call("POST", f"/v1/documents/{doc_id}:restore", {"expectedRevision": t["revision"], "parentId": None, "pathSegment": slug + "-r"}))
    for path in ["/", "/zh-Hans/", "/zh-Hans/" + slug, "/zh-Hans/navigation", "/robots.txt"]:
        status, page = call("GET", path, base=PORTAL)
        print("公开", path, status, str(page)[:160].replace("\n", " "))



if __name__ == "__main__":
    main()
