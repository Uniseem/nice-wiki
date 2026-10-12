<script setup lang="ts">defineProps<{message:string}>()</script>
<template><p role="status" class="text-sm text-muted"><span aria-hidden="true">●</span> {{ message }}</p></template>
