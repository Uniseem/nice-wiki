import { cn } from '@heroui/react';
import { useEffect, useRef, useState, type KeyboardEvent, type PointerEvent } from 'react';
import { useStoredState } from './hooks';

type PaneOptions = {
  widthKey: string; // 宽度记在本浏览器
  initial: number;
  min: number;
  max: number;
  collapsedWidth: number; // 折叠后的宽度：后台侧栏留一条图标窄栏，文档树为 0
  collapsed: boolean;
  onCollapsedChange: (collapsed: boolean) => void;
};

export type ResizablePane = ReturnType<typeof useResizablePane>;

// 可拖动宽度的侧栏：拖边上的把手改宽度（不小于最小宽度）；继续往里拖过最小宽度与折叠宽度的中点就自动折叠，
// 往外拖过这个阈值又展开。双击把手恢复默认宽度并展开；聚焦把手后方向键每次调 16px、回车切换折叠。
// 拖动中在 window 上跟踪指针：侧栏折叠时把手可能随之卸载，拖动照样能继续和结束。
export function useResizablePane({ widthKey, initial, min, max, collapsedWidth, collapsed, onCollapsedChange }: PaneOptions) {
  const [stored, setStored] = useStoredState(widthKey, initial);
  const [live, setLive] = useState<number | null>(null);
  const stop = useRef<(() => void) | null>(null);
  const width = Math.min(max, Math.max(min, live ?? stored));
  const threshold = (min + collapsedWidth) / 2;
  const clamp = (raw: number) => Math.min(max, Math.max(min, raw));
  const direction = (element: HTMLElement) => (getComputedStyle(element).direction === 'rtl' ? -1 : 1);
  useEffect(() => () => stop.current?.(), []);

  const handlers = {
    onPointerDown(event: PointerEvent<HTMLElement>) {
      if (event.button !== 0) return;
      event.preventDefault();
      const origin = event.clientX;
      const dir = direction(event.currentTarget);
      const start = collapsed ? collapsedWidth : width;
      let folded = collapsed;
      let last = width;
      let moved = false;
      const move = (next: globalThis.PointerEvent) => {
        moved = true;
        const raw = start + (next.clientX - origin) * dir;
        if ((raw < threshold) !== folded) {
          folded = !folded;
          onCollapsedChange(folded);
        }
        if (!folded) setLive((last = clamp(raw)));
      };
      const end = () => {
        window.removeEventListener('pointermove', move);
        window.removeEventListener('pointerup', end);
        window.removeEventListener('pointercancel', end);
        document.documentElement.classList.remove('nw-resizing');
        stop.current = null;
        if (!folded) setStored(last);
        setLive(null);
        // 侧栏折叠时把手已卸载，浏览器会把松手后的点击派给指针下的元素（比如一个链接）；拖动过就吞掉这一次点击。
        if (moved) {
          window.addEventListener('click', swallow, { capture: true, once: true });
          setTimeout(() => window.removeEventListener('click', swallow, { capture: true }));
        }
      };
      window.addEventListener('pointermove', move);
      window.addEventListener('pointerup', end);
      window.addEventListener('pointercancel', end);
      document.documentElement.classList.add('nw-resizing');
      stop.current = end;
      setLive(width);
    },
    onDoubleClick() {
      setStored(initial);
      onCollapsedChange(false);
    },
    onKeyDown(event: KeyboardEvent<HTMLElement>) {
      if (event.key === 'Enter') { event.preventDefault(); onCollapsedChange(!collapsed); return; }
      const step = event.key === 'ArrowRight' ? 16 : event.key === 'ArrowLeft' ? -16 : 0;
      if (!step) return;
      event.preventDefault();
      const raw = (collapsed ? collapsedWidth : width) + step * direction(event.currentTarget);
      if ((raw < threshold) !== collapsed) onCollapsedChange(!collapsed);
      if (raw >= threshold) setStored(clamp(raw));
    },
  };
  return { width: collapsed ? collapsedWidth : width, min, max, collapsed, dragging: live !== null, handlers };
}

function swallow(event: MouseEvent) {
  event.preventDefault();
  event.stopPropagation();
}

// 侧栏边上的拖动把手：放在侧栏（定位元素）里，贴着它靠正文的一边；平时看不见，悬停、聚焦或拖动时显出一条细线。
export function ResizeHandle({ pane, label, className }: { pane: ResizablePane; label: string; className?: string }) {
  return (
    <div
      aria-label={label}
      aria-orientation="vertical"
      aria-valuemax={pane.max}
      aria-valuemin={pane.min}
      aria-valuenow={pane.width}
      className={cn('group/handle absolute inset-y-0 end-0 z-20 w-3 translate-x-1/2 cursor-col-resize touch-none outline-none rtl:-translate-x-1/2', className)}
      data-dragging={pane.dragging || undefined}
      role="separator"
      tabIndex={0}
      {...pane.handlers}
    >
      <span className="absolute inset-y-0 start-1/2 w-0.5 -translate-x-1/2 rounded-full transition-colors group-hover/handle:bg-foreground/15 group-focus-visible/handle:bg-focus group-data-[dragging=true]/handle:bg-foreground/30" />
    </div>
  );
}
