<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';
import type { Board } from './drawing-board';
import type { AssetRef } from './upload';
import { uploadAsset } from './upload';
import { messageOf } from './client';

export type DrawingAttrs = { scene: AssetRef; lightSVG: AssetRef; darkSVG: AssetRef; previewURL?: string };
const props = defineProps<{ initial: DrawingAttrs | null }>();
const emit = defineEmits<{ (e: 'save', attrs: DrawingAttrs): void; (e: 'close'): void }>();

const host = ref<HTMLElement | null>(null);
const busy = ref(false), error = ref('');
let board: Board | null = null;

onMounted(async () => {
  try {
    let scene: any = null;
    if (props.initial) scene = await (await fetch(`/v1/assets/${props.initial.scene.assetId}/bytes?sha=${props.initial.scene.sha256}`)).json();
    // Excalidraw 在加载时读取字体路径，必须先设；字体随 content 界面一起发布，不走外网
    (window as any).EXCALIDRAW_ASSET_PATH = '/_admin/modules/content/excalidraw/';
    const { mountBoard } = await import('./drawing-board');
    if (host.value) board = mountBoard(host.value, scene);
  } catch (problem) { error.value = `画板加载失败：${messageOf(problem)}`; }
});
onBeforeUnmount(() => board?.destroy());

async function save() {
  if (!board || busy.value) return;
  busy.value = true; error.value = '';
  try {
    const result = await board.snapshot();
    if (!result) { error.value = '画板是空的，先画点什么。'; return; }
    const scene = await uploadAsset(new File([result.scene], 'scene.json', { type: 'application/json' }), 'drawing_scene');
    const lightSVG = await uploadAsset(new File([result.lightSVG], 'light.svg', { type: 'image/svg+xml' }), 'drawing_svg');
    const darkSVG = await uploadAsset(new File([result.darkSVG], 'dark.svg', { type: 'image/svg+xml' }), 'drawing_svg');
    emit('save', { scene, lightSVG, darkSVG, previewURL: 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(result.lightSVG) });
  } catch (problem) { error.value = `保存失败：${messageOf(problem)}`; }
  finally { busy.value = false; }
}
</script>

<template>
  <div class="drawing-dialog" role="dialog" aria-modal="true" aria-label="画板">
    <div class="drawing-bar">
      <strong>画板</strong>
      <span v-if="error" class="error">{{ error }}</span>
      <span style="flex:1"></span>
      <button type="button" :disabled="busy" @click="save">{{ busy ? '保存中…' : '完成并插入' }}</button>
      <button type="button" :disabled="busy" @click="emit('close')">取消</button>
    </div>
    <div ref="host" class="drawing-host"></div>
  </div>
</template>
