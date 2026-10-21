<template>
  <MasterDetailList
    :items="modelValue"
    :selectedIndex="clampedIndex"
    addLabel="Add rule"
    emptyText="No rules yet."
    @update:selectedIndex="(i) => emit('update:selectedIndex', i)"
    @add="addRule"
  >
    <template #item="{ item, index }">
      <span class="truncate flex-1">{{ item.name || item.id || `Rule ${index + 1}` }}</span>
      <i v-if="item.locked" class="pi pi-lock text-amber-500 text-[10px] flex-none" />
      <span v-if="!item.enabled" class="text-[9px] uppercase tracking-wide text-zinc-400 flex-none">off</span>
    </template>
    <template #detail="{ item, index }">
      <RuleForm
        :modelValue="item"
        :model="model"
        @update:modelValue="(r) => patch(index, r)"
        @remove="removeAt(index)"
      />
    </template>
  </MasterDetailList>
</template>

<script setup lang="ts">
// Master–detail host for the model's RuleDTO[]: a selectable rule list + the
// selected rule's RuleForm. Selection is controlled via v-model:selectedIndex so
// the summary can deep-link a specific rule. Owns add/remove/patch of the array
// and emits the full updated list.
import { computed } from 'vue'
import MasterDetailList from '@/shared/components/MasterDetailList.vue'
import RuleForm from './RuleForm.vue'
import type { RuleDTO, Model } from '@/shared/types'

const props = defineProps<{ modelValue: RuleDTO[]; model: Model; selectedIndex: number }>()
const emit = defineEmits<{
  'update:modelValue': [v: RuleDTO[]]
  'update:selectedIndex': [i: number]
}>()

// Guard the displayed selection against a stale index (e.g. after an external
// LLM update shrinks the list) without mutating the parent's ref.
const clampedIndex = computed(() =>
  props.modelValue.length ? Math.min(Math.max(0, props.selectedIndex), props.modelValue.length - 1) : 0
)

function patch(index: number, rule: RuleDTO) {
  emit('update:modelValue', props.modelValue.map((r, i) => (i === index ? rule : r)))
}
function removeAt(index: number) {
  emit('update:modelValue', props.modelValue.filter((_, i) => i !== index))
  // Clamp selection to the shrunk list (length-2 = last index after removal).
  emit('update:selectedIndex', Math.max(0, Math.min(index, props.modelValue.length - 2)))
}
function addRule() {
  const blank: RuleDTO = {
    id: '',
    name: '',
    description: '',
    priority: 0,
    applies_to: [],
    conditions: [],
    actions: [],
    execute_once: false,
    cooldown_steps: 0,
    enabled: true,
    locked: false,
  }
  emit('update:modelValue', [...props.modelValue, blank])
  emit('update:selectedIndex', props.modelValue.length) // select the new last item
}
</script>
