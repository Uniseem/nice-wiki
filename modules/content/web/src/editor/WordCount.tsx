import type { Node as PMNode } from '@tiptap/pm/model';
import { useEditorState, type Editor } from '@tiptap/react';
import { t } from '@ui';

const cjk = /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}]/gu;
const word = /[\p{L}\p{N}]+(?:['’][\p{L}\p{N}]+)*/gu;
const counted = new WeakMap<PMNode, number>();

// 字数：中日韩文字一字算一个，其余语言按词算；同一份文档（只是移动光标）不重算。
function countWords(doc: PMNode): number {
  const cached = counted.get(doc);
  if (cached !== undefined) return cached;
  const text = doc.textBetween(0, doc.content.size, ' ', ' ');
  const asian = text.match(cjk)?.length ?? 0;
  const others = text.replace(cjk, ' ').match(word)?.length ?? 0;
  counted.set(doc, asian + others);
  return asian + others;
}

export function WordCount({ editor }: { editor: Editor }) {
  const count = useEditorState({ editor, selector: ({ editor: e }) => countWords(e.state.doc) });
  return <span>{t('{0} 字', count.toLocaleString())}</span>;
}
