import { useSyncExternalStore } from 'react';

// 后台只有一层页面路由（/admin/<页面>），不需要完整路由库：history + 订阅即可。
const listeners = new Set<() => void>();
let guard: (() => boolean | Promise<boolean>) | null = null;

function emit(): void {
  listeners.forEach((listener) => listener());
}

// 每条历史记录带序号，浏览器后退/前进时才能算出走了几步：先退回原处过离开检查，同意后再走过去。
let index = 0;
let restoring = false;
let proceeding = false;

function entryIndex(state: unknown): number {
  const value = (state as { nwIndex?: unknown } | null)?.nwIndex;
  return typeof value === 'number' ? value : 0;
}

if (typeof window !== 'undefined') {
  index = entryIndex(history.state);
  history.replaceState({ ...(history.state ?? {}), nwIndex: index }, '');
  addEventListener('popstate', (event) => {
    const target = entryIndex(event.state);
    const delta = target - index;
    index = target;
    if (restoring) { restoring = false; return; }
    if (proceeding || !guard || delta === 0) { proceeding = false; emit(); return; }
    restoring = true;
    history.go(-delta);
    void confirmLeave().then((ok) => {
      if (!ok) return;
      proceeding = true;
      history.go(delta);
    });
  });
}

// 页面有未保存内容时可登记一个守卫（可以先异步保存），返回 false 表示拦下这次跳转。
// 返回取消登记的函数：只在守卫仍是自己时才清掉，避免后挂载的页面守卫被先卸载的页面清除。
export function setNavigationGuard(next: () => boolean | Promise<boolean>): () => void {
  guard = next;
  return () => { if (guard === next) guard = null; };
}

// 页面内部切换（不改路由，比如换一篇文档）也要先过离开检查。
export function confirmLeave(): Promise<boolean> {
  return Promise.resolve(guard ? guard() : true);
}

export function navigate(path: string, replace = false): void {
  if (path === location.pathname + location.search) return;
  void confirmLeave().then((ok) => {
    if (!ok) return;
    if (!replace) index++;
    history[replace ? 'replaceState' : 'pushState']({ nwIndex: index }, '', path);
    emit();
  });
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function usePath(): string {
  return useSyncExternalStore(subscribe, () => location.pathname);
}

export function useSearch(): string {
  return useSyncExternalStore(subscribe, () => location.search);
}
