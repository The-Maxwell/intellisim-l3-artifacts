<template>
  <div class="bg-white rounded-lg border border-zinc-200 flex flex-col h-full overflow-hidden">
    <div class="px-3 py-2 border-b border-zinc-100 flex items-center justify-between gap-2">
      <span class="text-xs font-medium text-zinc-700 truncate">{{ title }}</span>
      <div class="flex items-center gap-1 shrink-0">
        <button
          v-if="!readonly"
          class="text-zinc-400 hover:text-violet-600"
          aria-label="Edit widget"
          @click="$emit('edit')"
        >
          <i class="pi pi-pencil text-xs" />
        </button>
        <button
          v-if="!readonly"
          class="text-zinc-400 hover:text-red-500"
          aria-label="Delete widget"
          @click="$emit('delete')"
        >
          <i class="pi pi-trash text-xs" />
        </button>
      </div>
    </div>
    <div class="flex-1 p-2 overflow-hidden min-h-0">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
// Preview chrome for the typed widgets dashboard: title bar + Edit/Delete
// actions (hidden when readonly) and a content slot. Mirrors the legacy
// WidgetBase chrome but emits edit/delete (the legacy one emits configure/remove)
// so it never collides with the legacy components.
defineProps<{ title: string; readonly?: boolean }>()
defineEmits<{ edit: []; delete: [] }>()
</script>
