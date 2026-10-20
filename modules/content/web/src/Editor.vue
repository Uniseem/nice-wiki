<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue';
import { EditorContent, useEditor } from '@tiptap/vue-3';
import StarterKit from '@tiptap/starter-kit';
import TaskList from '@tiptap/extension-task-list';
import TaskItem from '@tiptap/extension-task-item';
import Highlight from '@tiptap/extension-highlight';
import { StableIds } from './ids';
import Link from '@tiptap/extension-link';
import { Table, TableCell, TableHeader, TableRow } from '@tiptap/extension-table';
import { Attachment, Callout, MathBlock } from './blocks';
import { ImageBlock } from './image';
import { uploadAsset, uploadImage } from './upload';
import { messageOf } from './client';
import type { JSONNode } from './convert';

const props = defineProps<{ doc: JSONNode; editable: boolean }>();
const emit = defineEmits<{ (e: 'change', doc: JSONNode): void }>();

const editor = useEditor({
  content: props.doc,
  editable: props.editable,
  extensions: [
    // 链接需要 LinkTarget 结构，暂不开放；标题只允许 H2–H4。
    StarterKit.configure({ heading: { levels: [2, 3, 4] }, link: false }),
    Link.configure({ openOnClick: false, autolink: false, protocols: ['mailto'], HTMLAttributes: { rel: 'noopener noreferrer nofollow' } }),
    Table.configure({ resizable: false }), TableRow, TableHeader, TableCell,
    TaskList, TaskItem.configure({ nested: true }), Highlight, ImageBlock, Callout, MathBlock, Attachment, StableIds,
  ],
  // 只有用户在编辑器里操作（获得焦点）才算修改，加载时补稳定 ID 不触发保存。
  onUpdate: ({ editor }) => { if (editor.isFocused) emit('change', editor.getJSON() as JSONNode); },
});

watch(() => props.editable, (value) => editor.value?.setEditable(value));
onBeforeUnmount(() => editor.value?.destroy());

const fileInput = ref<HTMLInputElement | null>(null);
const uploading = ref(false);
const uploadError = ref('');

async function insertImage(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = '';
  if (!file || !editor.value) return;
  uploading.value = true;
  uploadError.value = '';
  try {
    const asset = await uploadImage(file);
    editor.value.chain().focus().insertContent({ type: 'image', attrs: { asset, alt: null } }).run();
  } catch (e) { uploadError.value = `图片上传失败：${messageOf(e)}`; } finally { uploading.value = false; }
}

const attachInput = ref<HTMLInputElement | null>(null);

async function insertAttachment(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = '';
  if (!file || !editor.value) return;
  uploading.value = true;
  uploadError.value = '';
  try {
    const asset = await uploadAsset(file, 'attachment');
    editor.value.chain().focus().insertContent({ type: 'attachment', attrs: { asset, displayName: file.name.slice(0, 255) } }).run();
  } catch (e) { uploadError.value = `附件上传失败：${messageOf(e)}`; } finally { uploading.value = false; }
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
</script>

<template>
  <div v-if="editor">
    <div class="toolbar">
      <button v-for="b in buttons" :key="b.label" type="button" :class="{ on: b.on(editor) }" :disabled="!editable" @click="b.run(editor)">{{ b.label }}</button>
      <button type="button" :disabled="!editable || uploading" @click="fileInput?.click()">{{ uploading ? '上传中…' : '图片' }}</button>
      <input ref="fileInput" type="file" accept="image/png,image/jpeg,image/webp,image/gif" hidden @change="insertImage">
      <button type="button" :disabled="!editable || uploading" @click="attachInput?.click()">附件</button>
      <input ref="attachInput" type="file" hidden @change="insertAttachment">
    </div>
    <p v-if="uploadError" class="error">{{ uploadError }}</p>
    <EditorContent :editor="editor" />
  </div>
</template>
