<template>
  <div class="flex flex-wrap items-center gap-2">
    <Select
      :modelValue="section"
      :options="sectionOptions"
      optionLabel="label"
      optionValue="value"
      placeholder="Whole model"
      class="w-44"
      size="small"
      @update:modelValue="onSectionChange"
    />
    <Select
      v-if="section && entityOptions.length"
      :modelValue="modelValue?.uid ?? ''"
      :options="entityOptions"
      optionLabel="label"
      optionValue="value"
      placeholder="Whole section"
      class="w-52"
      size="small"
      showClear
      @update:modelValue="onEntityChange"
    />
  </div>
</template>

<script setup lang="ts">
// Produces an EditScope { section?, uid? } for the model chat:
//   - no section            → whole model (undefined)
//   - section, no uid       → that whole section
//   - section + uid         → a single entity within the section
import { computed } from 'vue'
import Select from 'primevue/select'
import type { EditScope, EditSection, Model } from '@/shared/types'

const props = defineProps<{ model: Model; modelValue: EditScope | undefined }>()
const emit = defineEmits<{ 'update:modelValue': [v: EditScope | undefined] }>()

interface SectionOpt { label: string; value: EditSection | '' }

const sectionOptions = computed<SectionOpt[]>(() => {
  const opts: SectionOpt[] = [
    { label: 'Whole model', value: '' },
    { label: 'Agents', value: 'agents' },
    // Environment is targetable for every model — non-spatial models still have
    // global properties + LLM prompt context (only geometry/shapes are spatial).
    { label: 'Environment', value: 'environment' },
  ]
  opts.push(
    { label: 'Rules', value: 'rules' },
    { label: 'Interactions', value: 'interactions' },
    { label: 'Events', value: 'events' }
  )
  return opts
})

const section = computed(() => props.modelValue?.section ?? '')

interface EntityOpt { label: string; value: string }

// Entities for the chosen section, keyed by their stable identifier (uid for
// agents/environment/shapes/events; id for rules/interactions).
const entityOptions = computed<EntityOpt[]>(() => {
  const m = props.model
  switch (props.modelValue?.section) {
    case 'agents':
      return m.agents.map((a) => ({ label: a.name || a.uid, value: a.uid }))
    case 'environment':
      // The environment itself + each shape can be targeted by uid.
      return [
        ...(m.environment ? [{ label: 'Environment', value: m.environment.uid }] : []),
        ...(m.environment?.shapes ?? []).map((s) => ({ label: s.name || s.uid, value: s.uid })),
      ]
    case 'rules':
      return m.rules.map((r) => ({ label: r.name || r.id, value: r.id }))
    case 'interactions':
      return m.interactions.map((it) => ({ label: it.name || it.id, value: it.id }))
    case 'events':
      return m.events.map((e) => ({ label: e.name || e.uid, value: e.uid }))
    default:
      return []
  }
})

function onSectionChange(value: EditSection | '') {
  if (!value) emit('update:modelValue', undefined)
  else emit('update:modelValue', { section: value }) // reset uid when section changes
}

function onEntityChange(uid: string | null) {
  const sec = props.modelValue?.section
  if (!sec) return
  emit('update:modelValue', uid ? { section: sec, uid } : { section: sec })
}
</script>
