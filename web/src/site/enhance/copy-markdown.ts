export function startCopyMarkdown() {
  const source = document.querySelector<HTMLTextAreaElement>('[data-markdown-source]')
  if (!source) return
  document.querySelectorAll<HTMLButtonElement>('[data-copy-markdown]').forEach(button => {
    if (!navigator.clipboard) { button.hidden = true; return }
    button.addEventListener('click', async () => {
      const original = button.textContent
      try { await navigator.clipboard.writeText(source.value); button.textContent = '已复制 Markdown' }
      catch { button.textContent = '复制失败，请打开 Markdown 版本' }
      setTimeout(() => { button.textContent = original }, 2200)
    })
  })
}
