#!/usr/bin/env python3
"""本机假供应方（只用于本地联调，不花钱、不连外网）：OpenAI 兼容 chat / embeddings / rerank，监听 127.0.0.1:9911。
用法：python3 scripts/dev_fake_provider.py；再用 scripts/dev_ai_setup.py 把它配进模型接入。"""
import hashlib, json, sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

DIM = 8

def vec(text):
    h = hashlib.sha256(text.encode()).digest()
    v = [(b - 128) / 128 for b in h[:DIM]]
    n = sum(x * x for x in v) ** 0.5 or 1
    return [x / n for x in v]

import re
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
    if "targetText" in system:
        try:
            payload = json.loads(last)
            return json.dumps({"units": [{"unitId": u["unitId"], "targetText": tr(u["text"])} for u in payload["units"]]}, ensure_ascii=False)
        except Exception:
            return "{}"
    return "（假回答）" + last[:200]

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
                    self.wfile.write(("data: " + json.dumps({"id": "c1", "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {"content": piece}}]}) + "\n\n").encode()); self.wfile.flush()
                self.wfile.write(("data: " + json.dumps({"id": "c1", "object": "chat.completion.chunk", "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}], "usage": {"prompt_tokens": 50, "completion_tokens": 20, "total_tokens": 70}}) + "\n\ndata: [DONE]\n\n").encode()); return
            return self.reply(200, {"id": "c1", "object": "chat.completion", "model": body.get("model", "fake"), "choices": [{"index": 0, "message": {"role": "assistant", "content": text}, "finish_reason": "stop"}], "usage": {"prompt_tokens": 50, "completion_tokens": 20, "total_tokens": 70}})
        if self.path.endswith("/rerank"):
            docs = body.get("documents", [])
            return self.reply(200, {"results": [{"index": i, "relevance_score": 1 - i / (len(docs) + 1)} for i in range(len(docs))]})
        self.reply(404, {"error": "not found"})

ThreadingHTTPServer(("127.0.0.1", 9911), H).serve_forever()
