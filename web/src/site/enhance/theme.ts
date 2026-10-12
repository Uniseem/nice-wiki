const key = 'nw-theme'
type Theme = 'light' | 'dark'
function storedTheme(): Theme | undefined {
  try { const value = localStorage.getItem(key); return value === 'light' || value === 'dark' ? value : undefined }
  catch { return undefined }
}
function apply(theme: Theme) {
  document.documentElement.dataset.theme = theme
  document.querySelectorAll<HTMLButtonElement>('[data-theme-toggle]').forEach(button => {
    button.setAttribute('aria-label', theme === 'dark' ? '切换为亮色' : '切换为暗色')
    button.setAttribute('aria-pressed', String(theme === 'dark'))
  })
}
export function startTheme() {
  const preference = window.matchMedia('(prefers-color-scheme: dark)')
  apply(storedTheme() ?? (preference.matches ? 'dark' : 'light'))
  preference.addEventListener('change', event => { if (!storedTheme()) apply(event.matches ? 'dark' : 'light') })
  document.querySelectorAll<HTMLButtonElement>('[data-theme-toggle]').forEach(button => {
    button.addEventListener('click', () => {
      const theme = document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark'
      apply(theme)
      try { localStorage.setItem(key, theme) } catch { /* 禁止存储时仍可切换本页。 */ }
    })
  })
}
