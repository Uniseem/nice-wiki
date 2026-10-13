<script setup lang="ts">
import { onMounted, ref } from 'vue'
import PageHeader from '../../components/PageHeader.vue'
import CostEstimateDialog from '../../components/CostEstimateDialog.vue'
import { client, errorMessage, requireData } from '../../../shared/api/client'
import type { components } from '../../../shared/api/schema.gen'

const value = ref<components['schemas']['LanguagesAdmin']>()
const original = ref<string[]>([])
const error = ref('')
const saved = ref(false)
const confirming = ref(false)
const estimatedCost = ref(0)

onMounted(async () => {
  try {
    value.value = requireData(await client.GET('/api/admin/languages'))
    original.value = [...value.value.enabled]
  } catch (cause) { error.value = errorMessage(cause) }
})

async function persist(confirmCosts: boolean) {
  if (!value.value) return
  requireData(await client.PUT('/api/admin/languages', { body: { enabled: value.value.enabled, confirmCosts } }))
  original.value = [...value.value.enabled]
  confirming.value = false
  saved.value = true
}

async function save() {
  if (!value.value) return
  error.value = ''; saved.value = false
  const previous = new Set(original.value)
  const added = value.value.enabled.filter(code => code !== value.value?.base && !previous.has(code))
  try {
    if (!added.length) { await persist(false); return }
    const estimate = requireData(await client.POST('/api/admin/translations/estimate', { body: { language: added[0]! } }))
    estimatedCost.value = estimate.cost * added.length
    confirming.value = true
  } catch (cause) { error.value = errorMessage(cause) }
}

async function confirm() {
  try { await persist(true) } catch (cause) { error.value = errorMessage(cause); confirming.value = false }
}
</script>

<template>
  <PageHeader
    title="语言"
    description="启用新语言前先估算费用，确认后才会投递已发布文档和站点文字的翻译任务。"
  />
  <p
    v-if="error"
    class="text-danger"
  >
    {{ error }}
  </p>
  <form
    v-if="value"
    class="grid gap-3"
    @submit.prevent="save"
  >
    <label
      v-for="language in value.available"
      :key="language.code"
      class="flex gap-2"
    >
      <input
        v-model="value.enabled"
        type="checkbox"
        :value="language.code"
        :disabled="language.code===value.base"
      >
      {{ language.nativeName }}（{{ language.name }}）
    </label>
    <button class="nw-button mt-4">
      保存语言设置
    </button>
    <p v-if="saved">
      已保存并投递所需任务。
    </p>
  </form>
  <CostEstimateDialog
    :open="confirming"
    :cost="estimatedCost"
    description="将翻译当前全部已发布文档和公开站点文字。旧译文会尽量复用，实际费用以模型渠道账单为准。"
    @confirm="confirm"
    @cancel="confirming=false"
  />
</template>
