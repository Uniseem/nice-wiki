import { Extension } from '@tiptap/core';
import type { Node as PMNode } from '@tiptap/pm/model';
import { Plugin, PluginKey } from '@tiptap/pm/state';
import { Decoration, DecorationSet } from '@tiptap/pm/view';

// 历史版本里标出这次发布新增、修改的块（按稳定 blockId）。父块的哈希包含子块，
// 子块有变化时只标最里层变化的块，不把整个列表或提示框都标上。
export const DiffHighlight = Extension.create<{ added: Set<string>; changed: Set<string> }>({
  name: 'diffHighlight',
  addOptions() {
    return { added: new Set<string>(), changed: new Set<string>() };
  },
  addProseMirrorPlugins() {
    const { added, changed } = this.options;
    const marked = (node: PMNode) => {
      const id = node.attrs.blockId as string | undefined;
      return !!id && (added.has(id) || changed.has(id));
    };
    const hasMarkedChild = (node: PMNode) => {
      let found = false;
      node.descendants((child) => { if (!found && marked(child)) found = true; return !found; });
      return found;
    };
    return [new Plugin({
      key: new PluginKey('diffHighlight'),
      props: {
        decorations(state) {
          const decorations: Decoration[] = [];
          state.doc.descendants((node, pos) => {
            const id = node.attrs.blockId as string | undefined;
            if (!id) return true;
            if (added.has(id)) { decorations.push(Decoration.node(pos, pos + node.nodeSize, { class: 'nw-diff-added' })); return false; }
            if (changed.has(id) && !hasMarkedChild(node)) { decorations.push(Decoration.node(pos, pos + node.nodeSize, { class: 'nw-diff-changed' })); return false; }
            return true;
          });
          return DecorationSet.create(state.doc, decorations);
        },
      },
    })];
  },
});
