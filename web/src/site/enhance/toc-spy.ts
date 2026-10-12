export function startTocSpy() {
  const headings = [...document.querySelectorAll<HTMLElement>('.nw-content h2[id], .nw-content h3[id], .nw-content h4[id]')]
  const links = new Map<string, HTMLAnchorElement>()
  document.querySelectorAll<HTMLAnchorElement>('[data-toc] a[href^="#"]').forEach(link => links.set(link.hash.slice(1), link))
  if (!headings.length || !links.size || !('IntersectionObserver' in window)) return
  const visible = new Set<string>()
  const update = () => {
    const current = headings.find(heading => visible.has(heading.id))
    links.forEach((link, id) => { if (current?.id === id) link.setAttribute('aria-current', 'location'); else link.removeAttribute('aria-current') })
  }
  const observer = new IntersectionObserver(entries => {
    entries.forEach(entry => { if (entry.isIntersecting) visible.add(entry.target.id); else visible.delete(entry.target.id) })
    update()
  }, { rootMargin: '-5% 0px -65% 0px' })
  headings.forEach(heading => observer.observe(heading))
}
