<script setup lang="ts">
defineProps<{ before: string, after: string }>()
</script>
<template>
  <div class="grid gap-4 lg:grid-cols-2">
    <section>
      <h3 class="mb-2 font-semibold">
        旧版本
      </h3><pre class="max-h-[60vh] overflow-auto rounded-card border border-border bg-surface p-4 text-xs"><code>{{ before }}</code></pre>
    </section>
    <section>
      <h3 class="mb-2 font-semibold">
        新版本
      </h3><pre class="max-h-[60vh] overflow-auto rounded-card border border-border bg-surface p-4 text-xs"><code>{{ after }}</code></pre>
    </section>
  </div>
</template>
