export function startBeacon() {
  if (!navigator.sendBeacon) return
  const payload = JSON.stringify({ path: location.pathname, language: document.documentElement.lang, referrer: document.referrer })
  addEventListener('load', () => navigator.sendBeacon('/api/public/beacon', new Blob([payload], { type: 'application/json' })), { once: true })
}
