import { createContext, useContext, useEffect, useSyncExternalStore, type ReactNode } from 'react';
import { createPortal } from 'react-dom';

// 后台顶栏里的“页面标题区”插槽：整屏页面（如文档）在宽屏时把自己的顶栏内容放进后台顶栏，只留一层顶栏；
// 窄屏放不下时照常渲染在页面里。claim 记录有没有页面正在占用，占用时后台顶栏不再显示自己的页面路径。
export type HeaderSlot = { element: HTMLElement | null; claim: (on: boolean) => void };
export const HeaderSlotContext = createContext<HeaderSlot | null>(null);

const wideQuery = '(min-width: 64rem)';
function subscribeWide(notify: () => void) {
  const query = matchMedia(wideQuery);
  query.addEventListener('change', notify);
  return () => query.removeEventListener('change', notify);
}

export function InHeader({ className, children }: { className: string; children: ReactNode }) {
  const slot = useContext(HeaderSlotContext);
  const wide = useSyncExternalStore(subscribeWide, () => matchMedia(wideQuery).matches, () => false);
  const target = wide ? slot?.element ?? null : null;
  useEffect(() => {
    if (!target || !slot) return;
    slot.claim(true);
    return () => slot.claim(false);
  }, [target, slot]);
  return target ? createPortal(children, target) : <header className={className}>{children}</header>;
}
