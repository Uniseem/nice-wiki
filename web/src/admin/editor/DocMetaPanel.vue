<script setup lang="ts">
const title = defineModel<string>('title', { required: true })
const slug = defineModel<string>('slug', { required: true })
const description = defineModel<string>('description', { required: true })
defineProps<{ readonly: boolean }>()
</script>

<template>
  <section
    class="grid gap-3"
    aria-label="文档属性"
  >
    <div class="grid gap-3 md:grid-cols-2">
      <label class="grid gap-2 text-sm">标题<input
        v-model="title"
        :disabled="readonly"
        maxlength="200"
        class="nw-input"
      ></label>
      <label class="grid gap-2 text-sm">英文路径名<input
        v-model="slug"
        :disabled="readonly"
        maxlength="120"
        pattern="[a-z0-9]+(-[a-z0-9]+)*"
        class="nw-input"
      ></label>
    </div>
    <label class="grid gap-2 text-sm">SEO 描述<textarea
      v-model="description"
      :disabled="readonly"
      maxlength="500"
      rows="2"
      class="nw-input"
    /></label>
    <p class="text-xs text-muted">
      属性随发布保存，草稿自动保存仅保存正文。
    </p>
  </section>
</template>
