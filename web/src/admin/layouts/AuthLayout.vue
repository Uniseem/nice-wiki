<script setup lang="ts">
defineProps<{ title: string; subtitle?: string }>()
</script>

<template>
  <main class="flex min-h-screen items-center justify-center px-6 py-12">
    <section class="w-full max-w-lg rounded-card border border-border bg-surface p-8">
      <p class="mb-6 text-sm font-semibold text-muted">
        nice-wiki
      </p>
      <h1 class="mb-3 text-2xl font-semibold">
        {{ title }}
      </h1>
      <p
        v-if="subtitle"
        class="mb-8 text-sm text-muted"
      >
        {{ subtitle }}
      </p>
      <slot />
    </section>
  </main>
</template>
