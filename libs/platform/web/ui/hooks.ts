import { useCallback, useEffect, useState, useSyncExternalStore, type DependencyList } from 'react';
import { currentGrants, currentSite, errorText, subscribeGrants } from './api';
import { notify } from './feedback';

export type Loaded<T> = {
  data: T | undefined;
  error: unknown;
  loading: boolean;
  reload: () => void;
  setData: (update: T | ((previous: T | undefined) => T)) => void;
};

// 读取型数据：依赖变化或 reload 时重新加载，组件卸载或再次加载时取消上一次请求。
export function useLoad<T>(load: (signal: AbortSignal) => Promise<T>, deps: DependencyList): Loaded<T> {
  const [data, setData] = useState<T | undefined>(undefined);
  const [error, setError] = useState<unknown>(undefined);
  const [loading, setLoading] = useState(true);
  const [tick, setTick] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(undefined);
    load(controller.signal).then(
      (value) => { if (!controller.signal.aborted) { setData(value); setLoading(false); } },
      (problem) => { if (!controller.signal.aborted) { setError(problem); setLoading(false); } },
    );
    return () => controller.abort();
  }, [...deps, tick]);
  const reload = useCallback(() => setTick((value) => value + 1), []);
  return { data, error, loading, reload, setData: setData as Loaded<T>['setData'] };
}

// 写操作：统一的忙碌状态、成功提示和错误提示；失败时返回 undefined。
export function useAction() {
  const [busy, setBusy] = useState(false);
  const run = useCallback(async <R>(action: () => Promise<R>, success?: string): Promise<R | undefined> => {
    setBusy(true);
    try {
      const result = await action();
      if (success) notify.success(success);
      return result;
    } catch (problem) {
      notify.error(errorText(problem));
      return undefined;
    } finally {
      setBusy(false);
    }
  }, []);
  return { busy, run };
}

// 当前会话的授权项（登录、刷新、改口令后会变化）。只决定显示什么，真正的权限在服务端校验。
export function useGrants(): string[] {
  return useSyncExternalStore(subscribeGrants, currentGrants);
}

// 后台当前选中的站点（多站点）；切换站点时授权项也跟着变。
export function useCurrentSite(): string {
  return useSyncExternalStore(subscribeGrants, currentSite);
}

export function useHasGrant(...grants: string[]): boolean {
  const current = useGrants();
  return grants.some((grant) => current.includes(grant));
}

// 只影响本浏览器显示的界面偏好（侧栏收起、树的展开状态等）：存本地存储，读写失败时退回默认值。
export function useStoredState<T>(key: string, initial: T): [T, (update: T | ((previous: T) => T)) => void] {
  const [value, setValue] = useState<T>(() => {
    try {
      const raw = localStorage.getItem(key);
      return raw === null ? initial : (JSON.parse(raw) as T);
    } catch { return initial; }
  });
  const update = useCallback((next: T | ((previous: T) => T)) => {
    setValue((previous) => {
      const resolved = typeof next === 'function' ? (next as (previous: T) => T)(previous) : next;
      try { localStorage.setItem(key, JSON.stringify(resolved)); } catch { /* 隐私模式下只在本页生效 */ }
      return resolved;
    });
  }, [key]);
  return [value, update];
}
