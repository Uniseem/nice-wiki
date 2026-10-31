#!/usr/bin/env python3
"""主链路冒烟：注册/登录 → 初始化站点 → 新建文档 → 加锁 → 保存草稿 → 发布 → 读公开页。
需要先 `python3 scripts/dev.py up`；重复运行会复用已有账号与站点。
`smoke.py multi`：多用户链路——站主发注册邀请码，两个新用户各建一个站、发布、上传，再检查站点之间互相读不到。"""
import json, os, sys, urllib.parse, urllib.request, urllib.error, uuid

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


def upload_bytes(png, name, kind, headers=None):
    import hashlib
    sha = hashlib.sha256(png).hexdigest()
    up = step("声明上传", call("POST", "/v1/uploads", {"sha256": sha, "byteSize": len(png), "originalName": name, "kind": kind}, headers=headers))["upload"]
    req = urllib.request.Request(PORTAL + f"/v1/uploads/{up['uploadId']}/chunks", data=png, method="PUT")
    for k, v in {"Origin": PORTAL, "Cookie": state["cookie"], "X-CSRF-Token": state["csrf"], "Idempotency-Key": uuid.uuid4().hex, "Content-Type": "application/octet-stream",
                 "X-Upload-Offset": "0", "X-Chunk-SHA256": sha, "If-Match": str(up["revision"]), **(headers or {})}.items():
        req.add_header(k, v)
    up = json.loads(urllib.request.urlopen(req).read())["upload"]
    return step("完成上传", call("POST", f"/v1/uploads/{up['uploadId']}:complete", {"expectedRevision": up["revision"]}, headers=headers))["asset"]["ref"]


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
        # 安装令牌：容器部署用 NW_SETUP_TOKEN 传入（docker compose exec account cat /data/identity/control/setup-token），本地实例直接读文件。
        token = os.environ.get("NW_SETUP_TOKEN") or open(os.path.join(os.environ.get("NW_DEV_DIR") or ".local/dev", "data", "identity", "control", "setup-token")).read().strip()
        body = step("注册首个超管", call("POST", "/v1/registration", {"accountName": ACCOUNT, "password": PASSWORD, "passwordConfirmation": PASSWORD, "setupToken": token}))
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



def login(account=ACCOUNT, password=PASSWORD):
    state["cookie"] = ""
    body = step("登录 " + account, call("POST", "/v1/sessions", {"accountName": account, "password": password}))
    state["csrf"] = body["session"]["csrfToken"] if "session" in body else body["tokens"]["csrfToken"]


def register_user(code, label):
    """用注册邀请码注册一个新用户并保持登录；返回 (邮箱, 口令)。"""
    email, pw = f"{label}-{uuid.uuid4().hex[:6]}@example.com", "User-Passw0rd-123456"
    state["cookie"] = ""
    body = step("注册 " + email, call("POST", "/v1/users:register", {"email": email, "password": pw, "passwordConfirmation": pw, "displayName": label, "inviteCode": code}))
    state["csrf"] = body["session"]["csrfToken"] if "session" in body else body["tokens"]["csrfToken"]
    return email, pw


def site_host(slug):
    return f"{slug}.localhost:{urllib.parse.urlsplit(PORTAL).port}"


def create_site(name):
    """建站并初始化、启用基准语言；返回站点（含 siteId、slug）。后台请求用 X-Nw-Site 选站点，和浏览器一样。"""
    slug = "s" + uuid.uuid4().hex[:8]
    site = step("建站 " + slug, call("POST", "/v1/sites", {"slug": slug, "name": name}))["site"]
    h = {"X-Nw-Site": site["siteId"]}
    step("初始化 " + slug, call("POST", "/v1/initialization", {"name": name, "baseLanguage": "zh-Hans"}, headers=h))
    key = uuid.uuid4().hex
    step("启用基准语言", call("PUT", "/v1/languages/zh-Hans", headers={**h, "Idempotency-Key": key}, body={"expectedVersion": 0, "idempotencyKey": key, "language": {
        "tag": "zh-Hans", "version": 1, "enabled": True, "isDefault": True, "automaticTranslation": False, "direction": "ltr",
        "reflectionEnabled": False, "batchOutputTokens": 1000, "translationMemoryMinimumCharacters": 20}}))
    return site


def publish_doc(h, title, text, slug):
    doc = step("新建文档", call("POST", "/v1/documents", {"title": title, "parentId": None, "beforeDocumentId": None}, headers=h))
    doc_id, draft = doc["draft"]["documentId"], doc["draft"]
    lease = step("获取编辑锁", call("POST", f"/v1/documents/{doc_id}/lock", {"tabId": uuid.uuid4().hex}, headers=h))["lease"]
    body = {"schemaVersion": 1, "type": "doc", "content": [{"type": "paragraph", "attrs": {"blockId": uuid.uuid4().hex}, "content": [{"type": "text", "text": text, "marks": []}]}]}
    saved = step("保存草稿", call("PUT", f"/v1/documents/{doc_id}/draft", {
        "body": body, "expectedDraftVersion": draft["draftVersion"], "expectedDraftRevisionId": draft["revisionId"],
        "leaseId": lease["leaseId"], "fencingToken": lease["fencingToken"], "serverBootId": lease["serverBootId"]}, headers=h))["draft"]
    node = next(n for n in call("GET", "/v1/tree", headers=h)[1]["documents"] if n["documentId"] == doc_id)
    step("发布", call("POST", f"/v1/documents/{doc_id}:publish", {"draftRevisionId": saved["revisionId"], "expectedDraftVersion": saved["draftVersion"],
        "expectedRevision": node["revision"], "manualPathSegment": slug, "manualSEO": title}, headers=h))
    step("释放编辑锁", call("DELETE", f"/v1/documents/{doc_id}/lock", {"leaseId": lease["leaseId"], "serverBootId": lease["serverBootId"]}, headers=h))
    return doc_id


def multi():
    import base64
    login()
    code = step("注册邀请码", call("POST", "/v1/registration-invites", {"maxUses": 2}))["code"]
    users = []
    for label in ("alice", "bob"):
        email, pw = register_user(code, label)
        site = create_site(label + " 的文档")
        h = {"X-Nw-Site": site["siteId"]}
        marker = f"{label} 的秘密 {uuid.uuid4().hex[:6]}"
        doc_id = publish_doc(h, label + " 首页", marker, "hello")
        upload_bytes(base64.b64decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg=="), "red.png", "image", headers=h)
        users.append({"email": email, "pw": pw, "site": site, "doc": doc_id, "marker": marker, "cookie": state["cookie"], "csrf": state["csrf"]})
    alice, bob = users
    # bob 带着 alice 的站点去读写：服务端按成员角色拒绝。
    for path in ["/v1/tree", f"/v1/documents/{alice['doc']}/draft", "/v1/assets?limit=10"]:
        status, _ = call("GET", path, headers={"X-Nw-Site": alice["site"]["siteId"]})
        print("bob 读 alice 的", path, status)
        if status < 400:
            sys.exit("失败于：跨站点读取没有被拒绝")
    status, _ = call("POST", "/v1/documents", {"title": "x", "parentId": None, "beforeDocumentId": None}, headers={"X-Nw-Site": alice["site"]["siteId"]})
    print("bob 往 alice 的站点写", status)
    if status < 400:
        sys.exit("失败于：跨站点写入没有被拒绝")
    # 各自的公开页只有自己的内容。
    for me, other in ((alice, bob), (bob, alice)):
        host = site_host(me["site"]["slug"])
        status, page = call("GET", "/zh-Hans/hello", headers={"Host": host, "Origin": "http://" + host})
        print("公开", me["site"]["slug"], status)
        if status != 200 or me["marker"] not in str(page) or other["marker"] in str(page):
            sys.exit("失败于：公开页内容不对")
    print("多用户链路通过")


if __name__ == "__main__":
    multi() if "multi" in sys.argv else main()
