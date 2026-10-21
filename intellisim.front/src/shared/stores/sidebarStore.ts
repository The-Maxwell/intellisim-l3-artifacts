import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { ChatResponse } from '@/shared/types'

export const useSidebarStore = defineStore('sidebar', () => {
  const collapsed = ref(false)
  const generalChats = ref<ChatResponse[]>([])

  function toggle() {
    collapsed.value = !collapsed.value
  }

  function addGeneralChat(chat: ChatResponse) {
    const exists = generalChats.value.find((c) => c.id === chat.id)
    if (!exists) {
      generalChats.value.unshift(chat)
    }
  }

  function removeGeneralChat(id: string) {
    generalChats.value = generalChats.value.filter((c) => c.id !== id)
  }

  return {
    collapsed,
    generalChats,
    toggle,
    addGeneralChat,
    removeGeneralChat,
  }
})
