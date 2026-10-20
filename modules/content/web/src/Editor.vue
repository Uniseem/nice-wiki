<script setup lang="ts">
import { onBeforeUnmount, watch } from 'vue';
import { EditorContent, useEditor } from '@tiptap/vue-3';
import StarterKit from '@tiptap/starter-kit';
import TaskList from '@tiptap/extension-task-list';
import TaskItem from '@tiptap/extension-task-item';
import Highlight from '@tiptap/extension-highlight';
import { StableIds } from './ids';
import type { JSONNode } from './convert';

const props = defineProps<{ doc: JSONNode; editable: boolean }>();
const emit = defineEmits<{ (e: 'change', doc: JSONNode): void }>();

const editor = useEditor({
  content: props.doc,
  editable: props.editable,
  extensions: [
    // 链接需要 LinkTarget 结构，暂不开放；标题只允许 H2–H4。
    StarterKit.configure({ heading: { levels: [2, 3, 4] }, link: false }),
    TaskList, TaskItem.configure({ nested: true }), Highlight, StableIds,
  ],
  // 只有用户在编辑器里操作（获得焦点）才算修改，加载时补稳定 ID 不触发保存。
  onUpdate: ({ editor }) => { if (editor.isFocused) emit('change', editor.getJSON() as JSONNode); },
});

watch(() => props.editable, (value) => editor.value?.setEditable(value));
onBeforeUnmount(() => editor.value?.destroy());

const buttons = [
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
    </div>
    <EditorContent :editor="editor" />
  </div>
</template>
