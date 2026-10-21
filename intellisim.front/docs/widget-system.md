# Widget System Guide

**Files**: 
- `src/shared/components/WidgetBase.vue` — Base container
- `src/shared/components/widgets/` — Widget implementations
- `src/shared/types/widget.ts` — Type definitions
- `src/modules/analysis/views/AnalysisView.vue` — Widget dashboard

The widget system provides a modular dashboard for post-simulation data visualization with drag-and-drop layout, configurable queries, and pluggable widget types.

## Widget Types

### Defined Types (widget.ts)

```typescript
export type WidgetType =
  | 'line_plot'          // ✅ Implemented: Time-series line chart
  | 'counter'            // ✅ Implemented: Single large number
  | 'population'         // ✅ Implemented: Multi-series population chart
  | 'bar_chart'          // ⏳ Not yet implemented
  | 'histogram'          // ⏳ Not yet implemented
  | 'agent_table'        // ⏳ Not yet implemented
```

### Implemented Widgets

#### 1. LinePlotWidget

**File**: `src/shared/components/widgets/LinePlotWidget.vue`

**Purpose**: Display time-series data (property value over steps) as a line chart.

**Component Props**:
```typescript
interface Props {
  config: WidgetConfig         // Widget metadata (title, query, position)
  data: TimeSeriesPoint[] | unknown  // Array of { step, value }
}
```

**Data Format**:
```typescript
interface TimeSeriesPoint {
  step: number      // Simulation step
  value: number | string  // Property value at step
}
```

**Chart Features**:
- X-axis: Step numbers
- Y-axis: Property values (auto-scaled)
- Line color: Violet (#8B5CF6)
- Fill: Light violet with 10% opacity
- Tension: 0.3 (smooth curves)
- Legend: Hidden (single series)

**Example Data**:
```json
[
  { "step": 0, "value": 50 },
  { "step": 1, "value": 52 },
  { "step": 2, "value": 55 }
]
```

**Chart.js Config**:
- CategoryScale, LinearScale
- Responsive, maintainAspectRatio: false
- Grid on Y-axis only

---

#### 2. CounterWidget

**File**: `src/shared/components/widgets/CounterWidget.vue`

**Purpose**: Display a single numeric or string value in large, centered text.

**Component Props**:
```typescript
interface Props {
  config: WidgetConfig
  data: unknown              // Single value (number, string, or null)
}
```

**Display Logic**:
```typescript
const displayValue = computed(() => {
  if (data == null) return '—'  // Null/undefined → em dash
  if (typeof data === 'number') return Math.round(data * 100) / 100  // Round to 2 decimals
  return String(data)  // Convert to string
})
```

**Styling**: 
- Text: text-3xl, font-bold, zinc-800
- Centered: flex items-center justify-center

**Use Cases**:
- Total agent count
- Current generation number
- Final population
- Simulation duration

---

#### 3. PopulationChartWidget

**File**: `src/shared/components/widgets/PopulationChartWidget.vue`

**Purpose**: Display population by archetype over time as multi-line chart.

**Component Props**:
```typescript
interface Props {
  config: WidgetConfig
  data: PopulationPoint[] | unknown
}
```

**Data Format**:
```typescript
interface PopulationPoint {
  step: number
  count: {
    [archetypeName: string]: number
  }
}
```

**Example Data**:
```json
[
  { "step": 0, "count": { "Predator": 10, "Prey": 100 } },
  { "step": 1, "count": { "Predator": 12, "Prey": 98 } }
]
```

**Chart Features**:
- One series per unique archetype (auto-detected)
- Color palette: 7 colors (violet, blue, green, red, amber, pink, teal)
- Legend: Bottom, with archetype names
- X-axis: Steps
- Y-axis: Population count
- Tension: 0.3 (smooth curves)

**Color Assignment**:
```typescript
const COLORS = [
  '#8B5CF6', '#3B82F6', '#22C55E', '#EF4444', '#F59E0B', '#EC4899', '#14B8A6'
]
// Colors assigned in order: first archetype → first color, etc.
// If more archetypes than colors, cycle through palette
```

---

### Not Yet Implemented

#### 4. BarChartWidget (Placeholder)
- Type: 'bar_chart'
- Use Case: Categorical comparison (agents by location, etc.)
- Template: Follow LinePlotWidget pattern with BarChart from Chart.js

#### 5. HistogramWidget (Placeholder)
- Type: 'histogram'
- Use Case: Distribution of agent properties
- Data: Array of numeric values
- Library: Chart.js histogram plugin or custom implementation

#### 6. AgentTableWidget (Placeholder)
- Type: 'agent_table'
- Use Case: Sortable/filterable agent list at current step
- Data: Array of agents with selectable properties
- Library: PrimeVue DataTable or custom table

## WidgetConfig Schema

```typescript
export interface WidgetConfig {
  id: string                    // Unique widget ID (UUID or generated)
  type: WidgetType              // One of the 6 types above
  title: string                 // Display title (e.g., "Predator Population")
  query: ResultQuery            // Query to fetch data
  position: WidgetPosition      // Grid layout position
  options?: Record<string, unknown>  // Type-specific options (for future expansion)
}

export interface WidgetPosition {
  x: number                     // Grid column index
  y: number                     // Grid row index
  w: number                     // Width in grid units (1-12)
  h: number                     // Height in grid units (1-8)
}

export interface ResultQuery {
  type: string                  // 'timeseries', 'population', 'summary', etc.
  property_name?: string        // For timeseries: property to fetch
  property?: string             // Alternative field name
  step?: number                 // For spatial: specific step
  // ... other query params
}
```

**Query Examples**:

```typescript
// Time-series: Population density over time
{
  type: 'timeseries',
  property_name: 'population_density'
}

// Population: Count by archetype
{
  type: 'population'
}

// Spatial: Agents at step 100
{
  type: 'spatial',
  step: 100
}
```

## WidgetBase Container

**File**: `src/shared/components/WidgetBase.vue`

Base component wrapping all widgets with consistent styling, header, and controls.

**Props**:
```typescript
interface Props {
  title: string                 // Widget title (displayed in header)
}
```

**Emits**:
```typescript
interface Emits {
  configure: []                 // Configure button clicked
  remove: []                    // Remove button clicked
}
```

**Structure**:
```
┌─────────────────────────────────┐
│ [Title]          [⚙️] [❌]      │  Header: title + controls
├─────────────────────────────────┤
│                                 │
│      <slot>                     │  Content area
│   (widget implementation)        │
│                                 │
└─────────────────────────────────┘
```

**Styling**:
- Background: white
- Border: zinc-200 1px
- Rounded corners: lg
- Height: Full flex container
- Header: Flex, justify-between, border-bottom

**Controls**:
- **⚙️ (Gear)**: Emits 'configure' event (opens config modal)
- **❌ (Close)**: Emits 'remove' event (deletes widget from dashboard)

## Dashboard Layout System

**File**: `src/modules/analysis/views/AnalysisView.vue`

Uses `grid-layout-plus` library for drag-and-drop, resizable widget grid.

### Grid Setup

```typescript
import { GlFormItem, GlFormCheckbox, GlFormInput } from 'grid-layout-plus'

const layout = ref<WidgetConfig[]>([
  {
    id: 'widget1',
    type: 'counter',
    title: 'Total Agents',
    query: { type: 'summary', property: 'total_agents' },
    position: { x: 0, y: 0, w: 3, h: 2 }
  },
  // ... more widgets
])

const draggable = ref(true)
const resizable = ref(true)
const isDragging = ref(false)
```

### Grid Properties

- **Columns**: 12 (responsive breakpoints possible)
- **Row Height**: Typically 80-100px per unit
- **Gap**: 12-16px between widgets
- **Draggable**: true (users can reorder)
- **Resizable**: true (users can resize)

### Adding Widgets

```typescript
function addWidget(type: WidgetType) {
  const id = `widget_${Date.now()}`
  layout.value.push({
    id,
    type,
    title: `${type} Widget`,
    query: {},
    position: { x: 0, y: layout.value.length, w: 6, h: 3 }
  })
}
```

### Widget Presets

```typescript
export interface WidgetPreset {
  name: string                  // Preset name (e.g., "Predator-Prey Analysis")
  description: string           // Description
  widgets: Omit<WidgetConfig, 'id'>[]  // Widget configs without IDs
}

const presets: WidgetPreset[] = [
  {
    name: 'Population Overview',
    description: 'Track population trends by archetype',
    widgets: [
      {
        type: 'population',
        title: 'Population Over Time',
        query: { type: 'population' },
        position: { x: 0, y: 0, w: 12, h: 4 }
      }
    ]
  }
]
```

Users can load presets from dropdown → auto-populates dashboard.

## Data Sources

### Real-Time vs API

**Real-Time Data** (During Simulation):
- Source: `useWebSocketStream` (simulation runner)
- Data: latestAgents, latestShapes, progress
- Latency: <100ms
- Use Case: Live agent count, current step display
- Composable: Not directly used in AnalysisView

**API Data** (After Simulation):
- Source: `useSimulationApi` result endpoints
- Data: Fetched on-demand via composable
- Latency: Depends on query complexity, backend performance
- Use Case: Historical analysis, detailed queries
- Composable: `useSimulationApi.getTimeSeries()`, `.getPopulation()`, etc.

### Caching Strategy

AnalysisView can cache results to minimize API calls:

```typescript
const resultCache = reactive<Map<string, unknown>>(new Map())

async function getWidgetData(widget: WidgetConfig) {
  const cacheKey = `${simulationId}:${JSON.stringify(widget.query)}`
  
  if (resultCache.has(cacheKey)) {
    return resultCache.get(cacheKey)
  }
  
  const data = await queryResults(simulationId, widget.query)
  resultCache.set(cacheKey, data)
  return data
}
```

**Invalidation**: Cache cleared when simulation re-runs or user refreshes.

## Grid Layout Persistence

### Save Layout

```typescript
function saveLayout() {
  const layoutJson = JSON.stringify(layout.value)
  localStorage.setItem(`simulation_${simulationId}:layout`, layoutJson)
}

function loadLayout() {
  const saved = localStorage.getItem(`simulation_${simulationId}:layout`)
  if (saved) {
    layout.value = JSON.parse(saved)
  }
}
```

**Current State**: Not yet implemented (planned feature).

### Server Persistence (Future)

```typescript
async function saveLayoutToServer() {
  await apiClient.post(`/simulations/${simulationId}/layouts`, {
    name: 'Custom Analysis',
    widgets: layout.value
  })
}
```

Backend endpoint needed: `POST /simulations/:id/layouts`

## Widget Configuration Dialog

When user clicks gear icon on widget:

```typescript
function handleConfigure(widget: WidgetConfig) {
  selectedWidget.value = widget
  showConfigModal.value = true
}

async function saveWidgetConfig(updatedWidget: WidgetConfig) {
  const index = layout.value.findIndex(w => w.id === updatedWidget.id)
  if (index >= 0) {
    layout.value[index] = updatedWidget
    // Re-fetch data with new query
    await refreshWidget(updatedWidget.id)
  }
}
```

Modal fields:
- Title input (string)
- Property selector (dropdown from `getProperties()`)
- Query type selector (dropdown)
- Widget type selector (if changing type)

## Error Handling

### Missing Data

If API returns empty or null data:

```typescript
computed(() => {
  const data = widgetData.value
  if (!data || (Array.isArray(data) && data.length === 0)) {
    return null  // Widget renders "No data" message
  }
  return data
})
```

**Widget Behavior**:
- LinePlotWidget: "No data" message, no chart
- CounterWidget: Displays "—"
- PopulationChartWidget: "No data" message

### Load Failures

```typescript
async function loadWidgetData(widget: WidgetConfig) {
  try {
    const data = await api.queryResults(simulationId, widget.query)
    widgetData.set(widget.id, data)
  } catch (error) {
    console.error(`Failed to load widget ${widget.id}:`, error)
    widgetData.set(widget.id, { error: error.message })
  }
}
```

Widget checks for error field in data → displays error message.

## Example Dashboard Layout

```json
{
  "widgets": [
    {
      "id": "w1",
      "type": "counter",
      "title": "Total Agents",
      "query": { "type": "summary", "property": "total_agents" },
      "position": { "x": 0, "y": 0, "w": 3, "h": 2 }
    },
    {
      "id": "w2",
      "type": "population",
      "title": "Population by Archetype",
      "query": { "type": "population" },
      "position": { "x": 0, "y": 2, "w": 12, "h": 4 }
    },
    {
      "id": "w3",
      "type": "line_plot",
      "title": "Average Energy Level",
      "query": { "type": "timeseries", "property_name": "avg_energy" },
      "position": { "x": 3, "y": 0, "w": 6, "h": 2 }
    }
  ]
}
```

Renders as:

```
┌────┬──────────────┬──────────────┐
│ w1 │     w3       │              │  Row 0-2
├────┴──────────────┴──────────────┤
│                                  │
│              w2                  │  Row 2-6
│                                  │
└──────────────────────────────────┘
```

## Creating a New Widget Type

### 1. Create Component

File: `src/shared/components/widgets/HistogramWidget.vue`

```typescript
<template>
  <WidgetBase :title="config.title" @configure="$emit('configure')" @remove="$emit('remove')">
    <Bar v-if="chartData" :data="chartData" :options="chartOptions" />
    <div v-else class="flex items-center justify-center h-full text-zinc-400 text-xs">
      No data
    </div>
  </WidgetBase>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Bar } from 'vue-chartjs'
import WidgetBase from '@/shared/components/WidgetBase.vue'
import type { WidgetConfig } from '@/shared/types'

const props = defineProps<{
  config: WidgetConfig
  data: unknown
}>()

defineEmits<{ configure: []; remove: [] }>()

const chartData = computed(() => {
  // Transform data → Chart.js histogram format
  return { ... }
})
</script>
```

### 2. Update Type

Edit `src/shared/types/widget.ts`:

```typescript
export type WidgetType =
  | 'line_plot'
  | 'counter'
  | 'population'
  | 'bar_chart'
  | 'histogram'         // ← Add type
  | 'agent_table'
```

### 3. Register in Dashboard

In `AnalysisView.vue`, add to widget factory:

```typescript
function createWidgetComponent(type: WidgetType) {
  const components = {
    'line_plot': () => import('...LinePlotWidget.vue'),
    'counter': () => import('...CounterWidget.vue'),
    'population': () => import('...PopulationChartWidget.vue'),
    'histogram': () => import('...HistogramWidget.vue'),  // ← Add factory
  }
  return components[type]
}
```

### 4. Add to Widget Library UI

Dashboard "Add Widget" button shows available types → component renders.

## Performance Considerations

### Large Datasets

For simulations with 10,000+ steps:

1. **Pagination**: Fetch summary → paginate on request
2. **Aggregation**: Backend pre-aggregates (e.g., every 10th step)
3. **Windowing**: Load only visible time range in chart
4. **Index**: Backend indexes by step for fast queries

### Many Widgets

If dashboard has 20+ widgets:

1. **Lazy Load**: Defer widget data fetch until visible
2. **Batch Requests**: Combine queries into single API call
3. **Worker Thread**: Offload data processing to Web Worker
4. **Cache**: Store results in localStorage (with TTL)

### Rendering

- Chart.js handles rendering optimization (canvas batching)
- grid-layout-plus handles layout calculations
- Vue reactivity batches updates

**Avoid**: Deep watches on large arrays, watch every property change
