<template>
  <span class="inline-flex items-center">
    <!-- Primary: Full regenerate. Secondary action opens the partial-scope popover. -->
    <SplitButton
      label="Generate"
      icon="pi pi-sparkles"
      size="small"
      :model="menuItems"
      :disabled="disabled"
      @click="confirmFull"
    />

    <!-- Partial-scope popover: generation-model override + environment-only +
         per-archetype (population / properties / placement) toggles. -->
    <Popover ref="popover">
      <div class="w-72 p-1 space-y-3">
        <div>
          <p class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400 mb-1">
            Generation model override
          </p>
          <LlmExecutionConfig
            :modelValue="generationTarget ?? null"
            capability="simulation_generation"
            @update:modelValue="(t) => emit('update:generationTarget', t)"
          />
        </div>

        <div>
          <p class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400 mb-1">
            Environment
          </p>
          <Button
            label="Regenerate environment"
            icon="pi pi-globe"
            size="small"
            severity="secondary"
            outlined
            class="w-full justify-start"
            :disabled="disabled"
            @click="emitEnvironment"
          />
        </div>

        <div v-if="archetypes.length">
          <p class="text-[11px] font-semibold uppercase tracking-wide text-zinc-400 mb-1">
            Agents by archetype
          </p>
          <div
            v-for="arch in archetypes"
            :key="arch.uid"
            class="rounded-md border border-zinc-200 p-2 mb-2 space-y-1.5"
          >
            <p class="text-xs font-medium text-zinc-700">{{ arch.name || arch.uid }}</p>
            <div class="flex flex-col gap-1">
              <label
                v-for="part in PARTS"
                :key="part"
                class="flex items-center gap-2 text-xs text-zinc-600 cursor-pointer"
              >
                <Checkbox
                  :modelValue="selectedParts[arch.uid]?.includes(part) ?? false"
                  :binary="true"
                  @update:modelValue="(on) => togglePart(arch.uid, part, !!on)"
                />
                <span class="capitalize">{{ part }}</span>
              </label>
            </div>
            <Button
              label="Regenerate"
              icon="pi pi-refresh"
              size="small"
              severity="secondary"
              text
              class="w-full justify-start"
              :disabled="disabled || !(selectedParts[arch.uid]?.length)"
              @click="emitAgents(arch.uid)"
            />
          </div>
        </div>
      </div>
    </Popover>
  </span>
</template>

<script setup lang="ts">
// SplitButton driving simulation (re)generation. The primary action is a full
// regenerate (behind a confirm dialog, since it overwrites the current generated
// state). The dropdown menu + popover build PARTIAL scopes: environment-only,
// and per-archetype agent regen with population/properties/placement toggles.
// Emits a GenerationScope upward; the editor calls store.generate({ scope }).
// Disabled entirely while a generation run is in progress.
import { reactive, ref } from 'vue'
import SplitButton from 'primevue/splitbutton'
import Popover from 'primevue/popover'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import { useConfirm } from 'primevue/useconfirm'
import LlmExecutionConfig from '@/shared/components/LlmExecutionConfig.vue'
import type { MenuItem } from 'primevue/menuitem'
import type {
  ExecutionLlmTarget,
  GenerationScope,
  GenerationScopePart,
  SnapshotAgent,
} from '@/shared/types'

defineProps<{
  archetypes: SnapshotAgent[]
  disabled?: boolean
  // The current generation-model override (v-model). Pre-filled from the
  // simulation's persisted llm_generation_target; flows into GenerateRequest.
  generationTarget?: ExecutionLlmTarget | null
}>()
const emit = defineEmits<{
  generate: [scope: GenerationScope]
  'update:generationTarget': [t: ExecutionLlmTarget]
}>()

const confirm = useConfirm()
const popover = ref<InstanceType<typeof Popover> | null>(null)

const PARTS: GenerationScopePart[] = ['population', 'properties', 'placement']

// Per-archetype selected parts (uid → list of parts).
const selectedParts = reactive<Record<string, GenerationScopePart[]>>({})

// Dropdown menu items: a single entry that opens the partial-scope popover.
const menuItems = ref<MenuItem[]>([
  {
    label: 'Partial regenerate…',
    icon: 'pi pi-sliders-h',
    command: (e) => {
      // The menu item's originalEvent carries the DOM event for popover anchoring.
      const ev = (e as { originalEvent?: Event }).originalEvent
      if (ev) popover.value?.toggle(ev)
    },
  },
])

function confirmFull(event: Event) {
  confirm.require({
    target: event.currentTarget as HTMLElement,
    message: 'This overwrites the current generated state. Continue?',
    header: 'Full regenerate',
    icon: 'pi pi-exclamation-triangle',
    rejectProps: { label: 'Cancel', severity: 'secondary', size: 'small' },
    acceptProps: { label: 'Regenerate', size: 'small' },
    accept: () => emit('generate', { kind: 'full' }),
  })
}

function emitEnvironment() {
  emit('generate', { kind: 'environment' })
  popover.value?.hide()
}

function togglePart(uid: string, part: GenerationScopePart, on: boolean) {
  const list = selectedParts[uid] ?? []
  selectedParts[uid] = on ? [...new Set([...list, part])] : list.filter((p) => p !== part)
}

function emitAgents(uid: string) {
  const parts = selectedParts[uid] ?? []
  if (!parts.length) return
  emit('generate', { kind: 'agents', agent_uids: [uid], parts: [...parts] })
  popover.value?.hide()
}
</script>
