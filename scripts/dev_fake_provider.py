#!/usr/bin/env python3
"""本机假供应方（只用于本地联调，不花钱、不连外网）：OpenAI 兼容 chat / embeddings / rerank，以及 OpenRouter 的 Decisions 接口（/alpha/decisions），
默认监听 127.0.0.1:9911（NW_FAKE_PORT 可改）。用法：python3 scripts/dev_fake_provider.py；再用 scripts/dev_ai_setup.py 把它配进模型接入。"""
import hashlib, json, sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

DIM = 8

def vec(text):
    h = hashlib.sha256(text.encode()).digest()
    v = [(b - 128) / 128 for b in h[:DIM]]
    n = sum(x * x for x in v) ** 0.5 or 1
    return [x / n for x in v]

import re, uuid
def tr(text):
    if re.search(r"<i\d+>", text):
        return re.sub(r"(<i\d+>)(.*?)(</i\d+>)", lambda m: m.group(1) + "(EN) " + m.group(2) + m.group(3), text)
    return "(EN) " + text

def reply_text(messages, last):
    system = " ".join(m["content"] for m in messages if m["role"] == "system" and isinstance(m["content"], str))
    if '"items"' in system and "chunkId" in system:
        try:
            batch = json.loads(last)
            ids = [c.get("chunkId") or c.get("id") for c in (batch if isinstance(batch, list) else batch.get("chunks", batch.get("items", [])))]
            return json.dumps({"items": [{"chunkId": i, "prefix": "（假背景）"} for i in ids]}, ensure_ascii=False)
        except Exception as e:
            return "{}"
    if "URL 路径名" in system:
        return "fake-" + hashlib.sha256(last.encode()).hexdigest()[:6]
    if "搜索摘要" in system:
        try:
            return "（假摘要）" + json.loads(last)["title"]
        except Exception:
            return ""
    if "summary card" in system:
        try:
            title = json.loads(last)["title"] or "Document"
            return json.dumps({"title": "(EN) " + title, "summary": "(fake summary) Covers " + title + ".", "topics": [title], "questions": ["How do I use " + title + "?"]}, ensure_ascii=False)
        except Exception:
            return "{}"
    if "into English for an internal classifier" in system:
        return "(EN) " + last
    if "分析资料边界内的问题" in system:
        try:
            q = json.loads(last)["question"]
            return json.dumps({"canonicalQuestion": q, "complete": True, "cacheEligible": True, "resolvedReferences": True, "subqueries": [q], "classification": "simple", "risk": "normal", "currentPageDependent": False, "privateContextDependent": False}, ensure_ascii=False)
        except Exception:
            return "{}"
    if "界面用语" in system:
        try:
            payload = json.loads(last)
            return json.dumps({"items": [{"key": i["key"], "value": "[UI] " + i["value"]} for i in payload["items"]]}, ensure_ascii=False)
        except Exception:
            return "{}"
    if "targetText" in system:
        try:
            payload = json.loads(last)
            return json.dumps({"units": [{"unitId": u["unitId"], "targetText": tr(u["text"])} for u in payload["units"]]}, ensure_ascii=False)
        except Exception:
            return "{}"
    return "（假回答）" + last[:200]

def decide(body):
    """假决策：单选题选在状态里出现次数最多的选项（都没出现就选保守选项或第一个）；是非题默认 0.9，状态里带 fake-no 时 0.1；打分题取第一级。"""
    state = json.dumps(body.get("state"), ensure_ascii=False).lower()
    answers = {}
    for qid, q in body.get("questions", {}).items():
        if q["type"] == "noul":
            answers[qid] = {"type": "noul", "noul": 0.1 if "fake-no" in state else 0.9}
        elif q["type"] == "choice":
            options = list(q["criteria"].keys())
            # 选项都没在状态里出现时，优先选“正常 / 文档问题 / 复杂”这类保守选项，测试时在问题里写选项名就能选中它。
            safe = [o for o in ("normal", "docs", "complex") if o in options]
            pick = max(options, key=lambda o: (state.count(o.lower()), o in safe, -options.index(o)))
            answers[qid] = {"type": "choice", "choice": pick, "confidence": 0.8, "probabilities": {o: (0.9 if o == pick else round(0.1 / max(1, len(options) - 1), 4)) for o in options}}
        else:
            levels = q["criteria"]
            answers[qid] = {"type": "score", "score": 0.0, "confidence": 0.8, "legend": {str(i): l for i, l in enumerate(levels)}, "probabilities": {str(i): (1.0 if i == 0 else 0.0) for i in range(len(levels))}}
    return {"id": "gen-dec-fake-" + uuid.uuid4().hex, "model": body.get("model", "fake") + "-fake", "provider": "Fake", "answers": answers, "usage": {"input_tokens": len(state) // 4 + 1, "output_tokens": len(answers)}}

class H(BaseHTTPRequestHandler):
    def log_message(self, fmt, *a):
        sys.stderr.write("%s %s\n" % (self.command, self.path))

    def reply(self, code, obj):
        raw = json.dumps(obj).encode()
        self.send_response(code); self.send_header("Content-Type", "application/json"); self.send_header("Content-Length", str(len(raw))); self.end_headers(); self.wfile.write(raw)

    def do_POST(self):
        try:
            self._post()
        except Exception:
            import traceback; traceback.print_exc(); sys.stderr.flush()
            raise

    def _post(self):
        body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", "0"))) or b"{}")
        if self.path.endswith("/alpha/decisions"):
            return self.reply(200, decide(body))
        if self.path.endswith("/embeddings"):
            inputs = body["input"] if isinstance(body["input"], list) else [body["input"]]
            data = [{"object": "embedding", "index": i, "embedding": vec(t)} for i, t in enumerate(inputs)]
            return self.reply(200, {"object": "list", "data": data, "model": body.get("model", "fake"), "usage": {"prompt_tokens": sum(len(t) for t in inputs) // 4 + 1, "total_tokens": sum(len(t) for t in inputs) // 4 + 1}})
        if self.path.endswith("/chat/completions"):
            messages = body.get("messages", [])
            last = messages[-1]["content"] if messages else ""
            if isinstance(last, list):
                last = " ".join(p.get("text", "") for p in last)
            text = reply_text(messages, last)
            if body.get("stream"):
                self.send_response(200); self.send_header("Content-Type", "text/event-stream"); self.end_headers()
                for piece in [text[i:i + 8] for i in range(0, len(text), 8)]:
                    self.wfile.write(("data: " + json.dumps({"id": "fake-" + uuid.uuid4().hex, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {"content": piece}}]}) + "\n\n").encode()); self.wfile.flush()
                self.wfile.write(("data: " + json.dumps({"id": "fake-" + uuid.uuid4().hex, "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}], "usage": {"prompt_tokens": 50, "completion_tokens": 20, "total_tokens": 70}}) + "\n\ndata: [DONE]\n\n").encode()); return
            return self.reply(200, {"id": "fake-" + uuid.uuid4().hex, "object": "chat.completion", "model": body.get("model", "fake"), "choices": [{"index": 0, "message": {"role": "assistant", "content": text}, "finish_reason": "stop"}], "usage": {"prompt_tokens": 50, "completion_tokens": 20, "total_tokens": 70}})
        if self.path.endswith("/rerank"):
            docs = body.get("documents", [])
            return self.reply(200, {"results": [{"index": i, "relevance_score": 1 - i / (len(docs) + 1)} for i in range(len(docs))]})
        self.reply(404, {"error": "not found"})

import os
ThreadingHTTPServer(("127.0.0.1", int(os.environ.get("NW_FAKE_PORT", "9911"))), H).serve_forever()
