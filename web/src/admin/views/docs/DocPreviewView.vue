<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import PageHeader from '../../components/PageHeader.vue'
const route = useRoute()
const source = computed(() => `/api/admin/docs/${encodeURIComponent(String(route.params.id))}/preview`)
</script>

<template>
  <PageHeader
    title="草稿预览"
    description="此页面需要登录，不会被搜索引擎收录。"
  >
    <RouterLink
      to="/docs"
      class="text-sm text-primary"
    >
      返回编辑
    </RouterLink>
  </PageHeader>
  <iframe
    :src="source"
    title="草稿公开页面预览"
    class="h-[75vh] w-full rounded-card border border-border bg-surface"
  />
</template>
