import { Brush, CircleCheck, CircleExclamation, CircleInfo, Code, Function as FunctionIcon, Heading2, Heading3, Heading4, LayoutColumns, LayoutColumns3, LayoutHeaderCells, ListCheck, ListOl, ListUl, Minus, Paperclip, Picture, QuoteOpen, TextAlignLeft, TriangleExclamation, Video } from '@gravity-ui/icons';
import type { Editor } from '@tiptap/react';
import type { ComponentType } from 'react';

// 编辑器里所有“插入/转换块”的命令：斜杠菜单、工具栏插入菜单共用一份。

export type EditorActions = {
  pickImage: () => void;
  pickVideo: () => void;
  pickAttachment: () => void;
  openDrawing: () => void;
  openMath: () => void;
};

export type BlockCommand = {
  id: string;
  label: string;
  hint: string;
  keywords: string;
  group: '文本' | '列表' | '结构' | '媒体';
  icon: ComponentType<{ className?: string }>;
  run: (editor: Editor, actions: EditorActions) => void;
};

export function insertColumns(editor: Editor, count: number): void {
  if (editor.isActive('column')) return; // 分栏不能嵌套
  const column = () => ({ type: 'column', content: [{ type: 'paragraph' }] });
  editor.chain().focus().insertContent({ type: 'columns', content: Array.from({ length: count }, column) }).run();
}

export function toggleCallout(editor: Editor, kind: string): void {
  if (editor.isActive('callout', { kind })) editor.chain().focus().lift('callout').run();
  else if (editor.isActive('callout')) editor.chain().focus().updateAttributes('callout', { kind }).run();
  else editor.chain().focus().wrapIn('callout', { kind }).run();
}

export const calloutKinds = [
  { kind: 'info', label: '信息', icon: CircleInfo },
  { kind: 'tip', label: '技巧', icon: CircleCheck },
  { kind: 'warning', label: '警告', icon: TriangleExclamation },
  { kind: 'danger', label: '危险', icon: CircleExclamation },
] as const;

export const blockCommands: BlockCommand[] = [
  { id: 'paragraph', label: '正文', hint: '普通段落', keywords: 'text paragraph zhengwen', group: '文本', icon: TextAlignLeft, run: (e) => { e.chain().focus().setParagraph().run(); } },
  { id: 'h2', label: '标题 2', hint: '章节标题', keywords: 'h2 heading biaoti', group: '文本', icon: Heading2, run: (e) => { e.chain().focus().setHeading({ level: 2 }).run(); } },
  { id: 'h3', label: '标题 3', hint: '小节标题', keywords: 'h3 heading biaoti', group: '文本', icon: Heading3, run: (e) => { e.chain().focus().setHeading({ level: 3 }).run(); } },
  { id: 'h4', label: '标题 4', hint: '更小一级', keywords: 'h4 heading biaoti', group: '文本', icon: Heading4, run: (e) => { e.chain().focus().setHeading({ level: 4 }).run(); } },
  { id: 'quote', label: '引用', hint: '引用一段话', keywords: 'quote blockquote yinyong', group: '文本', icon: QuoteOpen, run: (e) => { e.chain().focus().toggleBlockquote().run(); } },
  { id: 'code', label: '代码块', hint: '带语法高亮', keywords: 'code pre daima', group: '文本', icon: Code, run: (e) => { e.chain().focus().toggleCodeBlock().run(); } },
  { id: 'bullet', label: '无序列表', hint: '圆点列表', keywords: 'ul list bullet liebiao', group: '列表', icon: ListUl, run: (e) => { e.chain().focus().toggleBulletList().run(); } },
  { id: 'ordered', label: '有序列表', hint: '编号列表', keywords: 'ol list ordered liebiao', group: '列表', icon: ListOl, run: (e) => { e.chain().focus().toggleOrderedList().run(); } },
  { id: 'task', label: '任务列表', hint: '可勾选的待办', keywords: 'todo task checkbox renwu', group: '列表', icon: ListCheck, run: (e) => { e.chain().focus().toggleTaskList().run(); } },
  ...calloutKinds.map((item): BlockCommand => ({
    id: `callout-${item.kind}`, label: `${item.label}提示框`, hint: '醒目的提示块', keywords: `callout ${item.kind} tishi`, group: '结构', icon: item.icon,
    run: (e) => toggleCallout(e, item.kind),
  })),
  { id: 'table', label: '表格', hint: '3 × 3，带表头', keywords: 'table biaoge', group: '结构', icon: LayoutHeaderCells, run: (e) => { e.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run(); } },
  { id: 'columns2', label: '两栏', hint: '并排两栏', keywords: 'columns fenlan', group: '结构', icon: LayoutColumns, run: (e) => insertColumns(e, 2) },
  { id: 'columns3', label: '三栏', hint: '并排三栏', keywords: 'columns fenlan', group: '结构', icon: LayoutColumns3, run: (e) => insertColumns(e, 3) },
  { id: 'math', label: '公式', hint: 'LaTeX 数学公式', keywords: 'math latex formula gongshi', group: '结构', icon: FunctionIcon, run: (_, a) => a.openMath() },
  { id: 'divider', label: '分割线', hint: '水平分隔', keywords: 'hr divider line fenge', group: '结构', icon: Minus, run: (e) => { e.chain().focus().setHorizontalRule().run(); } },
  { id: 'image', label: '图片', hint: '上传 PNG、JPG、WebP、GIF', keywords: 'image picture img tupian', group: '媒体', icon: Picture, run: (_, a) => a.pickImage() },
  { id: 'video', label: '视频', hint: '上传 MP4 或 WebM', keywords: 'video mp4 webm shipin', group: '媒体', icon: Video, run: (_, a) => a.pickVideo() },
  { id: 'attachment', label: '附件', hint: '任意文件，供下载', keywords: 'file attachment fujian', group: '媒体', icon: Paperclip, run: (_, a) => a.pickAttachment() },
  { id: 'drawing', label: '画板', hint: '手绘图示（Excalidraw）', keywords: 'drawing board excalidraw huaban', group: '媒体', icon: Brush, run: (_, a) => a.openDrawing() },
];

export function filterCommands(query: string): BlockCommand[] {
  const q = query.trim().toLowerCase();
  if (!q) return blockCommands;
  return blockCommands.filter((command) => command.label.toLowerCase().includes(q) || command.keywords.includes(q));
}

export const codeLanguages = ['plaintext', 'bash', 'go', 'javascript', 'typescript', 'python', 'json', 'yaml', 'html', 'css', 'sql', 'rust', 'java', 'c', 'cpp', 'markdown', 'dockerfile'];
