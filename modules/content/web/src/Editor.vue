<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import { EditorContent, useEditor } from '@tiptap/vue-3';
import StarterKit from '@tiptap/starter-kit';
import TaskList from '@tiptap/extension-task-list';
import TaskItem from '@tiptap/extension-task-item';
import Highlight from '@tiptap/extension-highlight';
import { StableIds } from './ids';
import Link from '@tiptap/extension-link';
import { Table, TableCell, TableHeader, TableRow } from '@tiptap/extension-table';
import { Attachment, Callout, Column, Columns, MathBlock } from './blocks';
import { ImageBlock } from './image';
import { VideoBlock } from './video';
import { captureVideoPoster, uploadAsset, uploadImage } from './upload';
import { messageOf } from './client';
import type { JSONNode } from './convert';

const props = defineProps<{ doc: JSONNode; editable: boolean }>();
const emit = defineEmits<{ (e: 'change', doc: JSONNode): void }>();

let ready = false; // 斜杠菜单依赖后面才声明的变量，脚本跑完前不处理编辑器事务
const editor = useEditor({
  content: props.doc,
  editable: props.editable,
  extensions: [
    // 链接需要 LinkTarget 结构，暂不开放；标题只允许 H2–H4。
    StarterKit.configure({ heading: { levels: [2, 3, 4] }, link: false }),
    Link.configure({ openOnClick: false, autolink: false, protocols: ['mailto'], HTMLAttributes: { rel: 'noopener noreferrer nofollow' } }),
    Table.configure({ resizable: false }), TableRow, TableHeader, TableCell,
    TaskList, TaskItem.configure({ nested: true }), Highlight, ImageBlock, VideoBlock, Callout, MathBlock, Attachment, Columns, Column, StableIds,
  ],
  // 只有用户在编辑器里操作（获得焦点）才算修改，加载时补稳定 ID 不触发保存。
  onUpdate: ({ editor }) => { if (editor.isFocused) emit('change', editor.getJSON() as JSONNode); },
  onTransaction: ({ editor }) => refreshSlash(editor),
  editorProps: {
    // 粘贴或拖入图片文件：直接上传并插入。
    handlePaste: (_view, event) => handleFiles(event.clipboardData?.files),
    handleDrop: (_view, event) => handleFiles((event as DragEvent).dataTransfer?.files),
    handleKeyDown: (_view, event) => slashKey(event),
  },
});

function handleFiles(files: FileList | null | undefined): boolean {
  const images = Array.from(files ?? []).filter((f) => f.type.startsWith('image/'));
  if (!images.length) return false;
  void (async () => { for (const file of images) await insertFile(file, 'image'); })();
  return true;
}

// 斜杠菜单：在空段落里输入“/”，按名称过滤，方向键选择、回车执行、Esc 关闭。
const wrapper = ref<HTMLElement | null>(null);
const slash = ref<{ left: number; top: number; query: string } | null>(null);
const slashIndex = ref(0);
const slashCommands = [
  { label: '标题 2', key: 'h2', run: (e: any) => e.chain().focus().setHeading({ level: 2 }).run() },
  { label: '标题 3', key: 'h3', run: (e: any) => e.chain().focus().setHeading({ level: 3 }).run() },
  { label: '标题 4', key: 'h4', run: (e: any) => e.chain().focus().setHeading({ level: 4 }).run() },
  { label: '无序列表', key: 'ul list', run: (e: any) => e.chain().focus().toggleBulletList().run() },
  { label: '有序列表', key: 'ol list', run: (e: any) => e.chain().focus().toggleOrderedList().run() },
  { label: '任务列表', key: 'todo task', run: (e: any) => e.chain().focus().toggleTaskList().run() },
  { label: '引用', key: 'quote', run: (e: any) => e.chain().focus().toggleBlockquote().run() },
  { label: '代码块', key: 'code', run: (e: any) => e.chain().focus().toggleCodeBlock().run() },
  { label: '分割线', key: 'hr divider', run: (e: any) => e.chain().focus().setHorizontalRule().run() },
  { label: '表格', key: 'table', run: (e: any) => e.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run() },
  { label: '提示框', key: 'callout info', run: (e: any) => e.chain().focus().wrapIn('callout', { kind: 'info' }).run() },
  { label: '警告框', key: 'warning', run: (e: any) => e.chain().focus().wrapIn('callout', { kind: 'warning' }).run() },
  { label: '两栏', key: 'columns 分栏', run: (e: any) => insertColumns(e, 2) },
  { label: '三栏', key: 'columns 分栏', run: (e: any) => insertColumns(e, 3) },
  { label: '公式', key: 'math latex', run: (e: any) => { const latex = window.prompt('LaTeX 公式'); if (latex) e.chain().focus().insertContent({ type: 'mathBlock', attrs: { latex } }).run(); } },
  { label: '图片', key: 'image', run: () => fileInput.value?.click() },
  { label: '视频', key: 'video mp4 webm', run: () => videoInput.value?.click() },
  { label: '附件', key: 'file attachment', run: () => attachInput.value?.click() },
];
function insertColumns(e: any, count: number) {
  if (e.isActive('column')) return; // 分栏不能嵌套
  const column = () => ({ type: 'column', content: [{ type: 'paragraph' }] });
  e.chain().focus().insertContent({ type: 'columns', content: Array.from({ length: count }, column) }).run();
}
const slashItems = computed(() => {
  const q = slash.value?.query.trim().toLowerCase() ?? '';
  return slashCommands.filter((c) => !q || c.label.includes(q) || c.key.includes(q));
});

function refreshSlash(e: any) {
  if (!ready) return;
  const { state, view } = e;
  const { $from, empty } = state.selection;
  const text: string = $from.parent.textContent;
  if (!empty || $from.parent.type.name !== 'paragraph' || !text.startsWith('/') || $from.parentOffset !== text.length || !props.editable) {
    slash.value = null;
    return;
  }
  const coords = view.coordsAtPos($from.pos);
  const host = wrapper.value?.getBoundingClientRect();
  slash.value = { left: coords.left - (host?.left ?? 0), top: coords.bottom - (host?.top ?? 0) + 4, query: text.slice(1) };
  if (slashIndex.value >= slashItems.value.length) slashIndex.value = 0;
}

function runSlash(command: (typeof slashCommands)[number]) {
  const e = editor.value;
  if (!e) return;
  const { $from } = e.state.selection;
  e.chain().focus().deleteRange({ from: $from.start(), to: $from.pos }).run();
  slash.value = null;
  command.run(e);
}

function slashKey(event: KeyboardEvent): boolean {
  if (!slash.value || !slashItems.value.length) return false;
  if (event.key === 'ArrowDown') { slashIndex.value = (slashIndex.value + 1) % slashItems.value.length; return true; }
  if (event.key === 'ArrowUp') { slashIndex.value = (slashIndex.value + slashItems.value.length - 1) % slashItems.value.length; return true; }
  if (event.key === 'Enter') { runSlash(slashItems.value[slashIndex.value]); return true; }
  if (event.key === 'Escape') { slash.value = null; return true; }
  return false;
}

watch(() => props.editable, (value) => editor.value?.setEditable(value));
onBeforeUnmount(() => editor.value?.destroy());

const fileInput = ref<HTMLInputElement | null>(null);
const uploading = ref(false);
const uploadError = ref('');

const kindLabel = { image: '图片', attachment: '附件', video: '视频' };

async function insertFile(file: File, kind: 'image' | 'attachment' | 'video') {
  if (!editor.value) return;
  uploading.value = true;
  uploadError.value = '';
  try {
    if (kind === 'video' && file.size > videoLimit) throw new Error('视频超过 500MB，请压缩后再上传');
    const asset = kind === 'image' ? await uploadImage(file) : await uploadAsset(file, kind);
    const poster = kind === 'video' ? await captureVideoPoster(file) : null;
    const content = kind === 'image' ? { type: 'image', attrs: { asset, alt: null } }
      : kind === 'video' ? { type: 'video', attrs: { source: { kind: 'uploaded', asset }, title: file.name.replace(/\.[^.]+$/, '').slice(0, 500), description: '', poster, widthPercent: null } }
      : { type: 'attachment', attrs: { asset, displayName: file.name.slice(0, 255) } };
    editor.value.chain().focus().insertContent(content).run();
  } catch (e) { uploadError.value = `${kindLabel[kind]}上传失败：${messageOf(e)}`; } finally { uploading.value = false; }
}

async function insertImage(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = '';
  if (file) await insertFile(file, 'image');
}

const videoLimit = 500 << 20;
const videoInput = ref<HTMLInputElement | null>(null);
async function insertVideo(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = '';
  if (file) await insertFile(file, 'video');
}

const attachInput = ref<HTMLInputElement | null>(null);

async function insertAttachment(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = '';
  if (file) await insertFile(file, 'attachment');
}

function setLink(e: any) {
  const current = e.getAttributes('link').href ?? '';
  const href = window.prompt('链接地址（http、https 或 mailto）；留空取消链接', current);
  if (href === null) return;
  if (href === '') e.chain().focus().unsetLink().run();
  else if (/^(https?:|mailto:)/i.test(href)) e.chain().focus().setLink({ href }).run();
  else window.alert('只支持 http、https 或 mailto 链接');
}

function setCodeLanguage(e: any) {
  const language = window.prompt('代码语言（如 go、js、python；留空为纯文本）', e.getAttributes('codeBlock').language ?? '');
  if (language !== null) e.chain().focus().updateAttributes('codeBlock', { language: language || null }).run();
}

function insertMath(e: any) {
  const latex = window.prompt('LaTeX 公式');
  if (latex) e.chain().focus().insertContent({ type: 'mathBlock', attrs: { latex } }).run();
}

function toggleCallout(e: any, kind: string) {
  if (e.isActive('callout', { kind })) e.chain().focus().lift('callout').run();
  else e.chain().focus().wrapIn('callout', { kind }).run();
}

const buttons = [
  { label: '链接', run: setLink, on: (e: any) => e.isActive('link') },
  { label: '提示', run: (e: any) => toggleCallout(e, 'info'), on: (e: any) => e.isActive('callout', { kind: 'info' }) },
  { label: '警告', run: (e: any) => toggleCallout(e, 'warning'), on: (e: any) => e.isActive('callout', { kind: 'warning' }) },
  { label: '两栏', run: (e: any) => insertColumns(e, 2), on: (e: any) => e.isActive('columns') },
  { label: '三栏', run: (e: any) => insertColumns(e, 3), on: (e: any) => e.isActive('columns') },
  { label: '代码语言', run: setCodeLanguage, on: () => false },
  { label: '公式', run: insertMath, on: () => false },
  { label: '表格', run: (e: any) => e.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run(), on: (e: any) => e.isActive('table') },
  { label: '+行', run: (e: any) => e.chain().focus().addRowAfter().run(), on: () => false },
  { label: '+列', run: (e: any) => e.chain().focus().addColumnAfter().run(), on: () => false },
  { label: '删行', run: (e: any) => e.chain().focus().deleteRow().run(), on: () => false },
  { label: '删列', run: (e: any) => e.chain().focus().deleteColumn().run(), on: () => false },
  { label: '删表格', run: (e: any) => e.chain().focus().deleteTable().run(), on: () => false },
  { label: '加粗', run: (e: any) => e.chain().focus().toggleBold().run(), on: (e: any) => e.isActive('bold') },
  { label: '斜体', run: (e: any) => e.chain().focus().toggleItalic().run(), on: (e: any) => e.isActive('italic') },
  { label: '下划线', run: (e: any) => e.chain().focus().toggleUnderline().run(), on: (e: any) => e.isActive('underline') },
  { label: '删除线', run: (e: any) => e.chain().focus().toggleStrike().run(), on: (e: any) => e.isActive('strike') },
  { label: '行内代码', run: (e: any) => e.chain().focus().toggleCode().run(), on: (e: any) => e.isActive('code') },
  { label: '高亮', run: (e: any) => e.chain().focus().toggleHighlight().run(), on: (e: any) => e.isActive('highlight') },
  { label: '正文', run: (e: any) => e.chain().focus().setParagraph().run(), on: (e: any) => e.isActive('paragraph') },
  { label: 'H2', run: (e: any) => e.chain().focus().toggleHeading({ level: 2 }).run(), on: (e: any) => e.isActive('heading', { level: 2 }) },
  { label: 'H3', run: (e: any) => e.chain().focus().toggleHeading({ level: 3 }).run(), on: (e: any) => e.isActive('heading', { level: 3 }) },
  { label: 'H4', run: (e: any) => e.chain().focus().toggleHeading({ level: 4 }).run(), on: (e: any) => e.isActive('heading', { level: 4 }) },
  { label: '无序列表', run: (e: any) => e.chain().focus().toggleBulletList().run(), on: (e: any) => e.isActive('bulletList') },
  { label: '有序列表', run: (e: any) => e.chain().focus().toggleOrderedList().run(), on: (e: any) => e.isActive('orderedList') },
  { label: '任务列表', run: (e: any) => e.chain().focus().toggleTaskList().run(), on: (e: any) => e.isActive('taskList') },
  { label: '引用', run: (e: any) => e.chain().focus().toggleBlockquote().run(), on: (e: any) => e.isActive('blockquote') },
  { label: '代码块', run: (e: any) => e.chain().focus().toggleCodeBlock().run(), on: (e: any) => e.isActive('codeBlock') },
  { label: '分割线', run: (e: any) => e.chain().focus().setHorizontalRule().run(), on: () => false },
  { label: '撤销', run: (e: any) => e.chain().focus().undo().run(), on: () => false },
  { label: '重做', run: (e: any) => e.chain().focus().redo().run(), on: () => false },
];
ready = true;
</script>

<template>
  <div v-if="editor" ref="wrapper" style="position:relative">
    <div class="toolbar">
      <button v-for="b in buttons" :key="b.label" type="button" :class="{ on: b.on(editor) }" :disabled="!editable" @click="b.run(editor)">{{ b.label }}</button>
      <button type="button" :disabled="!editable || uploading" @click="fileInput?.click()">{{ uploading ? '上传中…' : '图片' }}</button>
      <input ref="fileInput" type="file" accept="image/png,image/jpeg,image/webp,image/gif" hidden @change="insertImage">
      <button type="button" :disabled="!editable || uploading" @click="videoInput?.click()">视频</button>
      <input ref="videoInput" type="file" accept="video/mp4,video/webm" hidden @change="insertVideo">
      <button type="button" :disabled="!editable || uploading" @click="attachInput?.click()">附件</button>
      <input ref="attachInput" type="file" hidden @change="insertAttachment">
    </div>
    <p v-if="uploadError" class="error">{{ uploadError }}</p>
    <EditorContent :editor="editor" />
    <ul v-if="slash && slashItems.length" class="slash-menu" :style="{ left: slash.left + 'px', top: slash.top + 'px' }">
      <li v-for="(c, i) in slashItems" :key="c.label" :class="{ on: i === slashIndex }" @mousedown.prevent="runSlash(c)">{{ c.label }}</li>
    </ul>
  </div>
</template>
