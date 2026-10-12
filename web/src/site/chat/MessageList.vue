<script setup lang="ts">import type{ChatMessage}from'./useChat';import MessageItem from'./MessageItem.vue';defineProps<{messages:ChatMessage[]}>()</script>
<template><div class="grid gap-4" aria-live="polite"><MessageItem v-for="(message,index) in messages" :key="index" :message="message" /></div></template>
