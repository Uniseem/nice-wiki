export function startTree() {
  document.querySelectorAll<HTMLButtonElement>('[data-tree-toggle]').forEach(button => {
    const id = button.dataset.treeToggle
    if (!id) return
    const subtree = document.getElementById(id)
    if (!subtree) return
    button.addEventListener('click', () => {
      const open = button.getAttribute('aria-expanded') !== 'true'
      button.setAttribute('aria-expanded', String(open))
      subtree.hidden = !open
    })
  })
}
