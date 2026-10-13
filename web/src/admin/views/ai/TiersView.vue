<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import PageHeader from '../../components/PageHeader.vue'
import { useAuthStore } from '../../stores/auth'
import { client, errorMessage, requireData } from '../../../shared/api/client'
import type { components } from '../../../shared/api/schema.gen'

type TierName = 'fast' | 'smart' | 'vision' | 'embedding' | 'rerank'

const auth = useAuthStore()
const value = ref<components['schemas']['AISettings']>()
const models = ref<components['schemas']['ModelInfo'][]>([])
const assigned = reactive<Record<TierName, string[]>>({ fast: [], smart: [], vision: [], embedding: [], rerank: [] })
const choices = reactive<Record<TierName, string>>({ fast: '', smart: '', vision: '', embedding: '', rerank: '' })
const error = ref('')
const saved = ref('')
const tiers: Array<{ name: TierName, label: string, capability: string, required: boolean }> = [
  { name: 'fast', label: '快速档', capability: 'chat', required: true },
  { name: 'smart', label: '精确档', capability: 'chat', required: true },
  { name: 'vision', label: '视觉档', capability: 'vision', required: false },
  { name: 'embedding', label: '嵌入档', capability: 'embedding', required: true },
  { name: 'rerank', label: '重排档', capability: 'rerank', required: false },
]

function modelKey(model: components['schemas']['ModelInfo']) {
  return `${model.channel}:${model.id}`
}

function refKey(model: components['schemas']['ModelRef']) {
  return `${model.channel}:${model.id}`
}

function parseRef(key: string): components['schemas']['ModelRef'] {
  const at = key.indexOf(':')
  return { channel: key.slice(0, at), id: key.slice(at + 1) }
}

function compatible(tier: typeof tiers[number]) {
  return models.value.filter(model => model.capabilities.includes(tier.capability) && (tier.name !== 'vision' || model.capabilities.includes('chat')))
}

function available(tier: typeof tiers[number]) {
  const selected = new Set(assigned[tier.name])
  return compatible(tier).filter(model => !selected.has(modelKey(model)))
}

function modelLabel(key: string) {
  const model = models.value.find(value => modelKey(value) === key)
  return model ? `${model.name || model.id} · ${model.channel}` : key
}

function addModel(tier: TierName) {
  const key = choices[tier]
  if (key && !assigned[tier].includes(key)) assigned[tier].push(key)
  choices[tier] = ''
}

function moveModel(tier: TierName, index: number, offset: number) {
  const target = index + offset
  if (target < 0 || target >= assigned[tier].length) return
  const model = assigned[tier][index]
  if (!model) return
  assigned[tier].splice(index, 1)
  assigned[tier].splice(target, 0, model)
}

function removeModel(tier: TierName, index: number) {
  assigned[tier].splice(index, 1)
}

onMounted(async () => {
  try {
    value.value = requireData(await client.GET('/api/admin/settings/ai'))
    if (auth.account?.superadmin) {
      const [catalog, configured] = await Promise.all([
        client.GET('/api/admin/models/catalog'),
        client.GET('/api/admin/models/tiers'),
      ])
      models.value = requireData(catalog)
      const current = requireData(configured)
      for (const tier of tiers) {
        const allowed = new Set(compatible(tier).map(modelKey))
        assigned[tier.name] = current[tier.name].map(refKey).filter(key => allowed.has(key))
      }
    }
  } catch (cause) {
    error.value = errorMessage(cause)
  }
})

async function saveAI() {
  if (!value.value) return
  try {
    error.value = ''
    requireData(await client.PUT('/api/admin/settings/ai', { body: value.value }))
    saved.value = 'AI 设置已保存。'
  } catch (cause) {
    error.value = errorMessage(cause)
  }
}

async function saveTiers() {
  try {
    error.value = ''
    const body = Object.fromEntries(tiers.map(tier => [tier.name, assigned[tier.name].map(parseRef)])) as components['schemas']['ModelTiers']
    requireData(await client.PUT('/api/admin/models/tiers', { body }))
    saved.value = '模型档位已保存。'
  } catch (cause) {
    error.value = errorMessage(cause)
  }
}

async function rebuild() {
  if (!confirm('投递全量重建索引任务？这会产生嵌入费用。')) return
  try {
    error.value = ''
    requireData(await client.POST('/api/admin/jobs/reindex'))
    saved.value = '已投递全量重建索引任务。'
  } catch (cause) {
    error.value = errorMessage(cause)
  }
}
</script>

<template>
  <PageHeader
    title="AI 设置与启用条件"
    description="公开问答需要快速、精确和嵌入档位完整可用。"
  />
  <p
    v-if="error"
    class="text-danger"
  >
    {{ error }}
  </p>
  <p
    v-if="saved"
    class="mb-4"
  >
    {{ saved }}
  </p>

  <form
    v-if="auth.account?.superadmin"
    class="mb-10 grid max-w-2xl gap-4"
    @submit.prevent="saveTiers"
  >
    <h2 class="text-xl font-semibold">
      模型档位
    </h2>
    <p class="text-sm text-muted">
      每档按选择顺序尝试模型；快速、精确和嵌入档为启用问答的必需项。
    </p>
    <label
      v-for="tier in tiers"
      :key="tier.name"
    >
      {{ tier.label }}{{ tier.required ? '（必需）' : '（可选）' }}
      <select
        v-model="choices[tier.name]"
        class="nw-input"
        @change="addModel(tier.name)"
      >
        <option value="">添加模型……</option>
        <option
          v-for="model in available(tier)"
          :key="modelKey(model)"
          :value="modelKey(model)"
        >
          {{ model.name || model.id }} · {{ model.channel }}
        </option>
      </select>
      <ol class="mt-2 grid gap-2">
        <li
          v-for="(key, index) in assigned[tier.name]"
          :key="key"
          class="flex items-center gap-2"
        >
          <span class="grow">{{ index + 1 }}. {{ modelLabel(key) }}</span>
          <button
            type="button"
            :disabled="index === 0"
            @click="moveModel(tier.name, index, -1)"
          >上移</button>
          <button
            type="button"
            :disabled="index === assigned[tier.name].length - 1"
            @click="moveModel(tier.name, index, 1)"
          >下移</button>
          <button
            type="button"
            class="text-danger"
            @click="removeModel(tier.name, index)"
          >移除</button>
        </li>
      </ol>
    </label>
    <button class="nw-button">
      保存模型档位
    </button>
  </form>

  <form
    v-if="value"
    class="grid max-w-xl gap-4"
    @submit.prevent="saveAI"
  >
    <h2 class="text-xl font-semibold">
      问答设置
    </h2>
    <label class="flex gap-2"><input
      v-model="value.enabled"
      type="checkbox"
    >启用公开 AI 问答</label>
    <label class="flex gap-2"><input
      v-model="value.exactCache"
      type="checkbox"
    >精确缓存</label>
    <label class="flex gap-2"><input
      v-model="value.semanticCache"
      type="checkbox"
    >语义缓存</label>
    <label>语义缓存判断方式<select
      v-model="value.semanticCacheMode"
      class="nw-input"
    ><option value="ai">快速模型判断（默认）</option><option value="threshold">严格相似度阈值</option></select></label>
    <label v-if="value.semanticCacheMode==='threshold'">相似度阈值<input
      v-model.number="value.semanticThreshold"
      type="number"
      min="0.5"
      max="0.999"
      step="0.001"
      class="nw-input"
    ></label>
    <label>问题最大长度<input
      v-model.number="value.maxQuestionLength"
      type="number"
      class="nw-input"
    ></label>
    <label>最大对话轮数<input
      v-model.number="value.maxTurns"
      type="number"
      class="nw-input"
    ></label>
    <label>最大输出 token<input
      v-model.number="value.maxOutputTokens"
      type="number"
      class="nw-input"
    ></label>
    <button class="nw-button">
      保存 AI 设置
    </button>
    <button
      type="button"
      class="text-danger"
      @click="rebuild"
    >
      全量重建索引
    </button>
  </form>
</template>
