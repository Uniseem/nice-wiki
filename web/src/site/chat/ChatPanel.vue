<script setup lang="ts">
import { ref } from 'vue'
import { useChat } from './useChat'
import ChatInput from './ChatInput.vue'
import MessageList from './MessageList.vue'
import TurnstileGate from './TurnstileGate.vue'
import ProgressSteps from './ProgressSteps.vue'
import BanNotice from './BanNotice.vue'

defineProps<{page?: string, turnstileSiteKey?: string}>()
const chat = useChat()
const verified = ref(sessionStorage.getItem('nw-qa-verified') === '1')
function ready() { verified.value = true; chat.verified(); sessionStorage.setItem('nw-qa-verified', '1') }
</script>

<template>
  <TurnstileGate
    v-if="!verified||chat.verificationRequired.value"
    :site-key="turnstileSiteKey"
    @ready="ready"
  />
  <section
    v-else
    class="grid gap-4"
  >
    <MessageList :messages="chat.messages.value" />
    <ProgressSteps
      v-if="chat.progress.value"
      :message="chat.progress.value"
    />
    <BanNotice
      v-if="chat.blocked.value"
      :reason="chat.blocked.value.reason"
      :until="chat.blocked.value.until"
    />
    <p
      v-else-if="chat.error.value"
      class="text-danger"
    >
      {{ chat.error.value }}
    </p>
    <ChatInput
      v-if="!chat.blocked.value"
      :disabled="chat.sending.value"
      @send="value=>chat.send(value,page)"
      @stop="chat.stop"
    />
    <button
      v-if="chat.messages.value.length"
      type="button"
      class="text-sm text-muted"
      @click="chat.clear"
    >
      清空对话
    </button>
  </section>
</template>
