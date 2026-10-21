<template>
  <DataTable
    :value="instances"
    dataKey="id"
    size="small"
    paginator
    :rows="20"
    :rowsPerPageOptions="[20, 50, 100]"
    class="text-sm"
  >
    <template #empty>
      <div class="text-center text-zinc-400 py-4 text-xs">No instances yet.</div>
    </template>

    <!-- ID (read-only reference) -->
    <Column field="id" header="ID" :style="{ width: '64px' }">
      <template #body="{ data }">
        <span class="text-zinc-400 text-xs">{{ data.id }}</span>
      </template>
    </Column>

    <!-- Name (controlled) -->
    <Column field="name" header="Name" :style="{ minWidth: '140px' }">
      <template #body="{ data }">
        <InputText
          :modelValue="data.name ?? ''"
          class="w-full"
          size="small"
          @update:modelValue="(v) => updateField(data.id, 'name', v ?? '')"
        />
      </template>
    </Column>

    <!-- State (controlled) -->
    <Column field="state" header="State" :style="{ minWidth: '130px' }">
      <template #body="{ data }">
        <Select
          :modelValue="data.state ?? null"
          :options="states"
          class="w-full"
          size="small"
          :placeholder="states.length ? 'state' : 'no states'"
          @update:modelValue="(v) => updateField(data.id, 'state', v ?? '')"
        />
      </template>
    </Column>

    <!-- X / Y (spatial only, controlled) -->
    <Column v-if="spatial" header="X" :style="{ width: '90px' }">
      <template #body="{ data }">
        <InputNumber
          :modelValue="data.position?.x ?? null"
          class="w-full"
          size="small"
          @update:modelValue="(v) => setPos(data.id, 'x', v)"
        />
      </template>
    </Column>
    <Column v-if="spatial" header="Y" :style="{ width: '90px' }">
      <template #body="{ data }">
        <InputNumber
          :modelValue="data.position?.y ?? null"
          class="w-full"
          size="small"
          @update:modelValue="(v) => setPos(data.id, 'y', v)"
        />
      </template>
    </Column>

    <!-- One column per snapshot property, typed controlled editor -->
    <Column
      v-for="prop in properties"
      :key="prop.uid"
      :header="prop.name || prop.uid"
      :style="{ minWidth: '130px' }"
    >
      <template #body="{ data }">
        <!-- number -->
        <InputNumber
          v-if="prop.type === 'number'"
          :modelValue="propNumber(data, prop)"
          class="w-full"
          size="small"
          :min="prop.min_value ?? undefined"
          :max="prop.max_value ?? undefined"
          @update:modelValue="(v) => setProp(data.id, prop, v ?? undefined)"
        />
        <!-- boolean -->
        <Checkbox
          v-else-if="prop.type === 'boolean'"
          :modelValue="propBool(data, prop)"
          :binary="true"
          @update:modelValue="(v) => setProp(data.id, prop, !!v)"
        />
        <!-- enum -->
        <Select
          v-else-if="prop.type === 'enum'"
          :modelValue="data.properties?.[prop.uid] ?? null"
          :options="(prop.allowed_values ?? []) as unknown[]"
          class="w-full"
          size="small"
          showClear
          @update:modelValue="(v) => setProp(data.id, prop, v ?? undefined)"
        />
        <!-- color -->
        <ColorPicker
          v-else-if="prop.type === 'color'"
          :modelValue="propColor(data, prop)"
          format="hex"
          @update:modelValue="(v) => setProp(data.id, prop, normalizeHex(v))"
        />
        <!-- string (default) -->
        <InputText
          v-else
          :modelValue="propString(data, prop)"
          class="w-full"
          size="small"
          @update:modelValue="(v) => setProp(data.id, prop, v ?? '')"
        />
      </template>
    </Column>

    <!-- Delete -->
    <Column :style="{ width: '56px' }">
      <template #body="{ data }">
        <Button
          icon="pi pi-trash"
          severity="danger"
          text
          rounded
          size="small"
          aria-label="Delete instance"
          @click="removeInstance(data.id)"
        />
      </template>
    </Column>
  </DataTable>
</template>

<script setup lang="ts">
// An editable table of SimulationAgent instances for ONE archetype. Columns:
// Name, State (from the snapshot archetype's states), X/Y (spatial only), then
// one TYPED editor column per snapshot property. EVERY editor is a single
// CONTROLLED input (:modelValue + @update:modelValue) — there is no PrimeVue
// cell-edit mode. Each change routes through exactly one mutator (updateField /
// setPos / setProp) that rebuilds the instances array immutably (keyed by id)
// and emits update:instances ONCE, always carrying the fresh row. Correctness
// over polish — large populations paginate (rows 20).
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Select from 'primevue/select'
import Checkbox from 'primevue/checkbox'
import ColorPicker from 'primevue/colorpicker'
import Button from 'primevue/button'
import { normalizeHex } from '@/modules/simulation/colorUtil'
import type { SimulationAgent, SnapshotProperty } from '@/shared/types'

const props = defineProps<{
  instances: SimulationAgent[]
  states: string[]
  properties: SnapshotProperty[]
  spatial: boolean
}>()
const emit = defineEmits<{ 'update:instances': [v: SimulationAgent[]] }>()

function propNumber(agent: SimulationAgent, prop: SnapshotProperty): number | null {
  const v = agent.properties?.[prop.uid]
  return typeof v === 'number' ? v : null
}
function propBool(agent: SimulationAgent, prop: SnapshotProperty): boolean {
  return agent.properties?.[prop.uid] === true
}
function propString(agent: SimulationAgent, prop: SnapshotProperty): string {
  const v = agent.properties?.[prop.uid]
  return v == null ? '' : String(v)
}
function propColor(agent: SimulationAgent, prop: SnapshotProperty): string {
  const v = agent.properties?.[prop.uid]
  return typeof v === 'string' && v ? v : '#888888'
}

// Emit a new instances array with the agent matched by id replaced by `next`.
// `next` is built from the CURRENT row in props.instances (not a captured
// snapshot), so the emitted value is always fresh.
function emitReplaced(id: number, next: (current: SimulationAgent) => SimulationAgent) {
  emit(
    'update:instances',
    props.instances.map((a) => (a.id === id ? next(a) : a))
  )
}

// Name / State — top-level scalar fields.
function updateField(id: number, field: 'name' | 'state', value: string) {
  emitReplaced(id, (current) => ({ ...current, [field]: value }))
}

function setPos(id: number, axis: 'x' | 'y', value: number | null) {
  emitReplaced(id, (current) => {
    const position = { x: current.position?.x ?? 0, y: current.position?.y ?? 0 }
    position[axis] = value ?? 0
    return { ...current, position }
  })
}

function setProp(id: number, prop: SnapshotProperty, value: unknown) {
  emitReplaced(id, (current) => {
    const properties = { ...(current.properties ?? {}) }
    if (value === undefined) {
      delete properties[prop.uid]
    } else {
      properties[prop.uid] = value
    }
    return { ...current, properties }
  })
}

function removeInstance(id: number) {
  emit(
    'update:instances',
    props.instances.filter((a) => a.id !== id)
  )
}
</script>
