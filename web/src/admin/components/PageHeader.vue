<script setup lang="ts">
defineProps<{ title: string; description?: string }>()
</script>
<template>
  <header class="mb-8 flex flex-wrap items-start justify-between gap-4">
    <div>
      <h1 class="text-2xl font-semibold">
        {{ title }}
      </h1><p
        v-if="description"
        class="mt-2 text-sm text-muted"
      >
        {{ description }}
      </p>
    </div>
    <slot />
  </header>
</template>
