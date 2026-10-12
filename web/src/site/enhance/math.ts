export function startMath() {
  const elements = [...document.querySelectorAll<HTMLElement>('.nw-math[data-latex]')]
  if (!elements.length) return
  void Promise.all([import('katex'), import('katex/dist/katex.min.css')]).then(([module]) => {
    for (const element of elements) {
      const latex = element.dataset.latex ?? element.textContent ?? ''
      try {
        module.default.render(latex, element, {
          displayMode: element.dataset.display === 'true', throwOnError: false,
          strict: 'warn', trust: false, output: 'htmlAndMathml',
        })
      } catch { element.textContent = latex }
    }
  }).catch(() => { /* 加载失败时保留服务端输出的 LaTeX 源码。 */ })
}
