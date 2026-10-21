<template>
  <MasterDetailList
    :items="modelValue"
    :selectedIndex="clampedIndex"
    addLabel="Add event"
    emptyText="No events yet."
    @update:selectedIndex="(i) => emit('update:selectedIndex', i)"
    @add="addEvent"
  >
    <template #item="{ item, index }">
      <span class="truncate flex-1">{{ item.name || item.uid || `Event ${index + 1}` }}</span>
      <i v-if="item.locked" class="pi pi-lock text-amber-500 text-[10px] flex-none" />
    </template>
    <template #detail="{ item, index }">
      <EventItemForm
        :modelValue="item"
        :agents="agents"
        @update:modelValue="(ev) => patch(index, ev)"
        @remove="removeAt(index)"
      />
    </template>
  </MasterDetailList>
</template>

<script setup lang="ts">
// Master–detail host for the model's EventDefDTO[]: a selectable list + the
// selected event's EventItemForm. Selection controlled via v-model:selectedIndex
// for summary deep-links. Owns the array; emits the full list.
import { computed } from 'vue'
import MasterDetailList from '@/shared/components/MasterDetailList.vue'
import EventItemForm from './EventItemForm.vue'
import type { EventDefDTO, Agent } from '@/shared/types'

const props = defineProps<{ modelValue: EventDefDTO[]; agents: Agent[]; selectedIndex: number }>()
const emit = defineEmits<{
  'update:modelValue': [v: EventDefDTO[]]
  'update:selectedIndex': [i: number]
}>()

const clampedIndex = computed(() =>
  props.modelValue.length ? Math.min(Math.max(0, props.selectedIndex), props.modelValue.length - 1) : 0
)

function patch(index: number, ev: EventDefDTO) {
  emit('update:modelValue', props.modelValue.map((x, i) => (i === index ? ev : x)))
}
function removeAt(index: number) {
  emit('update:modelValue', props.modelValue.filter((_, i) => i !== index))
  emit('update:selectedIndex', Math.max(0, Math.min(index, props.modelValue.length - 2)))
}
function addEvent() {
  const blank: EventDefDTO = {
    uid: '',
    name: '',
    description: '',
    scope: 'global',
    payload: [],
    producers: [],
    ttl_steps: 0,
    locked: false,
  }
  emit('update:modelValue', [...props.modelValue, blank])
  emit('update:selectedIndex', props.modelValue.length)
}
</script>
