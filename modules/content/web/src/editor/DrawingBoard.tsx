// 画板：Excalidraw（MIT）。单独成块，由 DrawingDialog 懒加载，不用画板时不影响编辑器体积。
import { Excalidraw, exportToSvg } from '@excalidraw/excalidraw';
import '@excalidraw/excalidraw/index.css';
import { useTheme } from '@heroui/react';
import { useEffect, useRef } from 'react';
import { normalizeScene } from './drawing';

// media 对 SVG 只放行固定的一组元素，不含 filter/metadata 等；导出后清理掉。
const allowed = new Set(['svg', 'g', 'defs', 'path', 'rect', 'circle', 'ellipse', 'line', 'polyline', 'polygon', 'text', 'tspan', 'title', 'desc', 'clipPath', 'mask', 'linearGradient', 'radialGradient', 'stop', 'pattern', 'use', 'image', 'a', 'marker', 'symbol']);

// 深色版：Excalidraw 靠 CSS 滤镜 invert(93%) hue-rotate(180deg) 实现，滤镜不允许，这里把同样的变换直接算到颜色上。
function darkColor(value: string): string {
  const m = /^#([0-9a-f]{6})$/i.exec(value.trim());
  if (!m) return value;
  const n = parseInt(m[1], 16);
  const [r, g, b] = [(n >> 16) & 255, (n >> 8) & 255, n & 255].map((c) => 0.93 - 0.86 * (c / 255));
  const clamp = (v: number) => Math.round(Math.min(1, Math.max(0, v)) * 255).toString(16).padStart(2, '0');
  return '#' + clamp(-0.574 * r + 1.43 * g + 0.144 * b) + clamp(0.426 * r + 0.43 * g + 0.144 * b) + clamp(0.426 * r + 1.43 * g - 0.856 * b);
}

function cleanSvg(svg: SVGSVGElement, dark: boolean): string {
  for (const el of Array.from(svg.querySelectorAll('*'))) {
    if (!allowed.has(el.localName)) { el.remove(); continue; }
    el.removeAttribute('filter');
    if (dark) for (const attr of ['stroke', 'fill', 'stop-color']) { const v = el.getAttribute(attr); if (v) el.setAttribute(attr, darkColor(v)); }
  }
  svg.removeAttribute('filter');
  return new XMLSerializer().serializeToString(svg);
}

export type BoardResult = { scene: string; lightSVG: string; darkSVG: string };
export type BoardHandle = { snapshot(): Promise<BoardResult | null> };
type Scene = { elements?: any[]; appState?: { viewBackgroundColor?: string }; files?: Record<string, any> } | null;

export default function DrawingBoard({ initial, onReady }: { initial: Scene; onReady: (handle: BoardHandle) => void }) {
  const apiRef = useRef<any>(null);
  const { resolvedTheme } = useTheme();

  useEffect(() => {
    onReady({
      async snapshot() {
        const api = apiRef.current;
        if (!api) return null;
        const elements = api.getSceneElements().filter((e: any) => !e.isDeleted);
        if (!elements.length) return null;
        const appState = api.getAppState();
        const files = api.getFiles();
        const scene = normalizeScene(elements, appState, files);
        const svg = async (dark: boolean) => cleanSvg(await exportToSvg({
          elements, files, exportPadding: 16, skipInliningFonts: true,
          appState: { exportBackground: true, viewBackgroundColor: appState.viewBackgroundColor ?? '#ffffff', exportWithDarkMode: false },
        }), dark);
        return { scene: JSON.stringify(scene), lightSVG: await svg(false), darkSVG: await svg(true) };
      },
    });
  }, [onReady]);

  const data = initial ? { elements: initial.elements ?? [], appState: { viewBackgroundColor: initial.appState?.viewBackgroundColor ?? '#ffffff' }, files: initial.files ?? {} } : undefined;
  return (
    <div className="h-full w-full">
      <Excalidraw excalidrawAPI={(api: any) => { apiRef.current = api; }} initialData={data as any} langCode="zh-CN" theme={resolvedTheme === 'dark' ? 'dark' : 'light'} />
    </div>
  );
}
