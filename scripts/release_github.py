#!/usr/bin/env python3
"""把发布包发布到 GitHub Releases（由 scripts/release.sh publish 调用）：标签须已推到 GitHub，
先建草稿、逐个上传附件，全部传完才转为正式版并标为最新，不会出现附件不全的公开版本。
用法：release_github.py <版本> <说明文件> <附件...>；需要 GITHUB_TOKEN（仓库内容可写），NW_GITHUB_REPO 默认 Uniseem/nice-wiki。"""
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request

API = "https://api.github.com"
repo = os.environ.get("NW_GITHUB_REPO", "Uniseem/nice-wiki")
token = os.environ["GITHUB_TOKEN"]
tag, notes_file, assets = sys.argv[1], sys.argv[2], sys.argv[3:]


def call(method, url, body=None, content_type="application/json"):
    request = urllib.request.Request(url, data=body, method=method, headers={
        "Authorization": f"Bearer {token}", "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28", "Content-Type": content_type})
    try:
        with urllib.request.urlopen(request) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        sys.exit(f"GitHub {method} {url} 失败：{error.code} {error.read().decode(errors='replace')[:300]}")


call("GET", f"{API}/repos/{repo}/git/ref/tags/{urllib.parse.quote(tag)}")  # 标签不在 GitHub 上时 GitHub 会在默认分支上另建，先挡住
release = call("POST", f"{API}/repos/{repo}/releases", json.dumps({
    "tag_name": tag, "name": f"nice-wiki {tag}", "body": open(notes_file, encoding="utf-8").read().strip(), "draft": True}).encode())
upload = release["upload_url"].split("{")[0]
for path in assets:
    name = os.path.basename(path)
    with open(path, "rb") as file:
        call("POST", f"{upload}?name={urllib.parse.quote(name)}", file.read(), "application/octet-stream")
    print("已上传", name)
done = call("PATCH", f"{API}/repos/{repo}/releases/{release['id']}", json.dumps({"draft": False, "make_latest": "true"}).encode())
print("已发布", done["html_url"])
