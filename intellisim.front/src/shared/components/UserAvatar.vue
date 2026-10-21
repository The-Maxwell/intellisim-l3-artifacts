<template>
  <div>
    <button
      class="w-full flex items-center gap-2 rounded-lg hover:bg-zinc-800 transition-colors p-1.5"
      :class="collapsed ? 'justify-center' : ''"
      @click="toggle"
    >
      <Avatar
        :label="initials"
        shape="circle"
        class="shrink-0"
        style="background-color: var(--p-violet-500); color: white"
      />
      <span v-if="!collapsed" class="text-sm text-zinc-300 truncate">{{ auth.displayName }}</span>
    </button>

    <Popover ref="popover">
      <div class="w-64 p-1">
        <div class="flex items-center gap-3 pb-3 border-b border-zinc-200">
          <Avatar :label="initials" shape="circle" size="large"
            style="background-color: var(--p-violet-500); color: white" />
          <div class="min-w-0">
            <p class="text-sm font-semibold text-zinc-800 truncate">{{ auth.displayName }}</p>
            <p class="text-xs text-zinc-500 truncate">{{ auth.email }}</p>
            <Tag v-if="auth.isAdmin" value="Admin" severity="warn" class="mt-1" />
          </div>
        </div>
        <Button
          label="Log out"
          icon="pi pi-sign-out"
          text
          severity="secondary"
          class="w-full mt-2 justify-start"
          @click="auth.logout()"
        />
      </div>
    </Popover>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Avatar from 'primevue/avatar'
import Popover from 'primevue/popover'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import { useAuthStore } from '@/modules/auth/stores/authStore'

defineProps<{ collapsed?: boolean }>()

const auth = useAuthStore()
const popover = ref<InstanceType<typeof Popover> | null>(null)

// Initials: first letters of the first two words of the display name,
// falling back to the email's first letter.
const initials = computed(() => {
  const name = auth.displayName.trim()
  if (name) {
    const parts = name.split(/[\s._-]+/).filter(Boolean)
    return parts.slice(0, 2).map((p) => p[0]!.toUpperCase()).join('') || '?'
  }
  return (auth.email[0] || '?').toUpperCase()
})

function toggle(event: Event) {
  popover.value?.toggle(event)
}

onMounted(() => {
  if (!auth.me) auth.loadMe()
})
</script>
