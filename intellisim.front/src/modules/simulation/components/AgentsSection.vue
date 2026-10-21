<template>
  <p v-if="!archetypes.length" class="text-xs text-zinc-400">
    No archetypes in the model snapshot.
  </p>

  <!-- Master = archetypes (fixed set → no "Add" on the master); detail = the
       selected archetype's instance table + its "Add instance" button. -->
  <MasterDetailList
    v-else
    :items="archetypes"
    v-model:selectedIndex="sel"
  >
    <template #item="{ item }">
      <span class="truncate flex-1">{{ item.name || item.uid }}</span>
      <span class="text-[10px] font-mono text-zinc-400 flex-none tabular-nums">{{ groups[item.uid]?.length ?? 0 }}</span>
    </template>

    <template #detail="{ item }">
      <div class="space-y-2">
        <div class="flex items-center gap-2 flex-wrap">
          <span class="text-sm font-medium text-zinc-700">{{ item.name || item.uid }}</span>
          <Tag :value="`${groups[item.uid]?.length ?? 0} instance(s)`" severity="secondary" />
          <span class="text-[11px] text-zinc-400">{{ populationHint(item) }}</span>
          <Button
            label="Add instance"
            icon="pi pi-plus"
            size="small"
            severity="secondary"
            text
            class="ml-auto"
            @click="addInstance(item)"
          />
        </div>
        <AgentInstanceTable
          :instances="groups[item.uid] ?? []"
          :states="item.states ?? []"
          :properties="item.properties ?? []"
          :spatial="spatial"
          @update:instances="(list) => onGroupUpdated(item.uid, list)"
        />
      </div>
    </template>
  </MasterDetailList>
</template>

<script setup lang="ts">
// Master–detail over the model-snapshot archetypes: the left list picks an
// archetype, the detail shows that archetype's editable AgentInstanceTable (+ an
// "Add instance" button). Instances are grouped by agent_uid; edits / adds /
// deletes recompose the FULL flat instances array (other archetypes untouched)
// and emit it upward, where the parent applies it to store.current.agents +
// markDirty. (Archetypes are fixed by the model, so the master has no Add.)
import { computed, ref } from 'vue'
import Tag from 'primevue/tag'
import Button from 'primevue/button'
import MasterDetailList from '@/shared/components/MasterDetailList.vue'
import AgentInstanceTable from './AgentInstanceTable.vue'
import type { SimulationAgent, SnapshotAgent } from '@/shared/types'

const props = defineProps<{
  agents: SimulationAgent[]
  archetypes: SnapshotAgent[]
  spatial: boolean
}>()
const emit = defineEmits<{ 'update:agents': [v: SimulationAgent[]] }>()

// Which archetype is shown in the detail pane.
const sel = ref(0)

// Group instances by archetype uid for per-archetype rendering + counts.
const groups = computed<Record<string, SimulationAgent[]>>(() => {
  const m: Record<string, SimulationAgent[]> = {}
  for (const a of props.agents) {
    ;(m[a.agent_uid] ??= []).push(a)
  }
  return m
})

function populationHint(arch: SnapshotAgent): string {
  const min = arch.initialization?.min_population
  const max = arch.initialization?.max_population
  if (min == null && max == null) return ''
  return `min ${min ?? '—'} / max ${max ?? '—'}`
}

// Replace one archetype's instances within the flat array, leaving every other
// archetype's instances in place (and preserving their original order).
function onGroupUpdated(uid: string, list: SimulationAgent[]) {
  const others = props.agents.filter((a) => a.agent_uid !== uid)
  emit('update:agents', [...others, ...list])
}

function nextId(): number {
  return props.agents.reduce((max, a) => Math.max(max, a.id), 0) + 1
}

function addInstance(arch: SnapshotAgent) {
  const instance: SimulationAgent = {
    id: nextId(),
    agent_uid: arch.uid,
    name: '',
    state: arch.initialization?.initial_state ?? '',
    properties: {},
    ...(props.spatial ? { position: { x: 0, y: 0 } } : {}),
  }
  emit('update:agents', [...props.agents, instance])
}
</script>
