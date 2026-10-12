export function startCodeCopy() {
  document.querySelectorAll<HTMLPreElement>('.nw-code').forEach(pre => {
    const code = pre.querySelector('code')
    if (!code || !navigator.clipboard) return
    const button = document.createElement('button')
    button.type = 'button'; button.textContent = '复制'; button.className = 'nw-code-copy'
    button.setAttribute('aria-label', '复制代码')
    button.addEventListener('click', async () => {
      try { await navigator.clipboard.writeText(code.textContent ?? ''); button.textContent = '已复制' }
      catch { button.textContent = '复制失败' }
      setTimeout(() => { button.textContent = '复制' }, 1800)
    })
    pre.prepend(button)
  })
}
