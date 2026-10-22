import { t } from '@ui';
// 编辑器 JSON 与 content 域块 schema 的互转。
// 服务端 schema 对 attrs 是封闭集合（多一个字段都会被拒绝），所以这里逐类型显式列出。

export type JSONNode = {
  type: string;
  attrs?: Record<string, any>;
  content?: JSONNode[];
  marks?: { type: string; attrs?: Record<string, any> }[];
  text?: string;
};

export function newId(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

const simpleMarks = new Set(['bold', 'italic', 'underline', 'strike', 'code']);
// 站内链接在编辑器里的占位地址：真正的目标在 linkTarget 属性里。
export const internalHref = '#nw-internal-link';

function inlines(nodes: JSONNode[] | undefined): any[] {
  const out: any[] = [];
  for (const node of nodes ?? []) {
    if (node.type === 'hardBreak') out.push({ type: 'hardBreak' });
    else if (node.type === 'text' && node.text) {
      const marks = (node.marks ?? []).flatMap((mark) => {
        if (simpleMarks.has(mark.type)) return [{ type: mark.type }];
        if (mark.type === 'link') {
          const href = mark.attrs?.href ?? '';
          const title = mark.attrs?.linkTitle ?? null;
          // 改成外部地址时以新地址为准；地址没动的站内链接按原目标保存。
          if (/^(https?:|mailto:)/i.test(href)) return [{ type: 'link', attrs: { target: { kind: 'external', href }, title } }];
          if (mark.attrs?.linkTarget?.kind === 'internal') return [{ type: 'link', attrs: { target: mark.attrs.linkTarget, title } }];
          return [];
        }
        if (mark.type === 'highlight') return [{ type: 'highlight', attrs: { color: mark.attrs?.color ?? null } }];
        return []; // 暂未支持的 mark 不写入，避免服务端拒绝整篇
      });
      out.push({ type: 'text', text: node.text, marks });
    }
  }
  return out;
}

function blocks(nodes: JSONNode[] | undefined, inColumn = false): any[] {
  return (nodes ?? []).map((n) => block(n, inColumn));
}

// 2–4 栏等分；比例保留 6 位小数，余数补在最后一栏，保证总和为 1。
function equalRatios(count: number): number[] {
  const each = Math.floor(1e6 / count) / 1e6;
  const ratios = Array<number>(count).fill(each);
  ratios[count - 1] = Math.round((1 - each * (count - 1)) * 1e6) / 1e6;
  return ratios;
}

function block(node: JSONNode, inColumn = false): any {
  const blockId = node.attrs?.blockId ?? newId();
  switch (node.type) {
    case 'paragraph': return { type: 'paragraph', attrs: { blockId }, content: inlines(node.content) };
    case 'heading': return { type: 'heading', attrs: { blockId, level: Math.min(4, Math.max(2, node.attrs?.level ?? 2)) }, content: inlines(node.content) };
    case 'blockquote': return { type: 'blockquote', attrs: { blockId }, content: blocks(node.content, inColumn) };
    case 'horizontalRule': return { type: 'horizontalRule', attrs: { blockId } };
    case 'bulletList': return { type: 'bulletList', attrs: { blockId }, content: (node.content ?? []).map(listItem) };
    case 'orderedList': return { type: 'orderedList', attrs: { blockId, start: node.attrs?.start ?? 1 }, content: (node.content ?? []).map(listItem) };
    case 'taskList': return { type: 'taskList', attrs: { blockId }, content: (node.content ?? []).map(taskItem) };
    case 'codeBlock': {
      const text = (node.content ?? []).map((c) => c.text ?? '').join('');
      return { type: 'codeBlock', attrs: { blockId, language: node.attrs?.language || null }, content: text ? [{ type: 'text', text }] : [] };
    }
    case 'callout': return { type: 'callout', attrs: { blockId, kind: node.attrs?.kind ?? 'info' }, content: blocks(node.content, inColumn) };
    case 'columns': {
      if (inColumn) throw new Error(t('分栏不能嵌套分栏'));
      const cols = node.content ?? [];
      const kept = node.attrs?.widthRatios;
      const widthRatios = Array.isArray(kept) && kept.length === cols.length ? kept : equalRatios(cols.length);
      return { type: 'columns', attrs: { blockId, widthRatios }, content: cols.map((c) => ({ type: 'column', attrs: { columnId: c.attrs?.columnId ?? newId() }, content: blocks(c.content, true) })) };
    }
    case 'mathBlock': return { type: 'mathBlock', attrs: { blockId, latex: node.attrs?.latex || ' ' } };
    case 'attachment': {
      const a = node.attrs?.asset;
      if (!a) throw new Error(t('附件缺少资源引用'));
      return { type: 'attachment', attrs: { blockId, asset: { assetId: a.assetId, version: a.version, sha256: a.sha256, mediaType: a.mediaType, byteSize: a.byteSize }, displayName: node.attrs?.displayName || t('附件') } };
    }
    case 'drawing': {
      const { scene, lightSVG, darkSVG } = node.attrs ?? {};
      if (!scene || !lightSVG || !darkSVG) throw new Error(t('画板缺少资源引用'));
      const ref = (a: any) => ({ assetId: a.assetId, version: a.version, sha256: a.sha256, mediaType: a.mediaType, byteSize: a.byteSize });
      return { type: 'drawing', attrs: { blockId, scene: ref(scene), lightSVG: ref(lightSVG), darkSVG: ref(darkSVG), widthPercent: node.attrs?.widthPercent ?? null, caption: node.attrs?.caption ?? [] } };
    }
    case 'table': return { type: 'table', attrs: { blockId }, content: (node.content ?? []).map(tableRow) };
    case 'video': {
      const a = node.attrs?.source?.asset;
      if (!a) throw new Error(t('视频缺少资源引用'));
      const poster = node.attrs?.poster;
      return { type: 'video', attrs: { blockId, source: { kind: 'uploaded', asset: { assetId: a.assetId, version: a.version, sha256: a.sha256, mediaType: a.mediaType, byteSize: a.byteSize } }, title: node.attrs?.title ?? '', description: node.attrs?.description ?? '', poster: poster ? { assetId: poster.assetId, version: poster.version, sha256: poster.sha256, mediaType: poster.mediaType, byteSize: poster.byteSize } : null, widthPercent: node.attrs?.widthPercent ?? null } };
    }
    case 'image': {
      const alt = typeof node.attrs?.alt === 'string' && node.attrs.alt !== '' ? node.attrs.alt : null;
      const a = node.attrs?.asset;
      if (!a) throw new Error(t('图片缺少资源引用'));
      const altSource = alt ? (node.attrs?.altSource ?? 'manual') : 'auto';
      return { type: 'image', attrs: { blockId, asset: { assetId: a.assetId, version: a.version, sha256: a.sha256, mediaType: a.mediaType, byteSize: a.byteSize }, alt, altSource, caption: node.attrs?.caption ?? [], widthPercent: node.attrs?.widthPercent ?? null } };
    }
    default: throw new Error(t('编辑器含暂不支持保存的块：{0}', node.type));
  }
}

function tableRow(node: JSONNode): any {
  return { type: 'tableRow', attrs: { rowId: node.attrs?.rowId ?? newId() }, content: (node.content ?? []).map(tableCell) };
}

function tableCell(node: JSONNode): any {
  const colspan = node.attrs?.colspan ?? 1;
  const widths = Array.isArray(node.attrs?.colwidth) && node.attrs!.colwidth.length === colspan && node.attrs!.colwidth.every((w: number) => w >= 40 && w <= 1200) ? node.attrs!.colwidth : null;
  return { type: node.type === 'tableHeader' ? 'tableHeader' : 'tableCell', attrs: { cellId: node.attrs?.cellId ?? newId(), colspan, rowspan: node.attrs?.rowspan ?? 1, colwidth: widths, backgroundColor: node.attrs?.backgroundColor ?? null }, content: blocks(node.content) };
}

function listItem(node: JSONNode): any {
  return { type: 'listItem', attrs: { itemId: node.attrs?.itemId ?? newId() }, content: blocks(node.content) };
}

function taskItem(node: JSONNode): any {
  return { type: 'taskItem', attrs: { itemId: node.attrs?.itemId ?? newId(), checked: !!node.attrs?.checked }, content: blocks(node.content) };
}

export function toBody(doc: JSONNode): { schemaVersion: 1; type: 'doc'; content: any[] } {
  return { schemaVersion: 1, type: 'doc', content: blocks(doc.content) };
}

// 服务端 → 编辑器：除 attrs 里的稳定 ID 外结构基本一致；不认识的块原样保留为错误提示。
export function fromBody(body: { content: JSONNode[] }): JSONNode {
  const walk = (node: JSONNode): JSONNode => {
    const copy: JSONNode = { type: node.type };
    if (node.attrs) copy.attrs = { ...node.attrs };
    if (node.type === 'codeBlock') {
      copy.content = (node.content ?? []).filter((c) => c.text).map((c) => ({ type: 'text', text: c.text }));
    } else if (node.content) {
      copy.content = node.content.map(walk);
    }
    if (node.text !== undefined) copy.text = node.text;
    if (node.marks) {
      copy.marks = node.marks.map((m) => {
        if (m.type !== 'link') return { ...m };
        const target = m.attrs?.target;
        return target?.kind === 'internal'
          ? { type: 'link', attrs: { href: internalHref, linkTarget: target, linkTitle: m.attrs?.title ?? null } }
          : { type: 'link', attrs: { href: target?.href ?? '', linkTarget: null, linkTitle: m.attrs?.title ?? null } };
      });
    }
    return copy;
  };
  return { type: 'doc', content: body.content.map(walk) };
}
