"""把本机假供应方配成可用的模型、档位、任务分配、知识库索引（只用于本地联调；先运行 dev.py up 和 dev_fake_provider.py，站点需已初始化）。"""
import json as j, os, sys, uuid
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import smoke
s, b = smoke.call("POST", "/v1/sessions", {"accountName": smoke.ACCOUNT, "password": smoke.PASSWORD})
smoke.state["csrf"] = b["session"]["csrfToken"]
key = lambda: uuid.uuid4().hex


def call(method, path, body=None, show=True):
    h = {"Idempotency-Key": body["idempotencyKey"]} if isinstance(body, dict) and "idempotencyKey" in body else None
    r = smoke.call(method, path, body, headers=h)
    if show:
        print(method, path, r[0])
    return r


M = lambda n: {"currency": "USD", "micros": n}
for pid, adapter in (("fakechat", "openai_chat"), ("fakeemb", "openai_embeddings")):
    k = key()
    call("PUT", f"/v1/admin/providers/{pid}", {"expectedVersion": 0, "idempotencyKey": k, "provider": {"providerId": pid, "version": 1, "adapter": adapter, "baseUrl": "http://127.0.0.1:9911/v1", "enabled": True, "credentialVersion": 0, "connectTimeoutMs": 5000, "requestTimeoutMs": 60000, "displayName": pid, "extraHeaderNames": [], "extraHeaderSecretRefs": [], "concurrencyLimit": 4, "streamIdleTimeoutMs": 30000}}, show=False)
    call("POST", f"/v1/admin/providers/{pid}/credentials", {"providerId": pid, "secret": "test-secret", "expectedVersion": 0, "idempotencyKey": key()}, show=False)
price = {"inputPerMillionCost": M(100000), "outputPerMillionCost": M(200000), "fixedCost": M(0)}
for mid, pid, caps, dim, pre, version in (("chatm", "fakechat", ["chat", "structured", "vision"], 0, "none", "fake-chatm"), ("embm", "fakeemb", ["embedding"], 8, "none;tiktoken-go@v0.7.0;cl100k_base", "fake-embm"), ("decm", "fakechat", ["decision"], 0, "none", "typesafe/jev-1.14")):
    call("PUT", f"/v1/admin/models/{mid}", {"expectedVersion": 0, "idempotencyKey": key(), "manualPrice": price, "model": {"modelId": mid, "version": 1, "providerId": pid, "providerModelVersion": version, "capabilities": caps, "dimension": dim, "preprocessorVersion": pre, "maxInputTokens": 8000, "maxOutputTokens": 2000 if caps[0] == "chat" else 0, "catalogOrigin": "manual", "manualOverride": True, "enabled": True}}, show=False)
for cc in ("background", "qa"):
    for tier, m in (("fast", "chatm"), ("accurate", "chatm"), ("vision", "chatm"), ("embedding", "embm"), ("decision", "decm")):
        prof = f"{cc[:2]}-{tier}"
        call("PUT", f"/v1/admin/profiles/{prof}", {"expectedVersion": 0, "idempotencyKey": key(), "profile": {"profileId": prof, "version": 1, "costClass": cc, "tier": tier, "orderedModelIds": [m], "maximumCost": M(0), "concurrency": 4, "dailyLimitCost": M(0)}}, show=False)
for t in j.load(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'contracts', 'ai-tasks.json')))['tasks']:
    call("PUT", "/v1/task-assignments/" + t['id'], {"expectedVersion": 0, "idempotencyKey": key(), "assignment": {"taskId": t['id'], "tier": t['defaultTier'], "version": 1}}, show=False)
cat = call("GET", "/v1/admin/catalog", show=False)[1]
print({k: len(v) for k, v in cat.items()})
opt = call("GET", "/v1/index-options", show=False)[1]
prof = [p for p in opt["profiles"] if p["profileId"] == "ba-embedding"][0]
call("PUT", "/v1/index-config", {"idempotencyKey": key(), "expectedVersion": 0, "config": {"configId": key(), "version": 0, "enabled": True, "embeddingProfileId": "ba-embedding", "space": prof["space"], "chunkTokens": 512, "overlapTokens": 64, "language": "zh-Hans"}})
