export function startCitationHighlight() {
  const highlight = () => {
    if (!location.hash.startsWith('#b-')) return
    let id: string
    try { id = decodeURIComponent(location.hash.slice(1)) } catch { return }
    const target = document.getElementById(id)
    if (!target) return
    document.querySelectorAll('.nw-citation-target').forEach(element => element.classList.remove('nw-citation-target'))
    target.classList.add('nw-citation-target')
    setTimeout(() => { target.classList.remove('nw-citation-target') }, 3200)
  }
  window.addEventListener('hashchange', highlight)
  highlight()
}
