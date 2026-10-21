<template>
  <div :class="['flex', isUser ? 'justify-end' : 'justify-start']">
    <!-- Failed generation -->
    <div
      v-if="message.error"
      class="max-w-[75%] rounded-xl px-4 py-2.5 text-sm bg-red-50 border border-red-200 text-red-700 rounded-bl-sm"
    >
      <div class="flex items-center gap-2 font-medium">
        <i class="pi pi-exclamation-triangle" />
        Generation failed ({{ message.error.code }})
      </div>
      <p class="mt-1 text-xs text-red-600 break-words">{{ message.error.message }}</p>
      <p class="mt-1 text-xs text-red-400">Try sending your message again.</p>
      <div class="text-xs mt-1 text-red-300">{{ formattedTime }}</div>
    </div>

    <!-- Normal message -->
    <div
      v-else
      :class="[
        'max-w-[75%] rounded-xl px-4 py-2.5 text-sm',
        isUser
          ? 'bg-violet-500 text-white rounded-br-sm'
          : 'bg-white border border-zinc-200 text-zinc-800 rounded-bl-sm',
      ]"
    >
      <div v-if="isUser" class="whitespace-pre-wrap">{{ message.content }}</div>
      <div v-else class="prose prose-sm max-w-none" v-html="renderedContent" />
      <div :class="['text-xs mt-1', isUser ? 'text-violet-200' : 'text-zinc-400']">
        {{ formattedTime }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import MarkdownIt from 'markdown-it'
import type { ChatPanelMessage } from './chatPanelTypes'

const md = new MarkdownIt({ linkify: true, breaks: true })

const props = defineProps<{ message: ChatPanelMessage }>()

const isUser = computed(() => props.message.role === 'user')
const renderedContent = computed(() => md.render(props.message.content))
const formattedTime = computed(() =>
  new Date(props.message.created_at).toLocaleTimeString('en-US', {
    hour: '2-digit',
    minute: '2-digit',
  })
)
</script>
