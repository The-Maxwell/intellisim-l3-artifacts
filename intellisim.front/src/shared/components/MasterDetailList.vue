<template>
  <div class="grid gap-4 items-start" style="grid-template-columns: 11rem minmax(0, 1fr)">
    <!-- Master list (sticky, so it stays put while a long detail form scrolls) -->
    <div class="sticky top-0 self-start flex flex-col gap-1">
      <button
        v-for="(item, i) in items"
        :key="i"
        type="button"
        class="flex items-center gap-2 rounded-md px-2.5 py-1.5 text-left text-sm transition"
        :class="i === selectedIndex
          ? 'bg-violet-50 text-violet-700 font-medium'
          : 'text-zinc-600 hover:bg-zinc-100'"
        @click="emit('update:selectedIndex', i)"
      >
        <slot name="item" :item="item" :index="i" :selected="i === selectedIndex">
          <span class="truncate flex-1">{{ i + 1 }}</span>
        </slot>
      </button>
      <!-- Add is optional: only shown when an addLabel is provided (e.g. lists with
           a fixed set of items, like archetypes, omit it). -->
      <Button
        v-if="addLabel"
        :label="addLabel"
        icon="pi pi-plus"
        size="small"
        severity="secondary"
        outlined
        class="mt-1"
        @click="emit('add')"
      />
    </div>

    <!-- Detail: the selected item's form -->
    <div class="min-w-0">
      <slot
        v-if="selectedIndex >= 0 && selectedIndex < items.length"
        name="detail"
        :item="selectedItem"
        :index="selectedIndex"
      />
      <div
        v-else
        class="grid place-items-center text-center text-sm text-zinc-400 py-12 px-4 border border-dashed border-zinc-200 rounded-lg"
      >
        {{ items.length ? 'Select an item to edit.' : (emptyText ?? 'Nothing here yet — add one to begin.') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts" generic="T">
// Reusable master–detail layout: a compact selectable item list (left, sticky) +
// a detail slot for the selected item (right). Selection is CONTROLLED via
// v-model:selectedIndex so callers can deep-link to a specific item; the host
// owns the items array, add (optional, via addLabel) and (via the detail form)
// remove. Render each row through the #item slot and the selected item's form
// through the #detail slot. Showing one item at a time also sidesteps the v-for
// index-key focus churn stacked lists carry. Lives in shared/ — used by both the
// model editor (rules/interactions/events/agents) and the simulation editor.
import { computed } from 'vue'
import Button from 'primevue/button'

const props = defineProps<{
  items: T[]
  selectedIndex: number
  addLabel?: string
  emptyText?: string
}>()
const emit = defineEmits<{
  'update:selectedIndex': [index: number]
  add: []
}>()

// Narrowed selected item for the #detail slot — only read when the template's
// v-if has confirmed the index is in range, so the cast is safe.
const selectedItem = computed(() => props.items[props.selectedIndex] as T)
</script>
