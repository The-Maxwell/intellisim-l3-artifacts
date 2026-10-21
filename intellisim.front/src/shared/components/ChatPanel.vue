<template>
  <div class="flex flex-col h-full">
    <!-- Messages -->
    <div ref="messagesContainer" class="flex-1 overflow-y-auto px-4 py-4 space-y-3" @scroll="handleScroll">
      <!-- Older-pages loader -->
      <div v-if="loadingOlder" class="flex justify-center py-1">
        <ProgressSpinner style="width: 18px; height: 18px" strokeWidth="5" />
      </div>

      <!-- Empty state -->
      <slot v-if="messages.length === 0 && !loading && !thinking" name="empty">
        <div class="flex flex-col items-center justify-center h-full text-zinc-400">
          <i class="pi pi-comments text-5xl mb-3" />
          <p class="text-sm">Start a conversation</p>
        </div>
      </slot>

      <!-- Messages: stable id keys, chronological order is the caller's contract -->
      <MessageBubble v-for="msg in messages" :key="msg.id" :message="msg" />

      <!-- LLM thinking indicator -->
      <div v-if="thinking" class="flex justify-start">
        <div class="bg-white border border-zinc-200 rounded-xl px-4 py-3 rounded-bl-sm flex items-center gap-1.5">
          <span class="thinking-dot" />
          <span class="thinking-dot" style="animation-delay: 0.18s" />
          <span class="thinking-dot" style="animation-delay: 0.36s" />
        </div>
      </div>

      <!-- Initial load -->
      <div v-if="loading" class="flex justify-center py-4">
        <ProgressSpinner style="width: 24px; height: 24px" strokeWidth="4" />
      </div>
    </div>

    <!-- Input -->
    <div class="px-4 py-3 border-t border-zinc-200 bg-white">
      <div class="flex gap-2">
        <slot name="input-prefix" />
        <InputText
          v-model="inputText"
          :placeholder="placeholder"
          :maxlength="MESSAGE_CONTENT_MAX"
          class="flex-1"
          :disabled="disabled"
          @keydown.enter.exact="handleSend"
        />
        <Button
          icon="pi pi-send"
          :disabled="!canSend"
          severity="contrast"
          @click="handleSend"
        />
      </div>
      <div class="flex justify-between items-center mt-1 min-h-4">
        <slot name="input-footer" />
        <span
          v-if="inputText.length >= MESSAGE_COUNTER_THRESHOLD"
          class="text-xs"
          :class="inputText.length >= MESSAGE_CONTENT_MAX ? 'text-red-500' : 'text-zinc-400'"
        >
          {{ inputText.length }}/{{ MESSAGE_CONTENT_MAX }}
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import InputText from 'primevue/inputtext'
import Button from 'primevue/button'
import ProgressSpinner from 'primevue/progressspinner'
import MessageBubble from './MessageBubble.vue'
import { MESSAGE_CONTENT_MAX, MESSAGE_COUNTER_THRESHOLD } from '@/shared/validation/chatRules'
import type { ChatPanelMessage } from './chatPanelTypes'

export type { ChatPanelMessage }

const props = withDefaults(
  defineProps<{
    messages: ChatPanelMessage[]
    /** Initial page load in progress. */
    loading?: boolean
    /** LLM is generating — animated dots bubble. */
    thinking?: boolean
    /** Older messages exist; enables scroll-top pagination. */
    hasMore?: boolean
    loadingOlder?: boolean
    /** Blocks input entirely (e.g. model unavailable). */
    disabled?: boolean
    placeholder?: string
    /** Async older-page loader; the panel preserves scroll position. */
    loadOlder?: () => Promise<void>
  }>(),
  {
    loading: false,
    thinking: false,
    hasMore: false,
    loadingOlder: false,
    disabled: false,
    placeholder: 'Type a message...',
    loadOlder: undefined,
  }
)

const emit = defineEmits<{ send: [message: string] }>()

const inputText = ref('')
const messagesContainer = ref<HTMLElement | null>(null)

const canSend = computed(
  () =>
    !props.disabled &&
    !props.thinking &&
    inputText.value.trim().length > 0 &&
    inputText.value.length <= MESSAGE_CONTENT_MAX
)

function handleSend() {
  const text = inputText.value.trim()
  if (!text || !canSend.value) return
  emit('send', text)
  inputText.value = ''
}

function scrollToBottom() {
  const el = messagesContainer.value
  if (el) el.scrollTop = el.scrollHeight
}

// Infinite scroll up: load older near the top, preserving the viewport.
async function handleScroll() {
  const el = messagesContainer.value
  if (!el || !props.loadOlder || !props.hasMore || props.loadingOlder) return
  if (el.scrollTop > 60) return
  const prevHeight = el.scrollHeight
  await props.loadOlder()
  await nextTick()
  el.scrollTop += el.scrollHeight - prevHeight
}

// Autoscroll only when a NEW LAST message appears (appends), never on
// prepends from pagination — that was the old jump-to-bottom bug vector.
const lastId = computed(() =>
  props.messages.length > 0 ? props.messages[props.messages.length - 1]!.id : null
)
watch(lastId, async () => {
  await nextTick()
  scrollToBottom()
})
watch(
  () => props.thinking,
  async (isThinking) => {
    if (isThinking) {
      await nextTick()
      scrollToBottom()
    }
  }
)

defineExpose({ scrollToBottom })
</script>

<style scoped>
.thinking-dot {
  width: 7px;
  height: 7px;
  border-radius: 9999px;
  background-color: var(--p-zinc-400, #a1a1aa);
  animation: thinking-bounce 1.1s infinite ease-in-out;
}
@keyframes thinking-bounce {
  0%, 60%, 100% { transform: translateY(0); opacity: 0.5; }
  30% { transform: translateY(-4px); opacity: 1; }
}
</style>
