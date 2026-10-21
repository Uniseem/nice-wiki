import type { NodeViewProps } from '@tiptap/core';
import Highlight from '@tiptap/extension-highlight';
import Link from '@tiptap/extension-link';
import { Table, TableCell, TableHeader, TableRow } from '@tiptap/extension-table';
import TaskItem from '@tiptap/extension-task-item';
import TaskList from '@tiptap/extension-task-list';
import { Placeholder } from '@tiptap/extensions';
import { EditorContent, NodeViewWrapper, ReactNodeViewRenderer, useEditor, type Editor as TiptapEditor } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import { cn, ScrollShadow, toast } from '@heroui/react';
import { captureVideoPoster, errorText, uploadAsset, uploadImage } from '@ui';
import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent } from 'react';
import type { JSONNode } from '../convert';
import { Attachment, Callout, Column, Columns, MathBlock } from './blocks';
import { filterCommands, type BlockCommand, type EditorActions } from './commands';
import { Drawing } from './drawing';
import { DrawingDialog, type DrawingAttrs } from './DrawingDialog';
import { StableIds } from './ids';
import { ImageBlock } from './image';
import { MathDialog, renderMath } from './MathDialog';
import { EditorToolbar, editLink } from './Toolbar';
import { VideoBlock } from './video';

const videoLimit = 500 << 20;
const kindLabel = { image: '图片', attachment: '附件', video: '视频' } as const;

// 编辑器里的公式直接用 KaTeX 渲染成预览，双击打开公式对话框。
function MathView({ node, selected }: NodeViewProps) {
  const { html, error } = renderMath(String(node.attrs.latex ?? ''));
  return (
    <NodeViewWrapper className={cn('math-block', selected && 'ProseMirror-selectednode')} data-math="">
      {error ? <span className="text-danger">公式有误：{error}</span> : <span dangerouslySetInnerHTML={{ __html: html }} />}
    </NodeViewWrapper>
  );
}
const MathBlockWithPreview = MathBlock.extend({ addNodeView() { return ReactNodeViewRenderer(MathView); } });

type Slash = { left: number; top: number; query: string };

export function Editor({ doc, editable, onChange }: { doc: JSONNode; editable: boolean; onChange: (doc: JSONNode) => void }) {
  const wrapper = useRef<HTMLDivElement>(null);
  const onChangeRef = useRef(onChange);
  onChangeRef.current = onChange;
  const editableRef = useRef(editable);
  editableRef.current = editable;
  const [slash, setSlash] = useState<Slash | null>(null);
  const [slashIndex, setSlashIndex] = useState(0);
  const slashState = useRef<{ slash: Slash | null; index: number; items: BlockCommand[] }>({ slash: null, index: 0, items: [] });
  const [math, setMath] = useState<{ pos: number | null; latex: string | null } | null>(null);
  const [drawing, setDrawing] = useState<{ pos: number | null; initial: DrawingAttrs | null } | null>(null);
  const imageInput = useRef<HTMLInputElement>(null);
  const videoInput = useRef<HTMLInputElement>(null);
  const attachInput = useRef<HTMLInputElement>(null);
  const editorRef = useRef<TiptapEditor | null>(null);

  const emitChange = useCallback(() => {
    const current = editorRef.current;
    if (current) onChangeRef.current(current.getJSON() as JSONNode);
  }, []);

  const insertFile = useCallback(async (file: File, kind: 'image' | 'attachment' | 'video') => {
    const current = editorRef.current;
    if (!current) return;
    const label = kindLabel[kind];
    if (kind === 'video' && file.size > videoLimit) { toast.danger('视频超过 500MB', { description: '请压缩后再上传。' }); return; }
    const id = toast(`正在上传${label}`, { description: file.name, isLoading: true, timeout: 0 });
    try {
      const asset = kind === 'image' ? await uploadImage(file) : await uploadAsset(file, kind);
      const poster = kind === 'video' ? await captureVideoPoster(file) : null;
      const content = kind === 'image' ? { type: 'image', attrs: { asset, alt: null } }
        : kind === 'video' ? { type: 'video', attrs: { source: { kind: 'uploaded', asset }, title: file.name.replace(/\.[^.]+$/, '').slice(0, 500), description: '', poster, widthPercent: null } }
        : { type: 'attachment', attrs: { asset, displayName: file.name.slice(0, 255) } };
      current.chain().focus().insertContent(content).run();
      emitChange();
      toast.update(id, `${label}已插入`, { description: file.name, isLoading: false, timeout: 2500, variant: 'success' });
    } catch (problem) {
      toast.update(id, `${label}上传失败`, { description: errorText(problem), isLoading: false, timeout: 8000, variant: 'danger' });
    }
  }, [emitChange]);

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
  }), []);

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
      // 链接需要 LinkTarget 结构，单独配置；标题只允许 H2–H4。
      StarterKit.configure({ heading: { levels: [2, 3, 4] }, link: false }),
      Link.configure({ openOnClick: false, autolink: false, protocols: ['mailto'], HTMLAttributes: { rel: 'noopener noreferrer nofollow' } }),
      Table.configure({ resizable: false }), TableRow, TableHeader, TableCell,
      TaskList, TaskItem.configure({ nested: true }), Highlight,
      ImageBlock, VideoBlock, Callout, MathBlockWithPreview, Attachment, Columns, Column, Drawing, StableIds,
      Placeholder.configure({ placeholder: '输入正文，或输入 / 插入标题、列表、表格、图片…' }),
    ],
    editorProps: {
      attributes: { class: 'nw-prose', 'aria-label': '文档正文' },
      // 粘贴或拖入图片文件：直接上传并插入。
      handlePaste: (_view, event) => handleFiles(event.clipboardData?.files),
      handleDrop: (_view, event) => handleFiles((event as DragEvent).dataTransfer?.files),
      handleKeyDown: (_view, event) => {
        const { slash: open, items, index } = slashState.current;
        if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k' && editorRef.current) { void editLink(editorRef.current); return true; }
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
    // 只有用户在编辑器里操作（获得焦点）才算修改，加载时补稳定 ID 不触发保存。
    onUpdate: ({ editor: current }) => { if (current.isFocused) onChangeRef.current(current.getJSON() as JSONNode); },
    onTransaction: ({ editor: current }) => refreshSlash(current),
  });
  editorRef.current = editor;

  useEffect(() => { editor?.setEditable(editable); }, [editor, editable]);

  const pick = (kind: 'image' | 'attachment' | 'video') => (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (file) void insertFile(file, kind);
  };

  if (!editor) return null;
  const items = slash ? slashState.current.items : [];

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="sticky top-0 z-20 border-b border-separator bg-background/90 px-3 py-1.5 backdrop-blur-md sm:px-6">
        <EditorToolbar actions={actions} editable={editable} editor={editor} />
      </div>
      <div ref={wrapper} className="relative mx-auto w-full max-w-3xl px-5 pb-32 pt-6 sm:px-8">
        <EditorContent editor={editor} />
        {slash && items.length > 0 && (
          <div className="absolute z-30 w-72 overflow-hidden rounded-2xl border border-border bg-overlay p-1.5 shadow-overlay" role="listbox" aria-label="插入块" style={{ left: Math.max(0, Math.min(slash.left, (wrapper.current?.clientWidth ?? 600) - 296)), top: slash.top }}>
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
      <input ref={imageInput} accept="image/png,image/jpeg,image/webp,image/gif" className="hidden" type="file" onChange={pick('image')} />
      <input ref={videoInput} accept="video/mp4,video/webm" className="hidden" type="file" onChange={pick('video')} />
      <input ref={attachInput} className="hidden" type="file" onChange={pick('attachment')} />
      {math && (
        <MathDialog
          initial={math.latex}
          onClose={() => setMath(null)}
          onSave={(latex) => {
            if (math.pos !== null) editor.chain().focus().setNodeSelection(math.pos).updateAttributes('mathBlock', { latex }).run();
            else editor.chain().focus().insertContent({ type: 'mathBlock', attrs: { latex } }).run();
            setMath(null);
            emitChange();
          }}
        />
      )}
      {drawing && (
        <DrawingDialog
          initial={drawing.initial}
          onClose={() => setDrawing(null)}
          onSave={(attrs) => {
            if (drawing.pos !== null) editor.chain().focus().setNodeSelection(drawing.pos).updateAttributes('drawing', attrs).run();
            else editor.chain().focus().insertContent({ type: 'drawing', attrs }).run();
            setDrawing(null);
            emitChange(); // 插入时编辑器可能还没拿到焦点，主动通知保存
          }}
        />
      )}
    </div>
  );
}
