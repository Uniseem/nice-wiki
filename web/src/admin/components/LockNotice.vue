<script setup lang="ts">defineProps<{ message: string, readonly?: boolean }>()</script>
<template>
  <aside
    v-if="message"
    role="status"
    class="rounded-card border border-border bg-surface p-3 text-sm"
  >
    <strong>{{ readonly ? '只读模式' : '编辑锁' }}</strong><p class="mt-1 text-muted">
      {{ message }}
    </p>
  </aside>
</template>
