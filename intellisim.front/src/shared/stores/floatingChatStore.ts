import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useFloatingChatStore = defineStore('floatingChat', () => {
  const isOpen = ref(false)
  const chatId = ref<string | null>(null)

  function open() {
    isOpen.value = true
  }

  function close() {
    isOpen.value = false
  }

  function toggleOpen() {
    isOpen.value = !isOpen.value
  }

  function setChatId(id: string | null) {
    chatId.value = id
  }

  return {
    isOpen,
    chatId,
    open,
    close,
    toggleOpen,
    setChatId,
  }
})
