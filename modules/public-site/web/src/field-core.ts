// 字母点阵的绘制核心，不碰 DOM：平时在 Web Worker 里对着 OffscreenCanvas 画（独立线程），浏览器不支持时退回主线程。
// 密排的等宽字母各自缓慢随机换字；一组窄而分明的波纹缓缓流过，经过处略加黑；点击处荡开三圈更黑的涟漪，被扫到的字母换得更快。
// 波纹本身缓而重，机械感来自字母的离散跳变。时间按墙上时钟算、字母由格子和时间槽算出，所以哪个页面、哪个线程画出的都一样。

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

// 墙上时钟（秒）：页面和 Worker 各自的 performance.now 起点不同，换算成同一时钟才能接上。减去常数保住精度。
export const clock = (ms = performance.now()) => (performance.timeOrigin + ms) / 1000 - 1.7e9;

// 整数哈希到 [0, 1)：同一格子、同一时间槽在任何页面都得到同一个值。
function hash(a: number, b: number): number {
  let h = Math.imul(a ^ 0x9e3779b9, 0x85ebca6b) ^ Math.imul((b | 0) + 0x7f4a7c15, 0xc2b2ae35) ^ Math.imul(Math.floor(b / 4294967296), 0x27d4eb2f);
  h = Math.imul(h ^ h >>> 15, 0x2c1b3c6d);
  h = Math.imul(h ^ h >>> 12, 0x297a2d39);
  return ((h ^ h >>> 15) >>> 0) / 4294967296;
}

type Point = { x: number; y: number; t0: number };
type Ripple = Point & { power: number };
type Surface = HTMLCanvasElement | OffscreenCanvas;

export type Renderer = {
  resize(width: number, height: number, ratio: number): void;
  color(value: string): void;
  // 从 (x, y) 铺开点阵并荡开强涟漪；t0 是墙上时钟，换页后可以接着上一页的时刻继续。
  enter(x: number, y: number, t0: number): void;
  fade(to: number, ms: number): void;
  // 每个显示帧调用一次（now 为本线程 performance.now），自己决定这一帧画不画。
  frame(now: number): void;
};

export function createRenderer(canvas: Surface, initial: { visible: number; still: boolean; color: string }): Renderer {
  const context = canvas.getContext('2d') as CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D;
  const still = initial.still;
  let color = initial.color;
  let width = 0, height = 0, ratio = 1, columns = 0, rows = 0, painted = 0, dirty = true;
  // 每格的固定参数（由格子位置算出，各页一致）：平时换字周期与相位、快速翻动周期、进入快速翻动的门槛；以及当前字母和时间槽。
  let slow = new Float32Array(0), phase = new Float32Array(0), quick = new Float32Array(0), threshold = new Float32Array(0);
  let glyph = new Uint8Array(0), slot = new Float64Array(0);
  let atlas: Surface = canvas;
  let ripples: Ripple[] = [];
  let surge: Point | null = null; // 波前一道深色、身后字母快速翻动
  let reveal: Point | null = null; // 波前还没到的地方先不显示
  // 可见度按时间淡入淡出：从 fadeFrom 到 fadeTo，用时 fadeMs。
  let fadeFrom = initial.visible, fadeTo = initial.visible, fadeStart = 0, fadeMs = 0;

  const visibleAt = (now: number) => {
    const progress = fadeMs <= 0 ? 1 : Math.min(1, (now - fadeStart) / fadeMs);
    return fadeFrom + (fadeTo - fadeFrom) * (1 - (1 - progress) ** 3);
  };

  // 字形图集：每个字母先画一次，之后按格子贴图，比逐个 fillText 快得多。
  const buildAtlas = () => {
    const size = P.cell * ratio;
    atlas = typeof OffscreenCanvas === 'function' ? new OffscreenCanvas(size * glyphs.length, size) : Object.assign(document.createElement('canvas'), { width: size * glyphs.length, height: size });
    const draw = atlas.getContext('2d') as CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D;
    draw.fillStyle = color;
    draw.font = `${Math.round(P.cell * .78 * ratio)}px ui-monospace, "SF Mono", SFMono-Regular, Menlo, Consolas, monospace`;
    draw.textAlign = 'center';
    draw.textBaseline = 'middle';
    for (let i = 0; i < glyphs.length; i++) draw.fillText(glyphs[i], i * size + size / 2, size / 2 + ratio * .5);
    dirty = true;
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

  const draw = (t: number, visible: number) => {
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
        // 换字：按时间槽取字母，槽一变就换；被涟漪或波前扫到的字母进入快速翻动。机械感来自这种离散跳变。
        const fast = rp + s * 1.5 > threshold[i];
        const current = fast ? Math.floor(t / quick[i]) * 2 + 1 : Math.floor(t / slow[i] + phase[i]) * 2;
        if (current !== slot[i]) {
          slot[i] = current;
          glyph[i] = hash(column * 7919 + row * 104729, current) * glyphs.length | 0;
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

  return {
    resize(nextWidth, nextHeight, nextRatio) {
      width = nextWidth; height = nextHeight; ratio = nextRatio;
      canvas.width = Math.round(width * ratio);
      canvas.height = Math.round(height * ratio);
      columns = Math.ceil(width / P.cell) + 1;
      rows = Math.ceil(height / P.cell) + 1;
      const cells = columns * rows;
      slow = new Float32Array(cells); phase = new Float32Array(cells); quick = new Float32Array(cells); threshold = new Float32Array(cells);
      glyph = new Uint8Array(cells); slot = new Float64Array(cells).fill(NaN);
      for (let i = 0; i < cells; i++) {
        const seed = (i % columns) * 7919 + (i / columns | 0) * 104729;
        slow[i] = P.idleChange * (.4 + 1.2 * hash(seed, 1));
        phase[i] = hash(seed, 2);
        quick[i] = P.rippleChange * (.6 + .8 * hash(seed, 3));
        // 门槛各格不同：涟漪和波前消退时，快速翻动一格一格散去，不会整片同时停住。
        threshold[i] = .04 + .32 * hash(seed, 4);
      }
      buildAtlas();
    },
    color(value) {
      color = value;
      buildAtlas();
    },
    enter(x, y, t0) {
      fadeFrom = fadeTo = 1;
      fadeMs = 0;
      dirty = true;
      if (still) return;
      reveal = surge = { x, y, t0 };
      ripples.push({ x, y, t0, power: 1.6 });
    },
    fade(to, ms) {
      const now = performance.now();
      fadeFrom = visibleAt(now);
      fadeTo = to;
      fadeStart = now;
      fadeMs = still ? 0 : ms;
      dirty = true;
    },
    // 平时约 30 帧；铺开、扰动、涟漪和淡入淡出期间满帧；看不见时清空一次后不画；减少动态效果时只在变化时画一帧。
    frame(now) {
      const visible = visibleAt(now);
      if (visible <= .001) {
        if (painted) context.clearRect(0, 0, canvas.width, canvas.height);
        painted = 0;
        return;
      }
      const busy = ripples.length || surge || reveal || now - fadeStart < fadeMs;
      if (still ? !dirty : !busy && !dirty && now - painted < 33) return;
      painted = now;
      dirty = false;
      draw(clock(now), visible);
    },
  };
}
