import { Bold, Code, Italic, Link as LinkIcon, Paintbrush, Strikethrough, Underline } from '@gravity-ui/icons';
import { NodeSelection } from '@tiptap/pm/state';
import { useEditorState, type Editor } from '@tiptap/react';
import { BubbleMenu } from '@tiptap/react/menus';
import { t } from '@ui';
import type { EditorActions } from './commands';
import { Toggle } from './Toolbar';

// 选中文字时在选区上方浮出的格式工具条（常见 wiki 的做法），不用把视线移到顶部工具栏。
// 代码块里没有行内格式，整块选中（图片等）有自己的工具，都不显示。
export function SelectionMenu({ editor, actions }: { editor: Editor; actions: EditorActions }) {
  const state = useEditorState({
    editor,
    selector: ({ editor: e }) => ({
      bold: e.isActive('bold'), italic: e.isActive('italic'), underline: e.isActive('underline'), strike: e.isActive('strike'),
      code: e.isActive('code'), highlight: e.isActive('highlight'), link: e.isActive('link'),
    }),
  });
  const chain = () => editor.chain().focus();
  return (
    <BubbleMenu
      className="z-30 flex items-center gap-0.5 rounded-xl border border-border bg-overlay p-1 shadow-overlay"
      editor={editor}
      options={{ placement: 'top', offset: 8 }}
      shouldShow={({ editor: e, state: s, from, to }) => e.isEditable && from !== to && !(s.selection instanceof NodeSelection) && !e.isActive('codeBlock') && s.doc.textBetween(from, to).trim() !== ''}
    >
      <Toggle label={t('加粗')} shortcut="⌘B" isSelected={state.bold} onPress={() => chain().toggleBold().run()}><Bold /></Toggle>
      <Toggle label={t('斜体')} shortcut="⌘I" isSelected={state.italic} onPress={() => chain().toggleItalic().run()}><Italic /></Toggle>
      <Toggle label={t('下划线')} shortcut="⌘U" isSelected={state.underline} onPress={() => chain().toggleUnderline().run()}><Underline /></Toggle>
      <Toggle label={t('删除线')} isSelected={state.strike} onPress={() => chain().toggleStrike().run()}><Strikethrough /></Toggle>
      <Toggle label={t('行内代码')} shortcut="⌘E" isSelected={state.code} onPress={() => chain().toggleCode().run()}><Code /></Toggle>
      <Toggle label={t('高亮')} isSelected={state.highlight} onPress={() => chain().toggleHighlight().run()}><Paintbrush /></Toggle>
      <span className="mx-0.5 h-5 w-px bg-separator" />
      <Toggle label={t('链接')} shortcut="⌘K" isSelected={state.link} onPress={actions.editLink}><LinkIcon /></Toggle>
    </BubbleMenu>
  );
}
