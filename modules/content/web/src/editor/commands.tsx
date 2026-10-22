import { Brush, CircleCheck, CircleExclamation, CircleInfo, Code, FileText, Function as FunctionIcon, Heading2, Heading3, Heading4, LayoutColumns, LayoutColumns3, LayoutHeaderCells, ListCheck, ListOl, ListUl, Minus, Paperclip, Picture, QuoteOpen, TextAlignLeft, TriangleExclamation, Video } from '@gravity-ui/icons';
import { NodeSelection, TextSelection, type Transaction } from '@tiptap/pm/state';
import type { Editor, JSONContent } from '@tiptap/react';
import type { ComponentType } from 'react';
import { t } from '@ui';

// 编辑器里所有“插入/转换块”的命令：斜杠菜单、工具栏插入菜单共用一份。

export type EditorActions = {
  pickImage: () => void;
  pickVideo: () => void;
  pickAttachment: () => void;
  openDrawing: () => void;
  openMath: () => void;
  editLink: () => void;
};

export type BlockCommand = {
  id: string;
  label: string;
  hint: string;
  keywords: string;
  group: string;
  icon: ComponentType<{ className?: string }>;
  run: (editor: Editor, actions: EditorActions) => void;
};

// 光标移到某个块后面：后面是段落就放到段落开头，否则补一个空段落。
function caretAfter(editor: Editor, tr: Transaction, after: number): void {
  if (!tr.doc.resolve(after).nodeAfter?.isTextblock) tr.insert(after, editor.schema.nodes.paragraph.create());
  tr.setSelection(TextSelection.create(tr.doc, after + 1));
}

// 当前选中的是整块（比如刚点过的图片）时先把光标移到它后面，接下来插入的内容不会替换它。
export function leaveBlockSelection(editor: Editor): void {
  const { selection } = editor.state;
  if (selection instanceof NodeSelection) editor.chain().command(({ tr }) => { caretAfter(editor, tr, selection.to); return true; }).run();
}

// 插入原子块（图片、视频、附件、公式、画板）：插完把光标移到它后面，否则选区停在新块上，
// 接着再插入（比如一次拖入多张图）会把它替换掉。
export function insertBlock(editor: Editor, content: JSONContent): void {
  leaveBlockSelection(editor);
  editor.chain().focus().insertContent(content).command(({ tr }) => {
    if (tr.selection instanceof NodeSelection) caretAfter(editor, tr, tr.selection.to);
    return true;
  }).run();
}

export function insertColumns(editor: Editor, count: number): void {
  if (editor.isActive('column')) return; // 分栏不能嵌套
  const column = () => ({ type: 'column', content: [{ type: 'paragraph' }] });
  // 插入后光标默认落在最后一栏，挪到第一栏开头，接着输入的内容从左边开始。
  editor.chain().focus().insertContent({ type: 'columns', content: Array.from({ length: count }, column) }).command(({ tr }) => {
    const { $from } = tr.selection;
    for (let depth = $from.depth; depth > 0; depth--) {
      if ($from.node(depth).type.name === 'columns') {
        tr.setSelection(TextSelection.create(tr.doc, $from.before(depth) + 3));
        break;
      }
    }
    return true;
  }).run();
}

export function toggleCallout(editor: Editor, kind: string): void {
  if (editor.isActive('callout', { kind })) editor.chain().focus().lift('callout').run();
  else if (editor.isActive('callout')) editor.chain().focus().updateAttributes('callout', { kind }).run();
  else editor.chain().focus().wrapIn('callout', { kind }).run();
}

export const calloutKinds = [
  { kind: 'info', label: t('信息'), icon: CircleInfo },
  { kind: 'tip', label: t('技巧'), icon: CircleCheck },
  { kind: 'warning', label: t('警告'), icon: TriangleExclamation },
  { kind: 'danger', label: t('危险'), icon: CircleExclamation },
] as const;

export const blockCommands: BlockCommand[] = [
  { id: 'paragraph', label: t('正文'), hint: t('普通段落'), keywords: 'text paragraph zhengwen', group: t('文本'), icon: TextAlignLeft, run: (e) => { e.chain().focus().setParagraph().run(); } },
  { id: 'h2', label: t('标题 2'), hint: t('章节标题'), keywords: 'h2 heading biaoti', group: t('文本'), icon: Heading2, run: (e) => { e.chain().focus().setHeading({ level: 2 }).run(); } },
  { id: 'h3', label: t('标题 3'), hint: t('小节标题'), keywords: 'h3 heading biaoti', group: t('文本'), icon: Heading3, run: (e) => { e.chain().focus().setHeading({ level: 3 }).run(); } },
  { id: 'h4', label: t('标题 4'), hint: t('更小一级'), keywords: 'h4 heading biaoti', group: t('文本'), icon: Heading4, run: (e) => { e.chain().focus().setHeading({ level: 4 }).run(); } },
  { id: 'link', label: t('链接到文档'), hint: t('指向站内另一篇文档'), keywords: 'link wiki page lianjie wendang', group: t('文本'), icon: FileText, run: (_, a) => a.editLink() },
  { id: 'quote', label: t('引用'), hint: t('引用一段话'), keywords: 'quote blockquote yinyong', group: t('文本'), icon: QuoteOpen, run: (e) => { e.chain().focus().toggleBlockquote().run(); } },
  { id: 'code', label: t('代码块'), hint: t('带语法高亮'), keywords: 'code pre daima', group: t('文本'), icon: Code, run: (e) => { e.chain().focus().toggleCodeBlock().run(); } },
  { id: 'bullet', label: t('无序列表'), hint: t('圆点列表'), keywords: 'ul list bullet liebiao', group: t('列表'), icon: ListUl, run: (e) => { e.chain().focus().toggleBulletList().run(); } },
  { id: 'ordered', label: t('有序列表'), hint: t('编号列表'), keywords: 'ol list ordered liebiao', group: t('列表'), icon: ListOl, run: (e) => { e.chain().focus().toggleOrderedList().run(); } },
  { id: 'task', label: t('任务列表'), hint: t('可勾选的待办'), keywords: 'todo task checkbox renwu', group: t('列表'), icon: ListCheck, run: (e) => { e.chain().focus().toggleTaskList().run(); } },
  ...calloutKinds.map((item): BlockCommand => ({
    id: `callout-${item.kind}`, label: t('{0}提示框', item.label), hint: t('醒目的提示块'), keywords: `callout ${item.kind} tishi`, group: t('结构'), icon: item.icon,
    run: (e) => toggleCallout(e, item.kind),
  })),
  { id: 'table', label: t('表格'), hint: t('3 × 3，带表头'), keywords: 'table biaoge', group: t('结构'), icon: LayoutHeaderCells, run: (e) => { e.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run(); } },
  { id: 'columns2', label: t('两栏'), hint: t('并排两栏'), keywords: 'columns fenlan', group: t('结构'), icon: LayoutColumns, run: (e) => insertColumns(e, 2) },
  { id: 'columns3', label: t('三栏'), hint: t('并排三栏'), keywords: 'columns fenlan', group: t('结构'), icon: LayoutColumns3, run: (e) => insertColumns(e, 3) },
  { id: 'math', label: t('公式'), hint: t('LaTeX 数学公式'), keywords: 'math latex formula gongshi', group: t('结构'), icon: FunctionIcon, run: (_, a) => a.openMath() },
  { id: 'divider', label: t('分割线'), hint: t('水平分隔'), keywords: 'hr divider line fenge', group: t('结构'), icon: Minus, run: (e) => { e.chain().focus().setHorizontalRule().run(); } },
  { id: 'image', label: t('图片'), hint: t('上传 PNG、JPG、WebP、GIF'), keywords: 'image picture img tupian', group: t('媒体'), icon: Picture, run: (_, a) => a.pickImage() },
  { id: 'video', label: t('视频'), hint: t('上传 MP4 或 WebM'), keywords: 'video mp4 webm shipin', group: t('媒体'), icon: Video, run: (_, a) => a.pickVideo() },
  { id: 'attachment', label: t('附件'), hint: t('任意文件，供下载'), keywords: 'file attachment fujian', group: t('媒体'), icon: Paperclip, run: (_, a) => a.pickAttachment() },
  { id: 'drawing', label: t('画板'), hint: t('手绘图示（Excalidraw）'), keywords: 'drawing board excalidraw huaban', group: t('媒体'), icon: Brush, run: (_, a) => a.openDrawing() },
];

export function filterCommands(query: string): BlockCommand[] {
  const q = query.trim().toLowerCase();
  if (!q) return blockCommands;
  return blockCommands.filter((command) => command.label.toLowerCase().includes(q) || command.keywords.includes(q));
}

export const codeLanguages = ['plaintext', 'bash', 'go', 'javascript', 'typescript', 'python', 'json', 'yaml', 'html', 'css', 'sql', 'rust', 'java', 'c', 'cpp', 'markdown', 'dockerfile'];
