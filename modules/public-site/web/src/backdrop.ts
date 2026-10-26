import { startLetterField } from './field';

// 问答页的背景：铺满字母点阵，盖一层磨砂玻璃（模糊加底色），不影响阅读。从首页进来时玻璃随后升起；
// 直接打开或刷新时点阵和玻璃都已就位。问答页和文档页站标右边的“首页”按钮：当前页内容下沉淡出、问答页的玻璃和点阵化开，再回首页。

const ease = 'cubic-bezier(.2,.8,.2,1)';
const still = () => matchMedia('(prefers-reduced-motion: reduce)').matches;
let glass: HTMLElement | null = null;

// 铺好问答页的点阵和玻璃；rising 时玻璃渐渐升起。玻璃只渐变透明度、模糊半径不变：逐帧改模糊半径要整屏重算，会卡。
export function showAskBackdrop(rising: boolean) {
  startLetterField(1);
  if (!glass) {
    glass = document.createElement('div');
    glass.className = 'nw-glass';
    glass.setAttribute('aria-hidden', 'true');
    document.querySelector('.nw-field')!.after(glass);
  }
  if (rising && !still()) glass.animate([{ opacity: 0 }, { opacity: 1 }], { duration: 900, easing: ease });
}

// “首页”按钮：页内切换换上新页头后要再绑一次，已经绑过的跳过。
export function bindBackHome() {
  document.querySelectorAll<HTMLAnchorElement>('[data-back-home]:not([data-bound])').forEach((link) => {
    link.dataset.bound = '';
    link.addEventListener('click', (event) => {
      if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || 'leaving' in document.body.dataset) return;
      event.preventDefault();
      document.body.dataset.leaving = '';
      if (still()) { location.assign(link.href); return; }
      const out = document.getElementById('nw-main')?.animate([{ opacity: 1, translate: '0 0' }, { opacity: 0, translate: '0 10px' }], { duration: 280, easing: 'cubic-bezier(.4,0,1,1)', fill: 'forwards' });
      const fading = glass ? [glass.animate([{ opacity: 1 }, { opacity: 0 }], { duration: 480, easing: ease, fill: 'forwards' }).finished, startLetterField(1).fade(0, 480)] : [];
      void Promise.all([out?.finished, ...fading]).then(() => location.assign(link.href));
    });
  });
}

export function enhanceBackdrop() {
  if (document.body.dataset.page === 'ask') showAskBackdrop(startLetterField(1).resume());
  bindBackHome();
  // 浏览器返回命中页面缓存时，清掉已经完成的退场动画。
  window.addEventListener('pageshow', (event) => {
    if (!event.persisted) return;
    delete document.body.dataset.leaving;
    document.getAnimations().forEach((animation) => animation.cancel());
    if (glass) void startLetterField(1).fade(1, 300);
  });
}
