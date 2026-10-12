<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import AuthLayout from '../../layouts/AuthLayout.vue'
import SiteStep from './steps/SiteStep.vue'
import LanguageStep from './steps/LanguageStep.vue'
import ChannelStep from './steps/ChannelStep.vue'
import { useAuthStore } from '../../stores/auth'
import { client, requireData, errorMessage } from '../../../shared/api/client'
const step = ref(0)
const error = ref('')
const auth = useAuthStore()
const router = useRouter()
async function complete() { try { requireData(await client.POST('/api/admin/setup/complete')); await auth.refresh(); await router.replace('/docs') } catch (cause) { error.value = errorMessage(cause) } }
</script>
<template>
  <AuthLayout
    title="设置你的文档站"
    :subtitle="`第 ${step + 1} 步，共 3 步`"
  >
    <SiteStep
      v-if="step === 0"
      @done="step = 1"
    />
    <LanguageStep
      v-else-if="step === 1"
      @done="step = 2"
    />
    <ChannelStep
      v-else
      @done="complete"
    />
    <p
      v-if="error"
      role="alert"
      class="mt-4 text-danger"
    >
      {{ error }}
    </p>
    <button
      v-if="step > 0"
      class="mt-6 text-sm text-muted"
      @click="step--"
    >
      返回上一步
    </button>
  </AuthLayout>
</template>
