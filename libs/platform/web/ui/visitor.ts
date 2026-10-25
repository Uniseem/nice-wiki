// 公开交互只发送活动类型与文档编号；不包含提问、搜索词或URL查询参数。
// 访客cookie、连接IP和设备头由可信入口建立。
export function recordVisitorActivity(eventType: 'qa-ask' | 'search', documentId = document.body.dataset.doc || 'home', locale = document.body.dataset.locale || 'zh-Hans'): void {
  const id = crypto.randomUUID().replaceAll('-', '');
  const payload = { beaconId: id, documentId, locale, eventType, status: 200, elapsedMs: 0, referrerDomain: '', userAgent: navigator.userAgent.slice(0, 1000) };
  void fetch('/v1/beacons', { method: 'POST', keepalive: true, headers: { 'Content-Type': 'application/json', 'Idempotency-Key': id }, body: JSON.stringify(payload) }).catch(() => {});
}
