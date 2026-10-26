import { createRenderer, type Renderer } from './field-core';

// 字母点阵的绘制线程：主线程把画布控制权交过来（OffscreenCanvas），这里自己按显示帧画。
// 页面换内容、渲染 React、加载脚本时主线程再忙，点阵也照常流动。

export type FieldMessage =
  | { type: 'init'; canvas: OffscreenCanvas; width: number; height: number; ratio: number; color: string; visible: number; still: boolean }
  | { type: 'resize'; width: number; height: number; ratio: number }
  | { type: 'color'; value: string }
  | { type: 'enter'; x: number; y: number; t0: number }
  | { type: 'fade'; to: number; ms: number };

let renderer: Renderer | null = null;
const frame = (now: number) => {
  renderer?.frame(now);
  schedule();
};
const schedule = () => {
  if (typeof requestAnimationFrame === 'function') requestAnimationFrame(frame);
  else setTimeout(() => frame(performance.now()), 16);
};

self.onmessage = (event: MessageEvent<FieldMessage>) => {
  const message = event.data;
  if (message.type === 'init') {
    renderer = createRenderer(message.canvas, message);
    renderer.resize(message.width, message.height, message.ratio);
    schedule();
  } else if (message.type === 'resize') renderer?.resize(message.width, message.height, message.ratio);
  else if (message.type === 'color') renderer?.color(message.value);
  else if (message.type === 'enter') renderer?.enter(message.x, message.y, message.t0);
  else renderer?.fade(message.to, message.ms);
};
