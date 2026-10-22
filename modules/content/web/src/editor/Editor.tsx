import type { Extensions, NodeViewProps } from '@tiptap/core';
import Highlight from '@tiptap/extension-highlight';
import Link from '@tiptap/extension-link';
import { Table, TableCell, TableHeader, TableRow } from '@tiptap/extension-table';
import TaskItem from '@tiptap/extension-task-item';
import TaskList from '@tiptap/extension-task-list';
import { Placeholder } from '@tiptap/extensions';
import { EditorContent, NodeViewWrapper, ReactNodeViewRenderer, useEditor, type Editor as TiptapEditor } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import { cn, ScrollShadow, toast } from '@heroui/react';
import { captureVideoPoster, errorText, uploadAsset, uploadImage, t } from '@ui';
import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent, type MutableRefObject, type ReactNode } from 'react';
import { internalHref, type JSONNode } from '../convert';
import type { Meta } from '../types';
import { Attachment, Callout, Column, Columns, jsonAttr, MathBlock } from './blocks';
import { BlockHandle } from './BlockHandle';
import { SelectionMenu } from './SelectionMenu';
import { filterCommands, insertBlock, type BlockCommand, type EditorActions } from './commands';
import { Drawing } from './drawing';
import { DrawingDialog, type DrawingAttrs } from './DrawingDialog';
import { StableIds } from './ids';
import { ImageBlock } from './image';
import { MathDialog, renderMath } from './MathDialog';
import { Outline } from './Outline';
import { LinkDialog, type LinkChoice } from './LinkDialog';
import { EditorToolbar } from './Toolbar';
import { VideoBlock } from './video';

const videoLimit = 500 << 20;
const kindLabel = { image: t('图片'), attachment: t('附件'), video: t('视频') } as const;

// 编辑器里的公式直接用 KaTeX 渲染成预览，双击打开公式对话框。
function MathView({ node, selected }: NodeViewProps) {
  const { html, error } = renderMath(String(node.attrs.latex ?? ''));
  return (
    <NodeViewWrapper className={cn('math-block', selected && 'ProseMirror-selectednode')} data-math="">
      {error ? <span className="text-danger">{t('公式有误：{0}', error)}</span> : <span dangerouslySetInnerHTML={{ __html: html }} />}
    </NodeViewWrapper>
  );
}
const MathBlockWithPreview = MathBlock.extend({ addNodeView() { return ReactNodeViewRenderer(MathView); } });

// 站内链接没有外部地址：目标文档/块和链接标题放在 data 属性里原样保留，href 只是占位。
const WikiLink = Link.extend({ addAttributes() { return { ...this.parent?.(), linkTarget: jsonAttr('linkTarget'), linkTitle: jsonAttr('linkTitle') }; } });
// 单元格背景色暂不能在编辑器里改，原样保留并显示。
const cellBackground = {
  backgroundColor: {
    default: null,
    parseHTML: (element: HTMLElement) => element.getAttribute('data-background'),
    renderHTML: (attrs: Record<string, unknown>) => (attrs.backgroundColor ? { 'data-background': attrs.backgroundColor, style: `background-color:${attrs.backgroundColor}` } : {}),
  },
};
const WikiTableCell = TableCell.extend({ addAttributes() { return { ...this.parent?.(), ...cellBackground }; } });
const WikiTableHeader = TableHeader.extend({ addAttributes() { return { ...this.parent?.(), ...cellBackground }; } });

// 正文用到的全部节点与标记：编辑器和只读的历史版本查看共用一份。
export function documentExtensions(): Extensions {
  return [
    // 链接需要 LinkTarget 结构，单独配置；标题只允许 H2–H4。
    StarterKit.configure({ heading: { levels: [2, 3, 4] }, link: false }),
    WikiLink.configure({ openOnClick: false, autolink: false, protocols: ['mailto'], HTMLAttributes: { rel: 'noopener noreferrer nofollow' } }),
    Table.configure({ resizable: false }), TableRow, WikiTableHeader, WikiTableCell,
    TaskList, TaskItem.configure({ nested: true, a11y: { checkboxLabel: (node) => node.firstChild?.textContent ? t('任务：{0}', node.firstChild.textContent) : t('空白任务') } }), Highlight.configure({ multicolor: true }),
    ImageBlock, VideoBlock, Callout, MathBlockWithPreview, Attachment, Columns, Column, Drawing,
  ];
}

type Slash = { left: number; top: number; query: string };

// 编辑区：吸顶工具栏 + 居中正文（header 放在正文上方，如标题）+ 宽屏时右侧大纲；滚动容器在这里。
export function Editor({ doc, docs, documentId, editable, toolbar, header, wide = false, editorRef: exposed, onChange }: {
  doc: JSONNode;
  docs: Meta[];
  documentId: string;
  editable: boolean;
  toolbar: boolean;
  header?: ReactNode | ((editor: TiptapEditor) => ReactNode);
  wide?: boolean;
  editorRef?: MutableRefObject<TiptapEditor | null>;
  onChange: (doc: JSONNode) => void;
}) {
  const wrapper = useRef<HTMLDivElement>(null);
  const scroller = useRef<HTMLDivElement>(null);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;
  const editableRef = useRef(editable);
  editableRef.current = editable;
  const [slash, setSlash] = useState<Slash | null>(null);
  const [slashIndex, setSlashIndex] = useState(0);
  const slashState = useRef<{ slash: Slash | null; index: number; items: BlockCommand[] }>({ slash: null, index: 0, items: [] });
  const [math, setMath] = useState<{ pos: number | null; latex: string | null } | null>(null);
  const [drawing, setDrawing] = useState<{ pos: number | null; initial: DrawingAttrs | null } | null>(null);
  const [link, setLink] = useState<{ href: string; documentId: string | null; editing: boolean } | null>(null);
  const imageInput = useRef<HTMLInputElement>(null);
  const videoInput = useRef<HTMLInputElement>(null);
  const attachInput = useRef<HTMLInputElement>(null);
  const editorRef = useRef<TiptapEditor | null>(null);

  const insertFile = useCallback(async (file: File, kind: 'image' | 'attachment' | 'video') => {
    const current = editorRef.current;
    if (!current) return;
    const label = kindLabel[kind];
    if (kind === 'video' && file.size > videoLimit) { toast.danger(t('视频超过 500MB'), { description: t('请压缩后再上传。') }); return; }
    const id = toast(t('正在上传{0}', label), { description: file.name, isLoading: true, timeout: 0 });
    try {
      const asset = kind === 'image' ? await uploadImage(file) : await uploadAsset(file, kind);
      const poster = kind === 'video' ? await captureVideoPoster(file) : null;
      const content = kind === 'image' ? { type: 'image', attrs: { asset, alt: null } }
        : kind === 'video' ? { type: 'video', attrs: { source: { kind: 'uploaded', asset }, title: file.name.replace(/\.[^.]+$/, '').slice(0, 500), description: '', poster, widthPercent: null } }
        : { type: 'attachment', attrs: { asset, displayName: file.name.slice(0, 255) } };
      insertBlock(current, content);
      toast.update(id, t('{0}已插入', label), { description: file.name, isLoading: false, timeout: 2500, variant: 'success' });
    } catch (problem) {
      toast.update(id, t('{0}上传失败', label), { description: errorText(problem), isLoading: false, timeout: 8000, variant: 'danger' });
    }
  }, []);

  const handleFiles = useCallback((files: FileList | null | undefined): boolean => {
    const images = Array.from(files ?? []).filter((file) => file.type.startsWith('image/'));
    if (!images.length || !editableRef.current) return false;
    void (async () => { for (const file of images) await insertFile(file, 'image'); })();
    return true;
  }, [insertFile]);

  const actions = useMemo<EditorActions>(() => ({
    pickImage: () => imageInput.current?.click(),
    pickVideo: () => videoInput.current?.click(),
    pickAttachment: () => attachInput.current?.click(),
    openDrawing: () => setDrawing({ pos: null, initial: null }),
    openMath: () => setMath({ pos: null, latex: null }),
    editLink: () => {
      const current = editorRef.current;
      if (!current) return;
      const attrs = current.getAttributes('link');
      const internal = attrs.linkTarget?.kind === 'internal';
      setLink({ href: internal ? '' : (attrs.href as string | undefined) ?? '', documentId: internal ? attrs.linkTarget.documentId : null, editing: current.isActive('link') });
    },
  }), []);

  // 有选中文字（或光标在已有链接上）就给它加/改链接；什么都没选时插入以标题或地址为文字的链接。
  const applyLink = (choice: LinkChoice) => {
    const current = editorRef.current;
    setLink(null);
    if (!current) return;
    const attrs = choice.kind === 'external'
      ? { href: choice.href, linkTarget: null }
      : { href: internalHref, linkTarget: { kind: 'internal', documentId: choice.documentId, blockId: null } };
    if (current.state.selection.empty && !current.isActive('link')) {
      current.chain().focus().insertContent({ type: 'text', text: choice.kind === 'external' ? choice.href : choice.title, marks: [{ type: 'link', attrs }] }).run();
    } else {
      current.chain().focus().extendMarkRange('link').setLink(attrs as { href: string }).run();
    }
  };

  // 斜杠菜单：在空段落里输入“/”，按名称或拼音关键字过滤，方向键选择、回车执行、Esc 关闭。
  const refreshSlash = useCallback((current: TiptapEditor) => {
    const { state, view } = current;
    const { $from, empty } = state.selection;
    const text = $from.parent.textContent;
    let next: Slash | null = null;
    if (editableRef.current && empty && $from.parent.type.name === 'paragraph' && text.startsWith('/') && $from.parentOffset === text.length && text.length <= 24) {
      const coords = view.coordsAtPos($from.pos);
      const host = wrapper.current?.getBoundingClientRect();
      next = { left: coords.left - (host?.left ?? 0), top: coords.bottom - (host?.top ?? 0) + 6, query: text.slice(1) };
    }
    const previous = slashState.current.slash;
    if (previous?.query === next?.query && previous?.left === next?.left && previous?.top === next?.top) return;
    slashState.current.slash = next;
    slashState.current.items = next ? filterCommands(next.query) : [];
    slashState.current.index = 0;
    setSlash(next);
    setSlashIndex(0);
  }, []);

  const runSlash = useCallback((command: BlockCommand) => {
    const current = editorRef.current;
    if (!current) return;
    const { $from } = current.state.selection;
    current.chain().focus().deleteRange({ from: $from.start(), to: $from.pos }).run();
    slashState.current = { slash: null, index: 0, items: [] };
    setSlash(null);
    command.run(current, actions);
  }, [actions]);

  const editor = useEditor({
    content: doc,
    editable,
    extensions: [
      ...documentExtensions(), StableIds,
      Placeholder.configure({ placeholder: t('输入正文，或输入 / 插入标题、列表、表格、图片…') }),
    ],
    editorProps: {
      attributes: { class: 'nw-prose', 'aria-label': t('文档正文') },
      // 粘贴或拖入图片文件：直接上传并插入。
      handlePaste: (_view, event) => handleFiles(event.clipboardData?.files),
      handleDrop: (_view, event) => handleFiles((event as DragEvent).dataTransfer?.files),
      handleKeyDown: (_view, event) => {
        const { slash: open, items, index } = slashState.current;
        if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k' && editorRef.current) { actions.editLink(); return true; }
        if (!open || !items.length) return false;
        if (event.key === 'ArrowDown') { slashState.current.index = (index + 1) % items.length; setSlashIndex(slashState.current.index); return true; }
        if (event.key === 'ArrowUp') { slashState.current.index = (index + items.length - 1) % items.length; setSlashIndex(slashState.current.index); return true; }
        if (event.key === 'Enter') { runSlash(items[index]); return true; }
        if (event.key === 'Escape') { slashState.current.slash = null; setSlash(null); return true; }
        return false;
      },
      // 双击画板或公式：重新打开对应的编辑框。
      handleDoubleClickOn: (_view, pos, node) => {
        if (!editableRef.current) return false;
        if (node.type.name === 'drawing') { setDrawing({ pos, initial: { scene: node.attrs.scene, lightSVG: node.attrs.lightSVG, darkSVG: node.attrs.darkSVG } }); return true; }
        if (node.type.name === 'mathBlock') { setMath({ pos, latex: String(node.attrs.latex ?? '') }); return true; }
        return false;
      },
    },
    // 文档有变化就通知保存（工具栏按钮、链接/公式对话框、上传完成时编辑器往往不在焦点上，不能以焦点判断）。
    // 只有插件补出来的变化（补稳定 ID、补末尾段落）不算修改：点进空文档不该触发保存，下次真正编辑时一并存。
    onUpdate: ({ editor: current, transaction }) => { if (transaction.docChanged) onChangeRef.current(current.getJSON() as JSONNode); },
    onTransaction: ({ editor: current }) => refreshSlash(current),
  });
  editorRef.current = editor;
  if (exposed) exposed.current = editor;

  // 切换只读不是修改：不发 update 事件，避免打开文档就触发一次保存。
  useEffect(() => { editor?.setEditable(editable, false); }, [editor, editable]);

  const pick = (kind: 'image' | 'attachment' | 'video') => (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (file) void insertFile(file, kind);
  };

  if (!editor) return null;
  const items = slash ? slashState.current.items : [];

  return (
    <div ref={scroller} className="relative min-h-0 flex-1 overflow-y-auto">
      {toolbar && (
        <div className="sticky top-0 z-20 border-b border-separator bg-background/90 px-3 py-1.5 backdrop-blur-md sm:px-6">
          <EditorToolbar actions={actions} editable={editable} editor={editor} />
        </div>
      )}
      <div className="flex justify-center gap-10 px-5 sm:px-10">
        <div ref={wrapper} className={cn('relative w-full min-w-0 pb-40 pt-10', wide ? 'max-w-5xl' : 'max-w-3xl')}>
          {typeof header === 'function' ? header(editor) : header}
          <EditorContent editor={editor} />
          {editable && <BlockHandle editor={editor} host={wrapper} />}
          {editable && <SelectionMenu actions={actions} editor={editor} />}
          {slash && items.length > 0 && (
            <div className="absolute z-30 w-72 overflow-hidden rounded-2xl border border-border bg-overlay p-1.5 shadow-overlay" role="listbox" aria-label={t('插入块')} style={{ left: Math.max(0, Math.min(slash.left, (wrapper.current?.clientWidth ?? 600) - 296)), top: slash.top }}>
              <ScrollShadow className="max-h-80">
                {items.map((command, index) => (
                  <button
                    key={command.id}
                    ref={(node) => { if (node && index === slashIndex) node.scrollIntoView({ block: 'nearest' }); }}
                    aria-selected={index === slashIndex}
                    className={cn('flex w-full items-center gap-3 rounded-xl px-2.5 py-2 text-start transition-colors', index === slashIndex ? 'bg-default' : 'hover:bg-default/60')}
                    role="option"
                    type="button"
                    onMouseDown={(event) => { event.preventDefault(); runSlash(command); }}
                    onMouseEnter={() => { slashState.current.index = index; setSlashIndex(index); }}
                  >
                    <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border bg-surface text-muted"><command.icon className="size-4" /></span>
                    <span className="min-w-0">
                      <span className="block text-sm font-medium text-foreground">{command.label}</span>
                      <span className="block truncate text-xs text-muted">{command.hint}</span>
                    </span>
                  </button>
                ))}
              </ScrollShadow>
            </div>
          )}
        </div>
        <Outline editor={editor} scroller={scroller} wide={wide} />
      </div>
      <input ref={imageInput} accept="image/png,image/jpeg,image/webp,image/gif" className="hidden" type="file" onChange={pick('image')} />
      <input ref={videoInput} accept="video/mp4,video/webm" className="hidden" type="file" onChange={pick('video')} />
      <input ref={attachInput} className="hidden" type="file" onChange={pick('attachment')} />
      {link && (
        <LinkDialog
          currentId={documentId}
          docs={docs}
          editing={link.editing}
          initial={link}
          onClose={() => { setLink(null); editor.commands.focus(); }}
          onRemove={() => { setLink(null); editor.chain().focus().extendMarkRange('link').unsetLink().run(); }}
          onSubmit={applyLink}
        />
      )}
      {math && (
        <MathDialog
          initial={math.latex}
          onClose={() => setMath(null)}
          onSave={(latex) => {
            if (math.pos !== null) editor.chain().focus().setNodeSelection(math.pos).updateAttributes('mathBlock', { latex }).run();
            else insertBlock(editor, { type: 'mathBlock', attrs: { latex } });
            setMath(null);
          }}
        />
      )}
      {drawing && (
        <DrawingDialog
          initial={drawing.initial}
          onClose={() => setDrawing(null)}
          onSave={(attrs) => {
            if (drawing.pos !== null) editor.chain().focus().setNodeSelection(drawing.pos).updateAttributes('drawing', attrs).run();
            else insertBlock(editor, { type: 'drawing', attrs });
            setDrawing(null);
          }}
        />
      )}
    </div>
  );
}
