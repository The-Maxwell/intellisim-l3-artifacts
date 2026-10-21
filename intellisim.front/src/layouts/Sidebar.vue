<template>
  <aside
    :class="[
      'h-full flex flex-col bg-zinc-900 text-white transition-all duration-200',
      sidebarStore.collapsed ? 'w-16' : 'w-64',
    ]"
  >
    <!-- Header -->
    <div
      :class="[
        'flex items-center border-b border-zinc-700',
        sidebarStore.collapsed ? 'p-2 justify-center' : 'p-4 gap-3',
      ]"
    >
      <template v-if="!sidebarStore.collapsed">
        <div class="w-8 h-8 bg-violet-500 rounded-lg flex items-center justify-center font-bold text-sm shrink-0">
          IS
        </div>
        <span class="font-semibold text-sm truncate">IntelliSim</span>
        <button
          class="ml-auto text-zinc-400 hover:text-white"
          @click="sidebarStore.toggle()"
          title="Collapse sidebar"
        >
          <i class="pi pi-angle-double-left text-sm" />
        </button>
      </template>
      <button
        v-else
        class="w-10 h-10 rounded-lg hover:bg-zinc-800 flex items-center justify-center text-zinc-400 hover:text-white"
        @click="sidebarStore.toggle()"
        title="Expand sidebar"
      >
        <i class="pi pi-angle-double-right text-sm" />
      </button>
    </div>

    <!-- Action Buttons -->
    <div v-if="!sidebarStore.collapsed" class="p-3 space-y-2">
      <button
        class="w-full flex items-center gap-2 px-3 py-2 rounded-lg bg-violet-500 hover:bg-violet-600 text-white text-sm font-medium transition-colors"
        @click="$router.push('/models')"
      >
        <i class="pi pi-plus text-xs" />
        New Model
      </button>
      <button
        class="w-full flex items-center gap-2 px-3 py-2 rounded-lg border border-zinc-600 hover:bg-zinc-800 text-zinc-300 text-sm transition-colors"
        @click="$router.push('/chat/new')"
      >
        <i class="pi pi-comment text-xs" />
        New General Chat
      </button>
    </div>
    <div v-else class="p-2 space-y-2 flex flex-col items-center">
      <button
        class="w-10 h-10 rounded-lg bg-violet-500 hover:bg-violet-600 flex items-center justify-center"
        @click="$router.push('/models')"
        title="New Model"
      >
        <i class="pi pi-plus text-xs" />
      </button>
      <button
        class="w-10 h-10 rounded-lg border border-zinc-600 hover:bg-zinc-800 flex items-center justify-center text-zinc-300"
        @click="$router.push('/chat/new')"
        title="New General Chat"
      >
        <i class="pi pi-comment text-xs" />
      </button>
    </div>

    <!-- Navigation Lists -->
    <nav class="flex-1 overflow-y-auto px-2 py-2 space-y-4">
      <!-- Models -->
      <div v-if="!sidebarStore.collapsed">
        <router-link
          to="/models"
          :class="[
            'flex items-center gap-2 px-2 py-1.5 rounded text-sm truncate transition-colors',
            $route.path === '/models'
              ? 'bg-violet-500/20 text-violet-300'
              : 'text-zinc-300 hover:bg-zinc-800',
          ]"
        >
          <i class="pi pi-box text-xs shrink-0" />
          <span class="truncate">Models</span>
        </router-link>
        <router-link
          to="/simulations"
          :class="[
            'flex items-center gap-2 px-2 py-1.5 rounded text-sm truncate transition-colors mt-0.5',
            $route.path === '/simulations'
              ? 'bg-violet-500/20 text-violet-300'
              : 'text-zinc-300 hover:bg-zinc-800',
          ]"
        >
          <i class="pi pi-play text-xs shrink-0" />
          <span class="truncate">Simulations</span>
        </router-link>
      </div>

      <!-- General Chats -->
      <div v-if="!sidebarStore.collapsed">
        <p class="px-2 text-xs uppercase tracking-wider text-zinc-500 mb-1">General Chats</p>
        <div v-if="sidebarStore.generalChats.length === 0" class="px-2 text-xs text-zinc-600">
          No chats yet
        </div>
        <router-link
          v-for="chat in sidebarStore.generalChats"
          :key="chat.id"
          :to="`/chat/${chat.id}`"
          :class="[
            'group flex items-center gap-2 px-2 py-1.5 rounded text-sm transition-colors mb-0.5',
            $route.name === 'chat' && $route.params.id === chat.id
              ? 'bg-violet-500/20 text-violet-300'
              : 'text-zinc-300 hover:bg-zinc-800',
          ]"
        >
          <i class="pi pi-comments text-xs shrink-0" />
          <span class="truncate flex-1 min-w-0">{{ chat.name || 'Untitled Chat' }}</span>
          <button
            class="shrink-0 opacity-0 group-hover:opacity-100 text-zinc-500 hover:text-red-400 transition-opacity"
            aria-label="Delete chat"
            title="Delete chat"
            @click.stop.prevent="confirmDeleteChat($event, chat)"
          >
            <i class="pi pi-trash text-xs" />
          </button>
        </router-link>
      </div>
    </nav>

    <!-- User -->
    <div class="p-2 border-t border-zinc-700">
      <UserAvatar :collapsed="sidebarStore.collapsed" />
    </div>
  </aside>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useConfirm } from 'primevue/useconfirm'
import { useSidebarStore } from '@/shared/stores/sidebarStore'
import { useLlmProviderStore } from '@/shared/stores/llmProviderStore'
import { useChatApi } from '@/modules/chat/composables/useChatApi'
import { useErrorHandler } from '@/shared/composables/useErrorHandler'
import UserAvatar from '@/shared/components/UserAvatar.vue'
import type { ChatResponse } from '@/shared/types'

const sidebarStore = useSidebarStore()
const providerStore = useLlmProviderStore()
const chatApi = useChatApi()
const route = useRoute()
const router = useRouter()
const confirm = useConfirm()
const { handleError, handleSuccess } = useErrorHandler()

onMounted(async () => {
  providerStore.loadProviders()
  try {
    const resp = await chatApi.listChats()
    sidebarStore.generalChats = resp.chats
  } catch {
    // unauthenticated boot or backend down — list stays empty
  }
})

function confirmDeleteChat(event: MouseEvent, chat: ChatResponse) {
  confirm.require({
    target: event.currentTarget as HTMLElement,
    message: `Delete "${chat.name || 'Untitled Chat'}"? This cannot be undone.`,
    header: 'Delete chat',
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: 'Cancel', severity: 'secondary', size: 'small' },
    acceptProps: { label: 'Delete', severity: 'danger', size: 'small' },
    accept: async () => {
      try {
        await chatApi.deleteChat(chat.id)
        sidebarStore.removeGeneralChat(chat.id)
        handleSuccess('Chat deleted')
        // The sidebar has no SSE echo for a chat that isn't currently open
        // (chat.deleted only reaches the open chat's own subscriber) — if the
        // deleted chat is the one being viewed, navigate away explicitly.
        if (route.name === 'chat' && route.params.id === chat.id) {
          router.replace('/chat/new')
        }
      } catch (err) {
        handleError(err, 'Delete chat')
      }
    },
  })
}
</script>
