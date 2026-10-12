<script setup lang="ts">
import { NodeViewWrapper } from '@tiptap/vue-3'
import { ref } from 'vue'
import DrawingDialog from '../drawing/DrawingDialog.vue'
import type { DrawingScene } from '../drawing/mount-excalidraw'
import { useUpload } from '../../composables/useUpload'
const props=defineProps<{node:{attrs:Record<string,unknown>},updateAttributes:(attrs:Record<string,unknown>)=>void}>()
const open=ref(false);const initial=ref<DrawingScene>();const error=ref('');const upload=useUpload()
async function edit(){error.value='';try{const response=await fetch(`/assets/drawings/${props.node.attrs.sceneHash}.excalidraw`,{credentials:'same-origin'});if(!response.ok)throw new Error('无法读取画板场景');initial.value=await response.json() as DrawingScene;open.value=true}catch(cause){error.value=cause instanceof Error?cause.message:'无法读取画板场景'}}
async function save(value:{scene:Blob,light:Blob,dark:Blob}){try{const scene=await upload.upload(new File([value.scene],'drawing.excalidraw',{type:'application/json'}),'drawings');const light=await upload.upload(new File([value.light],'drawing-light.svg',{type:'image/svg+xml'}),'drawings');const dark=await upload.upload(new File([value.dark],'drawing-dark.svg',{type:'image/svg+xml'}),'drawings');props.updateAttributes({sceneHash:scene.hash,lightHash:light.hash,darkHash:dark.hash});open.value=false}catch(cause){error.value=cause instanceof Error?cause.message:'画板保存失败'}}
</script>
<template>
  <NodeViewWrapper
    as="figure"
    class="nw-drawing"
  >
    <img
      :src="`/assets/drawings/${node.attrs.lightHash}.svg`"
      :alt="String(node.attrs.alt ?? '')"
    ><button
      type="button"
      contenteditable="false"
      @click="edit"
    >
      编辑画板
    </button><p
      v-if="error"
      class="text-danger"
    >
      {{ error }}
    </p><input
      :value="String(node.attrs.alt ?? '')"
      class="nw-input"
      placeholder="替代文字"
      @input="updateAttributes({alt:($event.target as HTMLInputElement).value})"
    ><input
      :value="String(node.attrs.caption ?? '')"
      class="nw-input"
      placeholder="图注"
      @input="updateAttributes({caption:($event.target as HTMLInputElement).value})"
    ><DrawingDialog
      :open="open"
      :initial="initial"
      @close="open=false"
      @save="save"
    />
  </NodeViewWrapper>
</template>
