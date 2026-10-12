export function startDrawers() {
  document.querySelectorAll<HTMLButtonElement>('[data-drawer-toggle]').forEach(button => {
    const target = button.dataset.drawerToggle
    if (!target) return
    const drawer = document.getElementById(target)
    if (!drawer) return
    const close = () => { drawer.dataset.open = 'false'; button.setAttribute('aria-expanded', 'false') }
    button.addEventListener('click', () => {
      const open = drawer.dataset.open !== 'true'
      drawer.dataset.open = String(open)
      button.setAttribute('aria-expanded', String(open))
      if (open) drawer.querySelector<HTMLElement>('a, button')?.focus()
    })
    drawer.addEventListener('click', event => { if ((event.target as HTMLElement).closest('a')) close() })
    document.addEventListener('keydown', event => { if (event.key === 'Escape' && drawer.dataset.open === 'true') { close(); button.focus() } })
  })
}
