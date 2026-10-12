<script setup lang="ts">defineProps<{reason:string,until:string}>()</script>
<template><aside role="alert" class="rounded-card border border-danger p-4"><strong>问答暂时不可用</strong><p>{{ reason }}</p><p class="text-sm text-muted">预计恢复：{{ until }}</p></aside></template>
