<script setup lang="ts">
import type { ChatMessage } from './useChat'
import FeedbackButtons from './FeedbackButtons.vue'
import CitationChip from './CitationChip.vue'
import { parseAnswer } from './markdown'
defineProps<{message: ChatMessage}>()
</script>

<template>
  <article :class="`nw-chat-message nw-chat-${message.role}`">
    <strong>{{ message.role === 'user' ? '你' : '文档助手' }}</strong>
    <p
      v-if="message.role==='user'"
      class="whitespace-pre-wrap"
    >
      {{ message.text }}
    </p>
    <p
      v-else
      class="nw-chat-answer"
    >
      <template
        v-for="(part,index) in parseAnswer(message.text)"
        :key="index"
      >
        <br v-if="part.type==='break'">
        <strong v-else-if="part.type==='strong'">{{ part.text }}</strong>
        <code v-else-if="part.type==='code'">{{ part.text }}</code>
        <span
          v-else-if="part.type==='citation'"
          class="nw-citation"
        >{{ part.text }}</span>
        <template v-else>
          {{ part.text }}
        </template>
      </template>
    </p>
    <div
      v-if="message.citations?.length"
      class="flex flex-wrap gap-2"
    >
      <CitationChip
        v-for="citation in message.citations"
        :key="citation.blockId"
        :title="citation.title"
        :url="citation.url"
        :block-id="citation.blockId"
      />
    </div>
    <FeedbackButtons
      v-if="message.role==='assistant'&&message.recordId"
      :record-id="message.recordId"
    />
  </article>
</template>
