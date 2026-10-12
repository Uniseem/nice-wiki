export function startBanner() {
  document.querySelectorAll<HTMLElement>('[data-banner]').forEach(banner => {
    const key = `nw-banner:${banner.dataset.banner}`
    try { if (localStorage.getItem(key) === 'dismissed') banner.hidden = true } catch { /* 禁止存储时仍可关闭本页公告。 */ }
    banner.querySelector<HTMLButtonElement>('[data-banner-close]')?.addEventListener('click', () => {
      banner.hidden = true
      try { localStorage.setItem(key, 'dismissed') } catch { /* 不影响本页行为。 */ }
    })
  })
}
