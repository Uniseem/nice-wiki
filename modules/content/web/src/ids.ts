import { Extension } from '@tiptap/core';
import { Plugin, PluginKey } from '@tiptap/pm/state';
import { newId } from './convert';

const blockTypes = ['paragraph', 'heading', 'blockquote', 'horizontalRule', 'bulletList', 'orderedList', 'taskList', 'codeBlock'];
const itemTypes = ['listItem', 'taskItem'];

// 给每个块/列表项补稳定 ID；粘贴或拆分产生重复 ID 时，保留第一个，其余换新 ID。
export const StableIds = Extension.create({
  name: 'stableIds',
  addGlobalAttributes() {
    return [
      { types: blockTypes, attributes: { blockId: { default: null, rendered: false } } },
      { types: itemTypes, attributes: { itemId: { default: null, rendered: false } } },
    ];
  },
  addProseMirrorPlugins() {
    return [new Plugin({
      key: new PluginKey('stableIds'),
      appendTransaction(_transactions, _old, state) {
        const seen = new Set<string>();
        const tr = state.tr;
        let changed = false;
        state.doc.descendants((node, pos) => {
          const key = blockTypes.includes(node.type.name) ? 'blockId' : itemTypes.includes(node.type.name) ? 'itemId' : null;
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
