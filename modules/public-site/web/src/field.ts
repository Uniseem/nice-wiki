// 首页背景：一层细点阵。平时点的明暗随几组低频波缓慢流动，位置也轻轻浮动；指针附近略亮。
// 按下时从按下处向外逐格扩散：波前每一拍推进一格、距离也按格取整，点被顶大、点亮再逐拍熄灭，呈现机械式的推进。
// 减少动态效果时只画静态点阵，不响应点击。

type Ripple = { x: number; y: number; at: number };

const beat = 34; // 波前每拍推进一格（毫秒）
const afterglow = [1, .66, .4, .22, .1]; // 波前经过后各拍的余辉
const reach = 22; // 最远扩散的格数
const levels = 12; // 透明度分档，同档的点一次画完

export function startField(host: HTMLElement) {
  const canvas = document.createElement('canvas');
  canvas.className = 'nw-field';
  canvas.setAttribute('aria-hidden', 'true');
  host.prepend(canvas);
  const context = canvas.getContext('2d');
  if (!context) return;
  const still = matchMedia('(prefers-reduced-motion: reduce)');
  const ripples: Ripple[] = [];
  let width = 0, height = 0, gap = 28, frame = 0, painted = 0;
  let pointer: { x: number; y: number } | null = null;

  const measure = () => {
    const ratio = Math.min(2, window.devicePixelRatio || 1);
    width = host.clientWidth;
    height = host.clientHeight;
    gap = width < 640 ? 22 : 28;
    canvas.width = Math.round(width * ratio);
    canvas.height = Math.round(height * ratio);
    context.setTransform(ratio, 0, 0, ratio, 0, 0);
  };

  const draw = (now: number) => {
    const time = now / 1000;
    const columns = Math.ceil(width / gap) + 1, rows = Math.ceil(height / gap) + 1;
    const left = (width - (columns - 1) * gap) / 2, top = (height - (rows - 1) * gap) / 2;
    const buckets: number[][] = Array.from({ length: levels }, () => []);
    while (ripples.length && now - ripples[0].at > (reach + afterglow.length) * beat) ripples.shift();
    for (let row = 0; row < rows; row++) {
      for (let column = 0; column < columns; column++) {
        const gx = left + column * gap, gy = top + row * gap;
        const cloud = (Math.sin(gx * .009 + time * .32) + Math.sin(gy * .011 - time * .26) + Math.sin((gx - gy) * .006 + time * .18)) / 3;
        let alpha = .07 + (cloud + 1) * .075;
        let x = gx + Math.sin(time * .7 + row * .9 + column * .4) * .8;
        let y = gy + Math.cos(time * .6 + column * .8 + row * .3) * .8;
        let size = 1.4;
        if (pointer) {
          const near = 1 - Math.hypot(gx - pointer.x, gy - pointer.y) / 150;
          if (near > 0) { alpha += near * .1; size += near * .4; }
        }
        for (const ripple of ripples) {
          const dx = gx - ripple.x, dy = gy - ripple.y;
          const distance = Math.round(Math.hypot(dx, dy) / gap);
          const lag = Math.floor((now - ripple.at) / beat) - distance;
          if (lag < 0 || lag >= afterglow.length || distance > reach) continue;
          const pulse = afterglow[lag] * (1 - distance / (reach + 1));
          const length = Math.hypot(dx, dy) || 1;
          alpha += pulse * .7;
          size += pulse * 2.2;
          x += dx / length * pulse * 3;
          y += dy / length * pulse * 3;
        }
        buckets[Math.min(levels - 1, Math.floor(alpha * levels))].push(x - size / 2, y - size / 2, size);
      }
    }
    context.clearRect(0, 0, width, height);
    context.fillStyle = getComputedStyle(canvas).color;
    buckets.forEach((dots, level) => {
      if (!dots.length) return;
      context.globalAlpha = (level + .5) / levels;
      for (let index = 0; index < dots.length; index += 3) context.fillRect(dots[index], dots[index + 1], dots[index + 2], dots[index + 2]);
    });
  };

  // 平时约 30 帧，扩散时满帧。
  const loop = (now: number) => {
    if (ripples.length || now - painted > 32) {
      painted = now;
      draw(now);
    }
    frame = requestAnimationFrame(loop);
  };
  const start = () => {
    cancelAnimationFrame(frame);
    ripples.length = 0;
    if (still.matches) draw(0);
    else frame = requestAnimationFrame(loop);
  };

  new ResizeObserver(() => { measure(); if (still.matches) draw(0); }).observe(host);
  // 切换明暗主题时静态点阵也要重画成新颜色。
  new MutationObserver(() => { if (still.matches) draw(0); }).observe(document.documentElement, { attributes: true, attributeFilter: ['class', 'data-theme'] });
  still.addEventListener('change', start);
  host.addEventListener('pointerdown', (event) => {
    if (still.matches) return;
    const box = host.getBoundingClientRect();
    ripples.push({ x: event.clientX - box.left, y: event.clientY - box.top, at: performance.now() });
    if (ripples.length > 4) ripples.shift();
  });
  host.addEventListener('pointermove', (event) => {
    if (event.pointerType !== 'mouse') return;
    const box = host.getBoundingClientRect();
    pointer = { x: event.clientX - box.left, y: event.clientY - box.top };
  });
  host.addEventListener('pointerleave', () => { pointer = null; });
  measure();
  start();
}
