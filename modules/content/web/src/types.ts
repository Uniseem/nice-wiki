import { t } from '@ui';
// content 域接口返回的文档元数据、草稿与编辑锁（只列界面用到的字段）。

export type DocState = 'draft' | 'published' | 'trash' | 'purged';

export type Meta = {
  documentId: string;
  title: string;
  parentId: string | null;
  orderKey: string;
  state: DocState;
  pathSegment: string | null;
  seoDescription: string | null;
  revision: number;
  publishedReference?: { contentHash: string } | null;
  shareImage?: unknown;
  sourceLanguage?: string;
};

export type Draft = { draftVersion: number; revisionId: string; bodyHash: string };

export type Lease = {
  leaseId: string;
  fencingToken: number;
  serverBootId: string;
  heartbeatSequence: number;
  inputSequence: number;
  holderAccountName?: string;
};

export type DraftRead = { metadata: Meta; draft: Draft; body: { content: unknown[] } };

export type Tree = { documents: Meta[]; treeRevision: number };

export type HistoryEntry = { historyId: string; createdAt: string; authorAccountName: string; operation: 'publish' | 'withdraw' | 'rollback' | string };

export const operationLabel: Record<string, string> = { publish: t('发布'), withdraw: t('撤回'), rollback: t('回滚') };

export function liveDocs(docs: Meta[]): Meta[] {
  return docs.filter((doc) => doc.state === 'draft' || doc.state === 'published');
}

export function byOrder(a: Meta, b: Meta): number {
  return a.orderKey < b.orderKey ? -1 : a.orderKey > b.orderKey ? 1 : 0;
}

// 按父子关系和排序键展开成带深度的列表。
export function flatten(docs: Meta[]): { doc: Meta; depth: number; hasChildren: boolean }[] {
  const children = new Map<string | null, Meta[]>();
  for (const doc of liveDocs(docs)) {
    const key = doc.parentId ?? null;
    children.set(key, [...(children.get(key) ?? []), doc]);
  }
  const out: { doc: Meta; depth: number; hasChildren: boolean }[] = [];
  const walk = (parent: string | null, depth: number) => {
    for (const doc of (children.get(parent) ?? []).sort(byOrder)) {
      out.push({ doc, depth, hasChildren: (children.get(doc.documentId) ?? []).length > 0 });
      walk(doc.documentId, depth + 1);
    }
  };
  walk(null, 0);
  return out;
}

// 文档自己和所有后代（移动时不能放进它们下面）。
export function subtree(docs: Meta[], id: string): Set<string> {
  const banned = new Set<string>([id]);
  let grew = true;
  while (grew) {
    grew = false;
    for (const doc of docs) {
      if (doc.parentId && banned.has(doc.parentId) && !banned.has(doc.documentId)) { banned.add(doc.documentId); grew = true; }
    }
  }
  return banned;
}

export async function sha256Hex(text: string): Promise<string> {
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(text));
  return Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, '0')).join('');
}

// 页面路径只允许小写字母、数字和连字符；标题是英文时给出建议，中文标题给随机路径，由作者再改。
export const pathPattern = /^[a-z0-9]+(-[a-z0-9]+)*$/;
export function suggestPath(title: string): string {
  const slug = title.toLowerCase().normalize('NFKD').replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 60).replace(/-+$/g, '');
  return slug && pathPattern.test(slug) ? slug : `page-${crypto.getRandomValues(new Uint32Array(1))[0].toString(36).slice(0, 6)}`;
}
