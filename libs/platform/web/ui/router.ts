import { useSyncExternalStore } from 'react';

// 后台只有一层页面路由（/admin/<页面>），不需要完整路由库：history + 订阅即可。
const listeners = new Set<() => void>();
let guard: (() => boolean | Promise<boolean>) | null = null;

function emit(): void {
  listeners.forEach((listener) => listener());
}

if (typeof window !== 'undefined') addEventListener('popstate', emit);

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
    history[replace ? 'replaceState' : 'pushState'](null, '', path);
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
