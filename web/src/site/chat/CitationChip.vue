<script setup lang="ts">defineProps<{title:string,url:string,blockId:string}>()</script>
<template><a class="rounded-full border border-border px-2 py-1 text-xs" :href="`${url}#b-${blockId}`">{{ title }}</a></template>
