import { Extension } from '@tiptap/core';
import { Plugin, PluginKey } from '@tiptap/pm/state';
import { newId } from './convert';

// 每类节点用哪个属性保存稳定 ID（与 content 块 schema 一致）。
const idKey: Record<string, string> = {
  paragraph: 'blockId', heading: 'blockId', blockquote: 'blockId', horizontalRule: 'blockId', bulletList: 'blockId', orderedList: 'blockId',
  taskList: 'blockId', codeBlock: 'blockId', image: 'blockId', video: 'blockId', table: 'blockId', callout: 'blockId', mathBlock: 'blockId', attachment: 'blockId', columns: 'blockId', column: 'columnId', drawing: 'blockId',
  listItem: 'itemId', taskItem: 'itemId', tableRow: 'rowId', tableCell: 'cellId', tableHeader: 'cellId',
};

// 给每个节点补稳定 ID；粘贴或拆分产生重复 ID 时，保留第一个，其余换新 ID。
export const StableIds = Extension.create({
  name: 'stableIds',
  addGlobalAttributes() {
    const groups = new Map<string, string[]>();
    for (const [type, key] of Object.entries(idKey)) groups.set(key, [...(groups.get(key) ?? []), type]);
    return [...groups].map(([key, types]) => ({ types, attributes: { [key]: { default: null, rendered: false } } }));
  },
  addProseMirrorPlugins() {
    return [new Plugin({
      key: new PluginKey('stableIds'),
      appendTransaction(_transactions, _old, state) {
        const seen = new Set<string>();
        const tr = state.tr;
        let changed = false;
        state.doc.descendants((node, pos) => {
          const key = idKey[node.type.name];
          if (!key) return;
          const current = node.attrs[key] as string | null;
          if (!current || seen.has(current)) {
            tr.setNodeMarkup(pos, undefined, { ...node.attrs, [key]: newId() });
            changed = true;
          } else seen.add(current);
        });
        return changed ? tr : null;
      },
    })];
  },
});
