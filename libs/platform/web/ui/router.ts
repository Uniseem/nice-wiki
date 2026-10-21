import { useSyncExternalStore } from 'react';

// 后台只有一层页面路由（/admin/<页面>），不需要完整路由库：history + 订阅即可。
const listeners = new Set<() => void>();
let guard: (() => boolean) | null = null;

function emit(): void {
  listeners.forEach((listener) => listener());
}

if (typeof window !== 'undefined') addEventListener('popstate', emit);

// 页面有未保存内容时可登记一个守卫，返回 false 表示拦下这次跳转。
export function setNavigationGuard(next: (() => boolean) | null): void {
  guard = next;
}

export function navigate(path: string, replace = false): void {
  if (path === location.pathname + location.search) return;
  if (guard && !guard()) return;
  history[replace ? 'replaceState' : 'pushState'](null, '', path);
  emit();
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
