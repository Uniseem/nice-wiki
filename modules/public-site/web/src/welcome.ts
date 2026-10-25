import { preloadQa } from '@modules/qa/web/src/island';

// 欢迎字按实际译文宽度缩放；两种入口使用相同的退场动画，普通链接行为保持可用。
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
  let leaving = false;
  welcome.querySelectorAll<HTMLAnchorElement>('[data-home-choice]').forEach((link) => {
    if (link.dataset.homeChoice === 'agent') {
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
      const animation = welcome.animate([
        { opacity: 1, transform: 'translateY(0) scale(1)', filter: 'blur(0)' },
        { opacity: 0, transform: 'translateY(-18px) scale(1.025)', filter: 'blur(6px)' },
      ], { duration: 320, easing: 'cubic-bezier(.4,0,.2,1)', fill: 'forwards' });
      void animation.finished.then(() => location.assign(link.href));
    });
  });
  // 浏览器返回命中页面缓存时，清掉已经完成的退场动画。
  window.addEventListener('pageshow', () => {
    leaving = false;
    delete welcome.dataset.leaving;
    welcome.getAnimations().forEach((animation) => animation.cancel());
  });
}
