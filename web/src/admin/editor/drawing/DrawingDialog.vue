<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { mountExcalidraw, type DrawingScene, type MountedDrawing } from './mount-excalidraw'
const props = defineProps<{ open: boolean, initial?: DrawingScene }>()
const emit = defineEmits<{ close: [], save: [value: { scene: Blob, light: Blob, dark: Blob }] }>()
const host = ref<HTMLElement>(); const error = ref(''); const saving = ref(false); let mounted: MountedDrawing | undefined
watch(() => props.open, async open => { mounted?.destroy(); mounted = undefined; if (!open) return; await nextTick(); if (!host.value) return; try { mounted = await mountExcalidraw(host.value, props.initial) } catch (cause) { error.value = cause instanceof Error ? cause.message : '画板加载失败' } })
onBeforeUnmount(() => mounted?.destroy())
async function save() { if (!mounted) return; saving.value = true; try { const scene = new Blob([JSON.stringify(mounted.scene())], { type: 'application/json' }); const [light, dark] = await Promise.all([mounted.exportSVG(false), mounted.exportSVG(true)]); emit('save', { scene, light, dark }) } catch (cause) { error.value = cause instanceof Error ? cause.message : '画板导出失败' } finally { saving.value = false } }
</script>
<template>
  <div
    v-if="open"
    class="fixed inset-0 z-50 grid grid-rows-[auto_1fr] bg-surface"
  >
    <header class="flex items-center justify-between border-b border-border p-3">
      <p class="font-semibold">
        画板
      </p><div class="flex gap-4">
        <button
          :disabled="saving"
          class="text-primary"
          @click="save"
        >
          保存画板
        </button><button @click="emit('close')">
          关闭
        </button>
      </div>
    </header><p
      v-if="error"
      class="p-3 text-danger"
    >
      {{ error }}
    </p><div
      ref="host"
      class="min-h-0"
    />
  </div>
</template>
