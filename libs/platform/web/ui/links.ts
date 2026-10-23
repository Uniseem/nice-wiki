// 全站共用的外链行为：初始页面和后续渲染的链接都打开新标签页，站内导航保持原有行为。
export function installExternalLinks() {
  const prepare = (link: HTMLAnchorElement) => {
    // 编辑器管理自己的 DOM，编辑中的链接不触发页面导航。
    if (link.closest('[contenteditable="true"]')) return;
    const url = new URL(link.href, location.href);
    if ((url.protocol === 'https:' || url.protocol === 'http:') && url.origin !== location.origin) {
      link.target = '_blank';
      link.relList.add('noopener', 'noreferrer');
    }
  };
  const scan = (root: ParentNode) => {
    if (root instanceof HTMLAnchorElement) prepare(root);
    root.querySelectorAll<HTMLAnchorElement>('a[href]').forEach(prepare);
  };
  scan(document);
  new MutationObserver((records) => {
    for (const record of records) {
      if (record.type === 'attributes' && record.target instanceof HTMLAnchorElement) prepare(record.target);
      else record.addedNodes.forEach((node) => { if (node instanceof Element) scan(node); });
    }
  }).observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ['href'] });
}
