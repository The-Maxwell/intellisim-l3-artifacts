<template>
  <div class="rounded-lg border border-zinc-200 bg-white p-3 space-y-3">
    <!-- Header: name + color + lock + remove -->
    <div class="flex items-center gap-2">
      <div class="flex-1 min-w-0">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Name</label>
        <InputText
          :modelValue="modelValue.name"
          class="w-full"
          size="small"
          :maxlength="ENTITY_NAME_MAX"
          placeholder="Agent name"
          @update:modelValue="(v) => patch({ name: v ?? '' })"
        />
      </div>
      <!-- Colour (spatial only): the conventional `color` property drives how the
           agent renders on the world canvas. Native picker emits a #RRGGBB hex. -->
      <div v-if="isSpatial" class="w-16">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Color</label>
        <input
          type="color"
          class="w-full h-8 border border-zinc-200 rounded cursor-pointer"
          :value="colorHex"
          aria-label="Agent color"
          @input="onColor(($event.target as HTMLInputElement).value)"
        />
      </div>
      <div class="flex items-center gap-1 pt-4">
        <span v-tooltip.top="lockTooltip" class="flex items-center gap-1">
          <i :class="['pi text-xs', modelValue.locked ? 'pi-lock text-amber-500' : 'pi-lock-open text-zinc-400']" />
          <ToggleSwitch
            :modelValue="modelValue.locked"
            @update:modelValue="(v) => patch({ locked: !!v })"
          />
        </span>
        <Button
          icon="pi pi-trash"
          severity="danger"
          text
          rounded
          size="small"
          aria-label="Remove agent"
          @click="emit('remove')"
        />
      </div>
    </div>

    <!-- uid / type / move_type -->
    <div class="flex gap-2">
      <div class="flex-1 min-w-0">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">UID</label>
        <InputText
          :modelValue="modelValue.uid"
          class="w-full"
          size="small"
          :maxlength="UID_MAX"
          placeholder="wolf"
          @update:modelValue="(v) => patch({ uid: v ?? '' })"
        />
      </div>
      <div class="w-32">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Type</label>
        <Select
          :modelValue="modelValue.type"
          :options="AGENT_TYPES as unknown as string[]"
          class="w-full"
          size="small"
          @update:modelValue="(v) => patch({ type: v })"
        />
      </div>
      <div class="w-32">
        <label class="text-[11px] text-zinc-500 mb-0.5 block">Move type</label>
        <Select
          :modelValue="modelValue.move_type"
          :options="MOVE_TYPES as unknown as string[]"
          class="w-full"
          size="small"
          @update:modelValue="(v) => patch({ move_type: v })"
        />
      </div>
    </div>

    <!-- description -->
    <div>
      <label class="text-[11px] text-zinc-500 mb-0.5 block">Description</label>
      <Textarea
        :modelValue="modelValue.description"
        class="w-full text-xs"
        rows="2"
        autoResize
        @update:modelValue="(v) => patch({ description: v ?? '' })"
      />
    </div>

    <!-- states -->
    <div>
      <label class="text-[11px] text-zinc-500 mb-0.5 block">States</label>
      <InputChips
        :modelValue="modelValue.states ?? []"
        class="w-full"
        @update:modelValue="(v) => patch({ states: v ?? [] })"
      />
    </div>

    <!-- Initialization -->
    <Fieldset legend="Initialization" :toggleable="true" :collapsed="true" class="text-sm">
      <InitializationForm
        :modelValue="init"
        :states="modelValue.states ?? []"
        @update:modelValue="(i) => patch({ initialization: i })"
      />
    </Fieldset>

    <!-- Missing-colour hint (spatial only): appears if the user removed the
         conventional `color` property. Rendering then falls back to a palette
         colour; a one-click add restores an explicit, editable colour. -->
    <Message
      v-if="isSpatial && !hasColor"
      severity="warn"
      :closable="false"
      class="text-xs"
    >
      <div class="flex items-center justify-between gap-2">
        <span>No colour set — this agent renders with a default palette colour.</span>
        <Button label="Add colour" icon="pi pi-plus" size="small" text @click="onColor(defaultColor)" />
      </div>
    </Message>

    <!-- Properties -->
    <Fieldset legend="Properties" :toggleable="true" :collapsed="true" class="text-sm">
      <PropertyEditor
        :modelValue="modelValue.properties ?? []"
        @update:modelValue="(p) => patch({ properties: p })"
      />
    </Fieldset>

    <!-- Simulation body + display -->
    <Fieldset legend="Body & display" :toggleable="true" :collapsed="true" class="text-sm">
      <div class="space-y-3">
        <!-- Simulation body (footprint) -->
        <div class="space-y-2">
          <div class="w-44">
            <label class="text-[11px] text-zinc-500 mb-0.5 block">Simulation body</label>
            <Select
              :modelValue="footprintKind"
              :options="FOOTPRINT_KINDS as unknown as string[]"
              class="w-full"
              size="small"
              @update:modelValue="(v) => setFootprintKind(v as FootprintKind)"
            />
          </div>

          <!-- circle -->
          <div v-if="footprintKind === 'circle'" class="w-32">
            <label class="text-[11px] text-zinc-500 mb-0.5 block">Radius</label>
            <InputNumber
              :modelValue="modelValue.footprint?.radius ?? null"
              class="w-full"
              size="small"
              :min="0"
              @update:modelValue="(v) => patchFootprint({ radius: v ?? undefined })"
            />
          </div>

          <!-- rectangle -->
          <div v-else-if="footprintKind === 'rectangle'" class="flex gap-2">
            <div class="w-28">
              <label class="text-[11px] text-zinc-500 mb-0.5 block">Width</label>
              <InputNumber
                :modelValue="modelValue.footprint?.width ?? null"
                class="w-full"
                size="small"
                :min="0"
                @update:modelValue="(v) => patchFootprint({ width: v ?? undefined })"
              />
            </div>
            <div class="w-28">
              <label class="text-[11px] text-zinc-500 mb-0.5 block">Height</label>
              <InputNumber
                :modelValue="modelValue.footprint?.height ?? null"
                class="w-full"
                size="small"
                :min="0"
                @update:modelValue="(v) => patchFootprint({ height: v ?? undefined })"
              />
            </div>
          </div>

          <!-- polygon: draw the footprint on a mini-canvas (centered on the agent) -->
          <div v-else-if="footprintKind === 'polygon'" class="space-y-1">
            <p class="text-[11px] text-zinc-400 leading-snug">
              Draw the body — click to add points, drag a point to move,
              double-click to remove. (0,0) is the agent center.
            </p>
            <PolygonDrawCanvas
              :modelValue="footprintPoints"
              @update:modelValue="setFootprintPoints"
            />
          </div>

          <!-- collidable (only meaningful when the world blocks overlaps) -->
          <label
            v-if="collisionMode === 'block'"
            class="flex items-center gap-2 text-xs text-zinc-600 cursor-pointer"
          >
            <ToggleSwitch
              :modelValue="!!modelValue.collidable"
              @update:modelValue="(v) => patch({ collidable: v ? true : undefined })"
            />
            <span>Collidable (participates in overlap resolution)</span>
          </label>
        </div>

        <!-- Display (visual) -->
        <DisplayEditor
          :modelValue="modelValue.display"
          @update:modelValue="(d) => patch({ display: d })"
        />
      </div>
    </Fieldset>

    <!-- Per-agent LLM config override -->
    <Fieldset legend="LLM config override" :toggleable="true" :collapsed="true" class="text-sm">
      <div class="space-y-2">
        <label class="flex items-center gap-2 text-xs text-zinc-600 cursor-pointer">
          <ToggleSwitch :modelValue="hasLlmOverride" @update:modelValue="onToggleLlm" />
          <span>{{ hasLlmOverride ? 'Override model default' : 'Inherit model default' }}</span>
        </label>
        <LlmConfigForm
          v-if="hasLlmOverride && modelValue.llm_config"
          :modelValue="modelValue.llm_config"
          variant="agent"
          @update:modelValue="(c) => patch({ llm_config: c as LLMAgentConfigDTO })"
        />
      </div>
    </Fieldset>
  </div>
</template>

<script setup lang="ts">
// Edits one Agent (used in a list managed by the parent). Embeds PropertyEditor
// for properties and an optional per-agent LlmConfigForm override. The `locked`
// toggle prevents the LLM from modifying this agent. Always emits the full agent.
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import Select from 'primevue/select'
import InputNumber from 'primevue/inputnumber'
import InputChips from 'primevue/inputchips'
import ToggleSwitch from 'primevue/toggleswitch'
import Fieldset from 'primevue/fieldset'
import Button from 'primevue/button'
import Message from 'primevue/message'
import Tooltip from 'primevue/tooltip'
import PropertyEditor from './PropertyEditor.vue'
import LlmConfigForm from './LlmConfigForm.vue'
import DisplayEditor from './DisplayEditor.vue'
import PolygonDrawCanvas from './PolygonDrawCanvas.vue'
import InitializationForm from './InitializationForm.vue'
import { DEFAULT_SHAPE_COLOR, entityColor, hasColorProperty, setEntityColor } from './environmentCanvasGeometry'
import { computed } from 'vue'
import type {
  Agent,
  AgentInitializationDTO,
  AgentFootprintDTO,
  FootprintKind,
  PointDTO,
  LLMAgentConfigDTO,
  ExecutionLlmTarget,
} from '@/shared/types'
import {
  AGENT_TYPES,
  MOVE_TYPES,
  FOOTPRINT_KINDS,
  ENTITY_NAME_MAX,
  UID_MAX,
} from '@/shared/validation/modelRules'

// Local directive registration (PrimeVue Tooltip) — components import their own.
const vTooltip = Tooltip

// collisionMode is the model environment's collision policy; the collidable
// toggle is only shown when it is 'block'.
const props = defineProps<{ modelValue: Agent; collisionMode?: string; isSpatial?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [v: Agent]; remove: [] }>()

const lockTooltip = 'Locked: the LLM will not modify this agent'

function patch(partial: Partial<Agent>) {
  emit('update:modelValue', { ...props.modelValue, ...partial })
}

// --- colour (stored as the conventional `color` property; spatial models only) ---
const defaultColor = DEFAULT_SHAPE_COLOR
const hasColor = computed(() => hasColorProperty(props.modelValue))
const colorHex = computed(() => entityColor(props.modelValue) ?? defaultColor)
function onColor(hex: string) {
  emit('update:modelValue', setEntityColor(props.modelValue, hex))
}

// --- initialization ---
// The count/placement UI lives in the shared InitializationForm; here we only
// surface the current initialization (default an empty one) and funnel edits back.
const init = computed<AgentInitializationDTO>(
  () => props.modelValue.initialization ?? { population_count: 0 }
)

// --- simulation body (footprint) ---
const footprintKind = computed<FootprintKind>(() => props.modelValue.footprint?.kind ?? 'point')
const footprintPoints = computed<PointDTO[]>(() => props.modelValue.footprint?.points ?? [])

function setFootprintKind(kind: FootprintKind) {
  if (kind === 'point') {
    // point = default → drop the footprint entirely.
    patch({ footprint: undefined })
    return
  }
  patch({ footprint: { ...(props.modelValue.footprint ?? {}), kind } as AgentFootprintDTO })
}
function patchFootprint(partial: Partial<AgentFootprintDTO>) {
  const current = props.modelValue.footprint ?? { kind: footprintKind.value }
  patch({ footprint: { ...current, ...partial } as AgentFootprintDTO })
}
function setFootprintPoints(points: PointDTO[]) {
  patchFootprint({ points })
}

// --- LLM override ---
const hasLlmOverride = computed(() => !!props.modelValue.llm_config)
function emptyAgentLlmConfig(): LLMAgentConfigDTO {
  return {
    target: {} as ExecutionLlmTarget,
    strategy: {
      type: 'none',
      batch_size: 0,
      use_for_movement: false,
      use_for_interaction: false,
      use_for_properties: false,
      use_for_decisions: false,
      decision_frequency: 0,
      // A brand-new per-archetype override defaults to distribution (members
      // diversify); variation_strength stays unset to use the backend default.
      archetype_mode: 'distribution',
    },
  }
}
function onToggleLlm(on: unknown) {
  patch({ llm_config: on ? emptyAgentLlmConfig() : undefined })
}
</script>
