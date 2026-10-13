<script setup lang="ts">
import { onMounted, ref } from 'vue'
import PageHeader from '../../components/PageHeader.vue'
import { client, errorMessage, requireData } from '../../../shared/api/client'
import type { components } from '../../../shared/api/schema.gen'

const summary = ref<components['schemas']['StatsSummary']>({ views: 0, visitors: 0, aiCrawls: 0, popular: [], languages: [], countries: [], referrers: [] })
const error = ref('')
onMounted(async () => {
  try { summary.value = requireData(await client.GET('/api/admin/stats')) } catch (cause) { error.value = errorMessage(cause) }
})
</script>

<template>
  <PageHeader
    title="访问统计"
    description="不使用追踪 Cookie，独立访客标识每天失效；AI 爬虫读取与浏览器访问分开计算。"
  />
  <p
    v-if="error"
    class="text-danger"
  >
    {{ error }}
  </p>
  <div class="grid gap-4 sm:grid-cols-3">
    <article class="rounded-card border border-border p-5">
      <strong class="text-3xl">{{ summary.views }}</strong><p class="text-muted">
        浏览量
      </p>
    </article>
    <article class="rounded-card border border-border p-5">
      <strong class="text-3xl">{{ summary.visitors }}</strong><p class="text-muted">
        独立访客
      </p>
    </article>
    <article class="rounded-card border border-border p-5">
      <strong class="text-3xl">{{ summary.aiCrawls }}</strong><p class="text-muted">
        AI 爬虫读取
      </p>
    </article>
  </div>
  <h2 class="mt-8 text-xl font-semibold">
    热门文档
  </h2>
  <ol
    v-if="summary.popular.length"
    class="mt-4 grid gap-2"
  >
    <li
      v-for="item in summary.popular"
      :key="item.title"
    >
      {{ item.title }} · {{ item.views }}
    </li>
  </ol><p
    v-else
    class="mt-3 text-muted"
  >
    暂无数据。
  </p>
  <div class="mt-8 grid gap-8 lg:grid-cols-3">
    <section
      v-for="group in [{title:'语言',values:summary.languages},{title:'国家或地区',values:summary.countries},{title:'来源域名',values:summary.referrers}]"
      :key="group.title"
    >
      <h2 class="text-lg font-semibold">
        {{ group.title }}
      </h2>
      <ul
        v-if="group.values.length"
        class="mt-3 grid gap-2"
      >
        <li
          v-for="item in group.values"
          :key="item.value"
        >
          {{ item.value }} · {{ item.views }} 次 / {{ item.visitors }} 人
        </li>
      </ul><p
        v-else
        class="mt-3 text-muted"
      >
        暂无数据。
      </p>
    </section>
  </div>
</template>
