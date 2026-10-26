import { startLetterField, type LetterField } from './field';

// 问答页的背景：铺满字母点阵，盖一层磨砂玻璃（模糊加底色），不影响阅读。从首页点进来时接着首页的铺开，玻璃随后升起；
// 直接打开或刷新时点阵和玻璃都已就位。问答页和文档页左上角的“首页”按钮：当前页内容下沉淡出、问答页的玻璃和点阵化开，再回首页。

const ease = 'cubic-bezier(.2,.8,.2,1)';
const still = () => matchMedia('(prefers-reduced-motion: reduce)').matches;

export function enhanceBackdrop() {
  const page = document.body.dataset.page;
  let field: LetterField | null = null;
  let glass: HTMLElement | null = null;
  if (page === 'ask') {
    field = startLetterField(1);
    glass = document.createElement('div');
    glass.className = 'nw-glass';
    glass.setAttribute('aria-hidden', 'true');
    document.querySelector('.nw-field')!.after(glass);
    if (field.resume() && !still()) glass.animate([{ opacity: 0, '--nw-glass-blur': '0px' }, { opacity: 1 }], { duration: 700, delay: 150, easing: ease, fill: 'backwards' });
  }

  let leaving = false;
  document.querySelectorAll<HTMLAnchorElement>('[data-back-home]').forEach((link) => link.addEventListener('click', (event) => {
    if (event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    if (leaving) return;
    leaving = true;
    if (still()) { location.assign(link.href); return; }
    const main = document.getElementById('nw-main');
    const out = main?.animate([{ opacity: 1, translate: '0 0' }, { opacity: 0, translate: '0 10px' }], { duration: 280, easing: 'cubic-bezier(.4,0,1,1)', fill: 'forwards' });
    glass?.animate([{ opacity: 1 }, { opacity: 0, '--nw-glass-blur': '0px' }], { duration: 480, easing: ease, fill: 'forwards' });
    const fading = field?.fade(0, 480);
    void Promise.all([out?.finished, fading]).then(() => location.assign(link.href));
  }));
  // 浏览器返回命中页面缓存时，清掉已经完成的退场动画。
  window.addEventListener('pageshow', (event) => {
    if (!event.persisted) return;
    leaving = false;
    document.getAnimations().forEach((animation) => animation.cancel());
    void field?.fade(1, 300);
  });
}
