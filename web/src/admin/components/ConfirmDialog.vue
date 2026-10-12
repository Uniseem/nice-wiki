<script setup lang="ts">
defineProps<{ open: boolean, title: string, description: string, dangerous?: boolean }>()
const emit = defineEmits<{ confirm: [], cancel: [] }>()
</script>
<template>
  <div
    v-if="open"
    class="fixed inset-0 z-50 grid place-items-center bg-black/50 p-4"
    role="presentation"
    @click.self="emit('cancel')"
  >
    <section
      role="alertdialog"
      aria-modal="true"
      :aria-label="title"
      class="w-full max-w-md rounded-card bg-surface p-6"
    >
      <h2 class="text-lg font-semibold">
        {{ title }}
      </h2><p class="mt-3 text-sm text-muted">
        {{ description }}
      </p><div class="mt-6 flex justify-end gap-3">
        <button
          type="button"
          @click="emit('cancel')"
        >
          取消
        </button><button
          type="button"
          :class="dangerous ? 'text-danger' : 'text-primary'"
          @click="emit('confirm')"
        >
          确认
        </button>
      </div>
    </section>
  </div>
</template>
