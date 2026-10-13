<script setup lang="ts">import ChatPanel from'../chat/ChatPanel.vue';defineProps<{turnstileSiteKey?:string}>()</script>
<template><main class="mx-auto max-w-3xl p-6"><h1 class="mb-6 text-3xl font-semibold">询问文档</h1><ChatPanel :turnstile-site-key="turnstileSiteKey" /></main></template>
