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

function inlines(nodes: JSONNode[] | undefined): any[] {
  const out: any[] = [];
  for (const node of nodes ?? []) {
    if (node.type === 'hardBreak') out.push({ type: 'hardBreak' });
    else if (node.type === 'text' && node.text) {
      const marks = (node.marks ?? []).flatMap((mark) => {
        if (simpleMarks.has(mark.type)) return [{ type: mark.type }];
        if (mark.type === 'highlight') return [{ type: 'highlight', attrs: { color: mark.attrs?.color ?? null } }];
        return []; // 暂未支持的 mark（如链接）不写入，避免服务端拒绝整篇
      });
      out.push({ type: 'text', text: node.text, marks });
    }
  }
  return out;
}

function blocks(nodes: JSONNode[] | undefined): any[] {
  return (nodes ?? []).map(block);
}

function block(node: JSONNode): any {
  const blockId = node.attrs?.blockId ?? newId();
  switch (node.type) {
    case 'paragraph': return { type: 'paragraph', attrs: { blockId }, content: inlines(node.content) };
    case 'heading': return { type: 'heading', attrs: { blockId, level: Math.min(4, Math.max(2, node.attrs?.level ?? 2)) }, content: inlines(node.content) };
    case 'blockquote': return { type: 'blockquote', attrs: { blockId }, content: blocks(node.content) };
    case 'horizontalRule': return { type: 'horizontalRule', attrs: { blockId } };
    case 'bulletList': return { type: 'bulletList', attrs: { blockId }, content: (node.content ?? []).map(listItem) };
    case 'orderedList': return { type: 'orderedList', attrs: { blockId, start: node.attrs?.start ?? 1 }, content: (node.content ?? []).map(listItem) };
    case 'taskList': return { type: 'taskList', attrs: { blockId }, content: (node.content ?? []).map(taskItem) };
    case 'codeBlock': {
      const text = (node.content ?? []).map((c) => c.text ?? '').join('');
      return { type: 'codeBlock', attrs: { blockId, language: node.attrs?.language || null }, content: text ? [{ type: 'text', text }] : [] };
    }
    default: throw new Error(`编辑器含暂不支持保存的块：${node.type}`);
  }
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
    if (node.marks) copy.marks = node.marks.map((m) => ({ ...m }));
    return copy;
  };
  return { type: 'doc', content: body.content.map(walk) };
}
