export function startHeroVideo() {
  document.querySelectorAll<HTMLVideoElement>('[data-hero-video]').forEach(video => {
    const preference = window.matchMedia('(prefers-reduced-motion: reduce)')
    const update = () => {
      if (preference.matches || document.hidden) { video.pause(); return }
      video.muted = true
      void video.play().catch(() => { /* 自动播放受限时保留封面。 */ })
    }
    preference.addEventListener('change', update)
    document.addEventListener('visibilitychange', update)
    update()
  })
}
