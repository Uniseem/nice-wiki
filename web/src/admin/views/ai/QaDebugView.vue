<script setup lang="ts">
import { ref } from 'vue'
import PageHeader from '../../components/PageHeader.vue'
import { client, errorMessage, requireData } from '../../../shared/api/client'
import type { components } from '../../../shared/api/schema.gen'

const id = ref('')
const question = ref('')
const language = ref('zh-Hans')
const value = ref<components['schemas']['DebugTrace']>()
const error = ref('')

async function run() {
  try {
    error.value = ''
    value.value = requireData(await client.POST('/api/admin/qa/debug', { body: { question: question.value, language: language.value } }))
    id.value = value.value.id
  } catch (cause) { error.value = errorMessage(cause) }
}
async function load() {
  try { error.value = ''; value.value = requireData(await client.GET('/api/admin/qa/debug/{id}', { params: { path: { id: id.value } } })) } catch (cause) { error.value = errorMessage(cause) }
}
</script>

<template>
  <PageHeader
    title="问答调试台"
    description="只有点击运行才会调用模型并产生费用；调试记录保留一小时，不写入访客问答记录或回答缓存。"
  />
  <form
    class="grid max-w-2xl gap-3"
    @submit.prevent="run"
  >
    <textarea
      v-model="question"
      required
      maxlength="10000"
      rows="4"
      class="nw-input"
      placeholder="输入调试问题"
    />
    <input
      v-model="language"
      required
      class="nw-input"
      placeholder="语言代码"
    >
    <button class="nw-button">
      运行调试
    </button>
  </form>
  <form
    class="mt-6 flex gap-2"
    @submit.prevent="load"
  >
    <input
      v-model="id"
      class="nw-input"
      placeholder="已有调试记录 ID"
    ><button>读取</button>
  </form>
  <p
    v-if="error"
    class="text-danger"
  >
    {{ error }}
  </p>
  <article
    v-for="step in value?.steps"
    :key="`${step.name}:${step.duration}`"
    class="mt-4 rounded-card border border-border p-4"
  >
    <strong>{{ step.name }}</strong><pre class="overflow-auto whitespace-pre-wrap">{{ step.output }}</pre><small>{{ step.duration }} ns · ${{ step.cost }}</small>
  </article>
</template>
