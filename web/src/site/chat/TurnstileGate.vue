<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { client, errorMessage, requireData } from '../../shared/api/client'

interface TurnstileAPI { render(element: HTMLElement, options: { sitekey: string, callback: (token: string) => void, 'error-callback': () => void, 'expired-callback': () => void }): string, remove(id: string): void }
declare global { interface Window { turnstile?: TurnstileAPI } }

const props = defineProps<{siteKey?: string}>()
const emit = defineEmits<{ready: []}>()
const container = ref<HTMLElement>()
const error = ref('')
const busy = ref(false)
const usingPow = ref(!props.siteKey)
let worker: Worker | undefined
let widget = ''
let fallbackTimer: number | undefined

async function createSession(token: string) {
  requireData(await client.POST('/api/public/session', { body: { turnstileToken: token, powChallenge: '', powNonce: '' } }))
  emit('ready')
}
async function prove() {
  if (busy.value) return
  usingPow.value = true; busy.value = true; error.value = ''
  try {
    const value = requireData(await client.GET('/api/public/pow'))
    worker = new Worker(new URL('./pow-worker.ts', import.meta.url), { type: 'module' })
    const nonce = await new Promise<string>((resolve, reject) => { worker!.onmessage = event => resolve(event.data); worker!.onerror = event => reject(new Error(event.message)); worker!.postMessage(value) })
    requireData(await client.POST('/api/public/session', { body: { turnstileToken: '', powChallenge: value.challenge, powNonce: nonce } }))
    emit('ready')
  } catch (cause) { error.value = errorMessage(cause) }
  finally { busy.value = false; worker?.terminate(); worker = undefined }
}
function renderTurnstile() {
  if (!props.siteKey || !container.value || !window.turnstile) { void prove(); return }
  window.clearTimeout(fallbackTimer)
  widget = window.turnstile.render(container.value, {
    sitekey: props.siteKey,
    callback: token => { void createSession(token).catch(cause => { error.value = errorMessage(cause); void prove() }) },
    'error-callback': () => { void prove() },
    'expired-callback': () => { error.value = '验证已过期，请重试' },
  })
}
onMounted(() => {
  if (!props.siteKey) { void prove(); return }
  fallbackTimer = window.setTimeout(() => { void prove() }, 5000)
  const existing = document.querySelector<HTMLScriptElement>('script[data-nw-turnstile]')
  if (existing) { if (window.turnstile) renderTurnstile(); else existing.addEventListener('load', renderTurnstile, { once: true }); return }
  const script = document.createElement('script')
  script.src = 'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'
  script.async = true; script.defer = true; script.dataset.nwTurnstile = '1'
  script.addEventListener('load', renderTurnstile, { once: true })
  script.addEventListener('error', () => { void prove() }, { once: true })
  document.head.append(script)
})
onBeforeUnmount(() => { window.clearTimeout(fallbackTimer); worker?.terminate(); if (widget && window.turnstile) window.turnstile.remove(widget) })
</script>
<template>
  <section class="rounded-card border border-border p-4">
    <p>
      {{ usingPow ? '正在使用本地工作量证明保护公开问答。' : '正在完成人机验证…' }}
    </p>
    <div
      v-show="!usingPow"
      ref="container"
      class="mt-3"
    />
    <button
      v-if="usingPow&&error"
      class="nw-button mt-3"
      :disabled="busy"
      @click="prove"
    >
      {{ busy?'正在验证…':'重新验证' }}
    </button>
    <p
      v-if="error"
      class="text-danger"
    >
      {{ error }}
    </p>
  </section>
</template>
