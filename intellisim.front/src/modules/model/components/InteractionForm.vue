<template>
  <MasterDetailList
    :items="modelValue"
    :selectedIndex="clampedIndex"
    addLabel="Add interaction"
    emptyText="No interactions yet."
    @update:selectedIndex="(i) => emit('update:selectedIndex', i)"
    @add="addInteraction"
  >
    <template #item="{ item, index }">
      <span class="truncate flex-1">{{ item.name || item.id || `Interaction ${index + 1}` }}</span>
      <i v-if="item.locked" class="pi pi-lock text-amber-500 text-[10px] flex-none" />
      <span v-if="!item.enabled" class="text-[9px] uppercase tracking-wide text-zinc-400 flex-none">off</span>
    </template>
    <template #detail="{ item, index }">
      <InteractionItemForm
        :modelValue="item"
        :model="model"
        @update:modelValue="(it) => patch(index, it)"
        @remove="removeAt(index)"
      />
    </template>
  </MasterDetailList>
</template>

<script setup lang="ts">
// Master–detail host for the model's InteractionDTO[]: a selectable list + the
// selected interaction's InteractionItemForm. Selection controlled via
// v-model:selectedIndex for summary deep-links. Owns the array; emits the full list.
import { computed } from 'vue'
import MasterDetailList from '@/shared/components/MasterDetailList.vue'
import InteractionItemForm from './InteractionItemForm.vue'
import type { InteractionDTO, Model } from '@/shared/types'

const props = defineProps<{ modelValue: InteractionDTO[]; model: Model; selectedIndex: number }>()
const emit = defineEmits<{
  'update:modelValue': [v: InteractionDTO[]]
  'update:selectedIndex': [i: number]
}>()

const clampedIndex = computed(() =>
  props.modelValue.length ? Math.min(Math.max(0, props.selectedIndex), props.modelValue.length - 1) : 0
)

function patch(index: number, it: InteractionDTO) {
  emit('update:modelValue', props.modelValue.map((x, i) => (i === index ? it : x)))
}
function removeAt(index: number) {
  emit('update:modelValue', props.modelValue.filter((_, i) => i !== index))
  emit('update:selectedIndex', Math.max(0, Math.min(index, props.modelValue.length - 2)))
}
function addInteraction() {
  const blank: InteractionDTO = {
    id: '',
    name: '',
    description: '',
    type: 'agent_agent',
    conditions: [],
    initiator_actions: [],
    target_actions: [],
    bidirectional: false,
    use_llm: false,
    enabled: true,
    locked: false,
  }
  emit('update:modelValue', [...props.modelValue, blank])
  emit('update:selectedIndex', props.modelValue.length)
}
</script>
