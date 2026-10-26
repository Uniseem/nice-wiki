// 字母点阵背景：密排的等宽字母各自缓慢随机换字；一组窄而分明的波纹缓缓流过，经过处略加黑；
// 点击处荡开三圈更黑的涟漪，被扫到的字母换得更快。波纹本身缓而重，机械感来自字母的离散跳变。
// 首页默认不显示，点进 AI 问答时从点击处向外铺开；问答页铺满点阵并盖一层磨砂玻璃。
// 时间按墙上时钟算：从首页跳到问答页时，波纹位置、铺开的波前和涟漪都接着上一页继续。
// 减少动态效果时只画一帧静态点阵。

const P = {
  cell: 15, // 每格大小（px）
  base: .028, // 平时字母的浓度
  waveDark: .09, // 波峰处的额外加黑
  waveSpeed: 7, // 波纹推进速度（px/秒）
  waveLength: 900, // 波长（px）
  waveSharp: 7, // 波峰锐度：越大越窄越分明
  idleChange: 5.5, // 平时每个字母平均几秒换一次
  rippleSpeed: 190, // 涟漪扩散速度（px/秒）
  rippleDark: .42, // 涟漪加黑程度
  rippleLife: 3.6, // 涟漪持续秒数
  rippleChange: .12, // 涟漪里字母平均几秒换一次
  rippleWidth: 26, // 每圈涟漪的半宽（px）
  ringGap: .6, // 三圈涟漪前后相隔（秒）
};
const sweepSpeed = 820; // 铺开与扰动波前的速度（px/秒）
const glyphs = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
const levels = 64; // 透明度分档，同档的字母一次画完
const entryKey = 'nw-field-entry';

// 墙上时钟（秒）：两个页面各自的 performance.now 起点不同，换算成同一时钟才能接上。减去常数保住精度。
const clock = (ms = performance.now()) => (performance.timeOrigin + ms) / 1000 - 1.7e9;

type Point = { x: number; y: number; t0: number };
type Ripple = Point & { power: number };

export type LetterField = {
  // 从点击处铺开点阵（进入 AI 问答），并记下来让问答页接着铺。
  enter(x: number, y: number): void;
  // 接上首页留下的铺开；没有（直接打开问答页、刷新）就返回 false。
  resume(): boolean;
  fade(to: number, ms: number): Promise<void>;
};

export function startLetterField(initial: number): LetterField {
  const still = matchMedia('(prefers-reduced-motion: reduce)').matches;
  const canvas = document.createElement('canvas');
  canvas.className = 'nw-field';
  canvas.setAttribute('aria-hidden', 'true');
  document.body.prepend(canvas);
  const context = canvas.getContext('2d')!;
  let width = 0, height = 0, ratio = 1, columns = 0, rows = 0, painted = 0;
  let glyph = new Uint8Array(0), next = new Float64Array(0);
  let atlas = document.createElement('canvas');
  let ripples: Ripple[] = [];
  let surge: Point | null = null; // 波前一道深色、身后字母快速翻动
  let reveal: Point | null = null; // 波前还没到的地方先不显示
  let visible = initial;

  // 字形图集：每个字母先画一次，之后按格子贴图，比逐个 fillText 快得多。
  const buildAtlas = () => {
    const size = P.cell * ratio;
    atlas = document.createElement('canvas');
    atlas.width = size * glyphs.length;
    atlas.height = size;
    const draw = atlas.getContext('2d')!;
    draw.fillStyle = getComputedStyle(canvas).color;
    draw.font = `${Math.round(P.cell * .78 * ratio)}px ui-monospace, "SF Mono", SFMono-Regular, Menlo, Consolas, monospace`;
    draw.textAlign = 'center';
    draw.textBaseline = 'middle';
    for (let i = 0; i < glyphs.length; i++) draw.fillText(glyphs[i], i * size + size / 2, size / 2 + ratio * .5);
  };

  const resize = () => {
    ratio = Math.min(2, window.devicePixelRatio || 1);
    width = window.innerWidth;
    height = window.innerHeight;
    canvas.width = Math.round(width * ratio);
    canvas.height = Math.round(height * ratio);
    columns = Math.ceil(width / P.cell) + 1;
    rows = Math.ceil(height / P.cell) + 1;
    const now = clock();
    glyph = new Uint8Array(columns * rows);
    next = new Float64Array(columns * rows);
    for (let i = 0; i < glyph.length; i++) {
      glyph[i] = Math.random() * glyphs.length | 0;
      next[i] = now + Math.random() * P.idleChange * 2;
    }
    buildAtlas();
  };

  // 平时的流动：一组主波纹斜向缓缓推进，相位被低频起伏慢慢扭弯；波峰取高次幂，窄而分明。另有一组更淡的副波纹斜着穿过。
  const wave = (x: number, y: number, t: number) => {
    const k = 2 * Math.PI / P.waveLength, w = k * P.waveSpeed;
    const warp = 1.2 * Math.sin(k * .42 * y + w * .3 * t) + .9 * Math.sin(k * .35 * x - w * .22 * t + 2);
    const main = ((1 + Math.cos(k * (x * .86 + y * .5) - w * t + warp)) / 2) ** P.waveSharp;
    const side = ((1 + Math.cos(k * 1.55 * (x * -.32 + y * .95) - w * .8 * t + 1.3 + warp * .6)) / 2) ** (P.waveSharp * 1.4);
    return Math.min(1, main + side * .4);
  };

  // 涟漪：每次三圈，宽度固定、边缘利落（四次方的钟形），前后相隔 ringGap，随时间衰减。
  const ripple = (x: number, y: number, t: number) => {
    let sum = 0;
    for (const r of ripples) {
      const age = t - r.t0;
      const fade = Math.max(0, 1 - age / (P.rippleLife * r.power)) ** 1.5;
      if (age < 0 || fade <= 0) continue;
      const d = Math.hypot(x - r.x, y - r.y);
      for (let j = 0; j < 3; j++) {
        const front = P.rippleSpeed * r.power * (age - j * P.ringGap);
        if (front > 0) sum += Math.exp(-(((d - front) / P.rippleWidth) ** 4)) * [1, .72, .5][j] * fade * r.power / (1 + d / 1400);
      }
    }
    return Math.min(1.2, sum);
  };

  const draw = (t: number) => {
    ripples = ripples.filter((r) => t - r.t0 < P.rippleLife * r.power + 1.5);
    let surgeAmp = 0, surgeFront = 0;
    if (surge) {
      surgeAmp = Math.max(0, 1 - (t - surge.t0) / 1.6);
      surgeFront = (t - surge.t0) * sweepSpeed;
      if (surgeAmp <= 0) surge = null;
    }
    let revealFront = Infinity;
    if (reveal) {
      revealFront = (t - reveal.t0) * sweepSpeed;
      if (revealFront > Math.hypot(width, height) + 200) { reveal = null; revealFront = Infinity; }
    }
    const size = P.cell * ratio;
    const buckets: number[][] = Array.from({ length: levels }, () => []);
    for (let row = 0; row < rows; row++) {
      for (let column = 0; column < columns; column++) {
        const x = column * P.cell + P.cell / 2, y = row * P.cell + P.cell / 2, i = row * columns + column;
        const shown = revealFront === Infinity ? 1 : Math.min(1, (revealFront - Math.hypot(x - reveal!.x, y - reveal!.y)) / 140);
        if (shown <= 0) continue;
        const w = still ? .3 : wave(x, y, t);
        const rp = ripples.length ? ripple(x, y, t) : 0;
        let s = 0;
        if (surge) {
          const d = Math.hypot(x - surge.x, y - surge.y);
          s = surgeAmp * (Math.exp(-(((d - surgeFront) / 60) ** 4)) + (d < surgeFront ? .35 : 0));
        }
        // 换字：越黑换得越快；被涟漪或波前扫到的字母立刻进入快速翻动。
        const busy = rp + s * 1.5;
        if (t >= next[i]) {
          glyph[i] = Math.random() * glyphs.length | 0;
          next[i] = t + (busy > .05 ? P.rippleChange / Math.min(1, busy) : P.idleChange / (1 + w * 1.2)) * (.4 + Math.random() * 1.2);
        } else if (busy > .2 && next[i] - t > P.rippleChange * 2) {
          next[i] = t + P.rippleChange * Math.random();
        }
        const alpha = Math.min(.92, (P.base + P.waveDark * w + P.rippleDark * rp + .28 * s) * visible * shown);
        const level = Math.min(levels - 1, alpha * levels / .92 | 0);
        if (level > 0) buckets[level].push(i);
      }
    }
    context.clearRect(0, 0, canvas.width, canvas.height);
    for (let level = 1; level < levels; level++) {
      if (!buckets[level].length) continue;
      context.globalAlpha = (level + .5) / levels * .92;
      for (const i of buckets[level]) context.drawImage(atlas, glyph[i] * size, 0, size, size, (i % columns) * size, (i / columns | 0) * size, size, size);
    }
    context.globalAlpha = 1;
  };

  // 平时约 30 帧；铺开、扰动和涟漪期间满帧。看不见时不画。
  const loop = (ms: number) => {
    requestAnimationFrame(loop);
    const busy = ripples.length || surge || reveal;
    if (visible <= .001) {
      if (painted) context.clearRect(0, 0, canvas.width, canvas.height);
      painted = 0;
      return;
    }
    if (!busy && ms - painted < 33) return;
    painted = ms;
    draw(clock(ms));
  };

  resize();
  new ResizeObserver(() => { resize(); if (still) draw(clock()); }).observe(document.documentElement);
  // 切换明暗主题时字母换成新的前景色。
  new MutationObserver(() => { buildAtlas(); if (still) draw(clock()); }).observe(document.documentElement, { attributes: true, attributeFilter: ['class', 'data-theme'] });
  if (still) draw(clock());
  else requestAnimationFrame(loop);

  const start = (x: number, y: number, t0: number) => {
    visible = 1;
    if (still) { draw(clock()); return; }
    reveal = surge = { x, y, t0 };
    ripples.push({ x, y, t0, power: 1.6 });
  };

  return {
    enter(x, y) {
      const t0 = clock();
      start(x, y, t0);
      try { sessionStorage.setItem(entryKey, JSON.stringify({ x: x / width, y: y / height, t0 })); } catch { /* 隐私模式 */ }
    },
    resume() {
      let entry: { x: number; y: number; t0: number } | null = null;
      try {
        entry = JSON.parse(sessionStorage.getItem(entryKey) ?? 'null');
        sessionStorage.removeItem(entryKey);
      } catch { /* 隐私模式或记录损坏 */ }
      if (!entry || clock() - entry.t0 > 4) return false;
      start(entry.x * width, entry.y * height, entry.t0);
      return true;
    },
    fade(to, ms) {
      const from = visible, began = performance.now();
      return new Promise((done) => {
        const step = (now: number) => {
          const progress = still ? 1 : Math.min(1, (now - began) / ms);
          visible = from + (to - from) * (1 - (1 - progress) ** 3);
          if (still) draw(clock());
          if (progress < 1) requestAnimationFrame(step);
          else done();
        };
        requestAnimationFrame(step);
      });
    },
  };
}
