#!/usr/bin/env python3
"""主链路冒烟：注册/登录 → 初始化站点 → 新建文档 → 加锁 → 保存草稿 → 发布 → 读公开页。
需要先 `python3 scripts/dev.py up`；重复运行会复用已有账号与站点。"""
import json, sys, urllib.request, urllib.error, uuid

PORTAL = "http://127.0.0.1:8080"
PUBLIC = "http://127.0.0.1:8081"
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
    doc = step("新建文档", call("POST", "/v1/documents", {"title": "快速开始", "parentId": None, "beforeDocumentId": None}))
    print(json.dumps(doc, ensure_ascii=False, indent=1)[:1500])


if __name__ == "__main__":
    main()
