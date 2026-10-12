export function startVideoFacades() {
  document.querySelectorAll<HTMLButtonElement>('[data-video-embed]').forEach(button => {
    button.addEventListener('click', () => {
      const source = button.dataset.videoEmbed
      if (!source) return
      let parsed: URL
      try { parsed = new URL(source, location.origin) } catch { return }
      const allowed = ['www.youtube-nocookie.com', 'player.bilibili.com', 'www.tiktok.com', 'www.douyin.com']
      if (parsed.protocol !== 'https:' || !allowed.includes(parsed.hostname)) return
      const frame = document.createElement('iframe')
      frame.src = parsed.href
      frame.title = button.dataset.videoTitle || '平台视频播放器'
      frame.loading = 'lazy'
      frame.allow = 'accelerometer; autoplay; encrypted-media; gyroscope; picture-in-picture'
      frame.allowFullscreen = true
      frame.referrerPolicy = 'strict-origin-when-cross-origin'
      frame.className = 'nw-video-frame'
      button.replaceWith(frame)
    }, { once: true })
  })
}
