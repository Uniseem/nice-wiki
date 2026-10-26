import { preloadQa } from '@modules/qa/web/src/island';
import { startLetterField, type LetterField } from './field';

// 欢迎字按实际译文宽度缩放。首页本身不铺背景；两种入口各有退场：
// 进 AI 问答时字母点阵从点击处向外铺开、波前扫过处字母快速翻动，首页内容放大虚化散去（问答页接着铺开并升起磨砂玻璃）；
// 进文档时内容轻轻上移淡出，稳重利落。普通链接行为（新标签打开等）保持可用。
export function enhanceWelcome() {
  const welcome = document.querySelector<HTMLElement>('.nw-welcome');
  const outline = welcome?.querySelector<HTMLElement>('.nw-welcome-outline');
  const text = outline?.querySelector('span');
  if (!welcome || !outline || !text) return;
  const fit = () => {
    text.style.fontSize = '100px';
    text.style.fontSize = `${Math.min(260, welcome.clientHeight * .52, Math.max(32, outline.clientWidth / Math.max(1, text.scrollWidth) * 100))}px`;
  };
  const observer = new ResizeObserver(fit);
  observer.observe(outline);
  void document.fonts.ready.then(fit);
  let field: LetterField | null = null;
  let leaving = false;
  welcome.querySelectorAll<HTMLAnchorElement>('[data-home-choice]').forEach((link) => {
    const agent = link.dataset.homeChoice === 'agent';
    if (agent) {
      link.addEventListener('pointerenter', preloadQa, { once: true });
      link.addEventListener('focus', preloadQa, { once: true });
    }
    link.addEventListener('click', (event) => {
      if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.defaultPrevented) return;
      event.preventDefault();
      if (leaving) return;
      leaving = true;
      if (matchMedia('(prefers-reduced-motion: reduce)').matches) { location.assign(link.href); return; }
      welcome.dataset.leaving = link.dataset.homeChoice;
      // 键盘回车触发时没有指针位置，从入口中心铺开。
      const box = link.getBoundingClientRect();
      const x = event.detail ? event.clientX : box.left + box.width / 2, y = event.detail ? event.clientY : box.top + box.height / 2;
      if (agent) {
        field ??= startLetterField(0);
        field.enter(x, y);
      }
      const animations = [...welcome.children].map((node, index) => node.animate(agent
        ? [{ opacity: 1, filter: 'blur(0)', scale: '1' }, { opacity: 0, filter: 'blur(10px)', scale: '1.06' }]
        : [{ opacity: 1, translate: '0 0' }, { opacity: 0, translate: '0 -10px' }],
      { duration: agent ? 520 : 300, delay: index * (agent ? 50 : 25), easing: 'cubic-bezier(.4,0,1,1)', fill: 'forwards' }));
      void Promise.all(animations.map((animation) => animation.finished)).then(() => location.assign(link.href));
    });
  });
  // 浏览器返回命中页面缓存时，清掉已经完成的退场动画和点阵。
  window.addEventListener('pageshow', (event) => {
    if (!event.persisted) return;
    leaving = false;
    delete welcome.dataset.leaving;
    welcome.getAnimations({ subtree: true }).forEach((animation) => animation.cancel());
    void field?.fade(0, 300);
  });
}
