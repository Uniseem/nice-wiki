<script setup lang="ts">import{ref}from'vue';defineProps<{disabled?:boolean}>();const emit=defineEmits<{send:[value:string],stop:[]}>();const value=ref('');function submit(){const text=value.value.trim();if(!text)return;emit('send',text);value.value=''}</script>
<template>
  <form
    class="flex gap-2"
    @submit.prevent="submit"
  >
    <textarea
      v-model="value"
      rows="2"
      maxlength="500"
      class="nw-input grow"
      placeholder="询问文档内容（最多 500 字）"
    /><button
      v-if="!disabled"
      class="nw-button"
    >
      发送
    </button><button
      v-else
      type="button"
      @click="emit('stop')"
    >
      停止
    </button>
  </form>
</template>
