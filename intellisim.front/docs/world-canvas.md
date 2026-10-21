# WorldCanvas Component Guide

**File**: `src/modules/simulation/components/WorldCanvas.vue`

A PixiJS-based 2D canvas component for rendering agent-based simulation worlds with real-time agent/shape visualization, navigation controls, and interactive tooltips.

## Overview

WorldCanvas transforms simulation data (agents, shapes) into an interactive 2D visualization:

- **Agents**: Circles positioned by x/y, colored by archetype_name property
- **Shapes**: Rectangles and circles representing environment obstacles/regions
- **Navigation**: Pan (click-drag), zoom (mouse wheel or +/- buttons), reset view
- **Interaction**: Hover agent to see tooltip with name and type
- **Legend**: Color legend mapping archetypes to hex colors
- **Performance**: WebGL rendering, efficient event delegation

## Props

```typescript
interface Props {
  agents: SimulationStepDocument[]      // Agent data (position, properties, metadata)
  shapes: SimulationEnvironmentShape[]  // Environment shapes (circles, rectangles)
  envWidth: number                      // World width (pixels)
  envHeight: number                     // World height (pixels)
  showLegend?: boolean                  // Display archetype legend (default: true)
}
```

### agents

Array of agent documents from `useWebSocketStream.latestAgents`:

```typescript
interface SimulationStepDocument {
  metadata: {
    name: string         // Agent display name (e.g., "Agent_1")
    type: 'agent'        // Document type
  }
  position?: {
    x: number            // X coordinate in world
    y: number            // Y coordinate in world
  }
  properties?: {
    archetype_name?: string  // Archetype (e.g., "Predator", "Prey") — used for coloring
    [key: string]: unknown   // Other properties
  }
}
```

**Filtering**: Agents without `position` are skipped.

### shapes

Array of environment shapes defining obstacles, regions, or landmarks:

```typescript
interface SimulationEnvironmentShape {
  visible: boolean       // Show/hide flag
  type: 'rectangle' | 'circle'
  x?: number            // Position X
  y?: number            // Position Y
  width?: number        // Rectangle width
  height?: number       // Rectangle height
  radius?: number       // Circle radius
  color?: string        // Hex color (default: '#cccccc')
}
```

**Filtering**: Shapes with `visible: false` are not rendered.

### envWidth, envHeight

Simulation world dimensions in pixels. Used for:
- Boundary box drawing
- View reset calculation
- Coordinate scaling

**Example**: 1000x1000 pixel world

### showLegend

Boolean (default: true) to display color legend in bottom-left corner.

## Data Flow

```
useWebSocketStream.latestAgents / latestShapes (reactive refs)
  ↓
WorldCanvas props updated
  ↓
watch([() => props.agents, () => props.shapes]) triggered
  ↓
renderAll() called → clear + redraw
  ↓
PixiJS re-renders on next frame
  ↓
Canvas displays updated world state
```

Real-time updates happen at ~30-60 fps depending on agent count.

## PixiJS Setup

### Initialization (initPixi)

Called on component mount:

```typescript
async function initPixi() {
  if (!canvasRef.value || !containerRef.value) return

  app = new Application()
  await app.init({
    canvas: canvasRef.value,
    width: containerRef.value.clientWidth,
    height: containerRef.value.clientHeight,
    backgroundColor: 0xfafafa,      // Zinc-50 white
    antialias: true,                // Smooth rendering
    resolution: window.devicePixelRatio || 1,  // Retina support
    autoDensity: true,
  })

  worldContainer = new Container()
  worldContainer.eventMode = 'static'
  app.stage.addChild(worldContainer)

  // Setup pan/zoom/click handlers...
  renderAll()
}
```

### App

PixiJS Application instance managing canvas lifecycle, rendering loop, and event handling.

### Canvas

HTML5 canvas element (refs to DOM canvas created by PixiJS).

### worldContainer

Container holding all world objects (boundary, shapes, agents). Pan/zoom transformations applied to this container.

## Rendering

### renderAll()

Main render function called on init and prop change:

```typescript
function renderAll() {
  if (!worldContainer) return
  worldContainer.removeChildren()

  // Draw boundary box
  const boundary = new Graphics()
  boundary.rect(0, 0, props.envWidth, props.envHeight)
  boundary.stroke({ color: 0xe4e4e7, width: 2 })
  worldContainer.addChild(boundary)

  renderShapes()
  renderAgents()
  resetView()
}
```

**Sequence**:
1. Clear container (remove all objects)
2. Draw boundary rectangle (zinc-200 outline)
3. Render shapes (circles, rectangles)
4. Render agents (colored circles)
5. Reset view to fit world

### renderShapes()

Renders each visible shape as PixiJS Graphics object:

```typescript
function renderShapes() {
  if (!worldContainer) return
  for (const shape of props.shapes) {
    if (!shape.visible) continue
    const g = new Graphics()
    const color = parseInt((shape.color || '#cccccc').replace('#', ''), 16)

    if (shape.type === 'rectangle' && shape.x != null && shape.y != null && ...) {
      g.rect(shape.x, shape.y, shape.width, shape.height)
      g.fill({ color, alpha: 0.3 })
      g.stroke({ color, width: 1 })
    } else if (shape.type === 'circle' && shape.x != null && shape.y != null && ...) {
      g.circle(shape.x, shape.y, shape.radius)
      g.fill({ color, alpha: 0.3 })
      g.stroke({ color, width: 1 })
    }

    worldContainer.addChild(g)
  }
}
```

**Style**:
- 30% opacity fill (alpha: 0.3)
- Solid color stroke (1px)
- Color from shape.color or fallback #cccccc

### renderAgents()

Renders each agent as a colored circle:

```typescript
function renderAgents() {
  if (!worldContainer) return
  for (const agent of props.agents) {
    if (!agent.position) continue
    const g = new Graphics()
    const typeName = (agent.properties?.archetype_name as string) || agent.metadata.name
    const colorHex = getArchetypeColor(typeName)
    const color = parseInt(colorHex.replace('#', ''), 16)

    const baseRadius = 4
    g.circle(0, 0, baseRadius)
    g.fill({ color })
    g.x = agent.position.x
    g.y = agent.position.y

    g.eventMode = 'static'
    g.cursor = 'pointer'
    g.on('pointerover', (e: FederatedPointerEvent) => {
      hoveredAgent.value = agent
      tooltipPos.value = { x: e.globalX + 12, y: e.globalY - 10 }
    })
    g.on('pointerout', () => {
      hoveredAgent.value = null
    })

    worldContainer.addChild(g)
  }
}
```

**Agent Styling**:
- Circle radius: 4 pixels (base)
- Color: From archetype_name → color palette
- Position: x, y from agent.position
- Interaction: Hover shows tooltip

## Archetype Coloring

```typescript
const PALETTE = [
  '#8B5CF6', '#3B82F6', '#22C55E', '#EF4444', '#F59E0B',
  '#EC4899', '#14B8A6', '#F97316', '#6366F1', '#84CC16',
]

function getArchetypeColor(typeName: string): string {
  if (!archetypeColors.value.has(typeName)) {
    const idx = archetypeColors.value.size % PALETTE.length
    archetypeColors.value.set(typeName, PALETTE[idx] ?? '#8B5CF6')
  }
  return archetypeColors.value.get(typeName)!
}
```

**Logic**:
- Map archetype name → color (stored in archetypeColors Map)
- First unique archetype gets first palette color
- Second unique archetype gets second color, etc.
- Colors cycle if more archetypes than palette length
- Default fallback: violet (#8B5CF6)

**Palette**:
1. Violet (#8B5CF6) — Primary
2. Blue (#3B82F6) — Info
3. Green (#22C55E) — Success
4. Red (#EF4444) — Error
5. Amber (#F59E0B) — Warning
6. Pink (#EC4899) — Accent
7. Teal (#14B8A6) — Accent
8. Orange (#F97316) — Accent
9. Indigo (#6366F1) — Accent
10. Lime (#84CC16) — Accent

## Navigation

### Pan (Click and Drag)

```typescript
app.stage.on('pointerdown', (e: FederatedPointerEvent) => {
  dragging = true
  dragStart = { x: e.globalX - worldContainer!.x, y: e.globalY - worldContainer!.y }
})
app.stage.on('pointermove', (e: FederatedPointerEvent) => {
  if (dragging && worldContainer) {
    worldContainer.x = e.globalX - dragStart.x
    worldContainer.y = e.globalY - dragStart.y
  }
})
app.stage.on('pointerup', () => { dragging = false })
app.stage.on('pointerupoutside', () => { dragging = false })
```

**Behavior**:
- Click and hold on canvas
- Move mouse → world pans
- Release → stop panning

### Zoom (Mouse Wheel)

```typescript
containerRef.value.addEventListener('wheel', handleWheel, { passive: false })

function handleWheel(e: WheelEvent) {
  e.preventDefault()
  if (!worldContainer) return
  const factor = e.deltaY > 0 ? 0.9 : 1.1
  worldContainer.scale.x *= factor
  worldContainer.scale.y *= factor
}
```

**Behavior**:
- Scroll up → zoom in (1.1x scale)
- Scroll down → zoom out (0.9x scale)
- preventDefault prevents page scroll
- Scale applied uniformly (aspect ratio preserved)

### Zoom Buttons

```typescript
function zoomIn() {
  if (!worldContainer) return
  worldContainer.scale.x *= 1.2
  worldContainer.scale.y *= 1.2
}

function zoomOut() {
  if (!worldContainer) return
  worldContainer.scale.x *= 0.8
  worldContainer.scale.y *= 0.8
}
```

**UI**: + and - buttons in top-right corner (fixed position overlay).

### Reset View

```typescript
function resetView() {
  if (!worldContainer || !containerRef.value) return
  const scaleX = containerRef.value.clientWidth / props.envWidth
  const scaleY = containerRef.value.clientHeight / props.envHeight
  const scale = Math.min(scaleX, scaleY) * 0.9
  worldContainer.scale.set(scale)
  worldContainer.x = (containerRef.value.clientWidth - props.envWidth * scale) / 2
  worldContainer.y = (containerRef.value.clientHeight - props.envHeight * scale) / 2
}
```

**Logic**:
- Calculate scale to fit world in container
- Apply 0.9 factor for padding (10% margin)
- Center world in container (offset by half of unused space)
- Uniform scaling preserves aspect ratio

**Button**: "Reset" button (expand icon) in top-right corner.

## Agent Interaction

### Hover Tooltip

```typescript
g.on('pointerover', (e: FederatedPointerEvent) => {
  hoveredAgent.value = agent
  tooltipPos.value = { x: e.globalX + 12, y: e.globalY - 10 }
})
g.on('pointerout', () => {
  hoveredAgent.value = null
})
```

**Template**:

```vue
<div v-if="hoveredAgent" class="absolute bg-zinc-900 text-white text-xs rounded-lg px-3 py-2 shadow-lg" :style="{ left: tooltipPos.x + 'px', top: tooltipPos.y + 'px' }">
  <p class="font-medium">{{ hoveredAgent.metadata.name }}</p>
  <p class="text-zinc-400">{{ hoveredAgent.metadata.type }}</p>
</div>
```

**Behavior**:
- Position tooltip at cursor + 12px right, 10px up
- Show agent name (from metadata)
- Show agent type ("agent")
- Tooltip disappears on pointerout

### Click Handling

Currently not implemented. Possible future features:
- Agent detail panel (properties, history)
- Agent selection highlighting
- Agent trajectory replay

## Color Legend

Displayed in bottom-left corner when `showLegend` prop is true:

```vue
<div v-if="showLegend && archetypeColors.size > 0" class="absolute bottom-3 left-3 bg-white/90 rounded-lg shadow border border-zinc-200 p-2 text-xs">
  <div v-for="[name, color] in archetypeColors" :key="name" class="flex items-center gap-2 mb-1 last:mb-0">
    <div :style="{ backgroundColor: color }" class="w-3 h-3 rounded-full" />
    <span class="text-zinc-600">{{ name }}</span>
  </div>
</div>
```

**Content**:
- One row per unique archetype
- Colored circle matching agent color
- Archetype name text
- Updated dynamically as new archetypes appear

## Performance Notes

### Rendering Efficiency

1. **Batch Updates**: All shapes/agents cleared and redrawn per step (not incremental)
   - Simpler logic vs. tracking individual updates
   - Acceptable for typical agent counts (<5000)
   - For very large simulations, consider incremental rendering

2. **WebGL Acceleration**: PixiJS uses WebGL (fallback: Canvas)
   - Hardware-accelerated rendering
   - Efficient for many graphics objects

3. **Event Delegation**: Agents have individual hover listeners
   - Per-agent overhead minimal (closure capture only)
   - Alternative: Single canvas listener with spatial lookup

4. **Viewport-Only Rendering**: No culling implemented
   - All agents rendered regardless of viewport
   - For zoomed-out view of 10,000+ agents, consider spatial partitioning

### Memory Management

- **Graphics Objects**: Created per render, destroyed on clear
- **Container**: Reused across renders
- **Maps**: archetypeColors, chunkBuffer held in memory
  - archetypeColors: One entry per unique archetype (typically <50)
  - Good memory profile

### Scaling Recommendations

| Agent Count | Status | Notes |
|-------------|--------|-------|
| <1,000 | Excellent | Smooth 60fps, no optimization needed |
| 1,000-5,000 | Good | Solid 30-60fps, visible rendering cost |
| 5,000-10,000 | Fair | 15-30fps, consider agent filtering or LOD |
| >10,000 | Poor | <15fps, implement spatial culling or batch rendering |

## Reactive Updates

Props watched for changes:

```typescript
watch([() => props.agents, () => props.shapes], () => {
  if (worldContainer) {
    worldContainer.removeChildren()
    const boundary = new Graphics()
    boundary.rect(0, 0, props.envWidth, props.envHeight)
    boundary.stroke({ color: 0xe4e4e7, width: 2 })
    worldContainer.addChild(boundary)
    renderShapes()
    renderAgents()
  }
}, { deep: true })
```

**Trigger**: Any change to agents or shapes array (deep watch).

**Potential Issue**: Deep watch can be expensive with large arrays. Alternative: Use key binding or computed + shallow watch.

## Component Lifecycle

```
onMounted
  ↓
initPixi() — Create Application, Container, render initial world
  ↓
addEventListener('wheel') — Pan/zoom handler
  ↓
Parent updates agents/shapes props
  ↓
watch triggered → renderAll()
  ↓
onUnmounted
  ↓
removeEventListener('wheel')
  ↓
app.destroy(true) — Clean up WebGL, event listeners
```

## Styling

The component uses:

- **Tailwind Classes**: Wrapper div with `w-full h-full relative bg-zinc-100`
- **PixiJS Graphics**: Stroke/fill for shapes, no CSS

**Boundary Color**: `0xe4e4e7` (zinc-200 hex)
**Background**: `0xfafafa` (zinc-50 hex)
**Canvas Classes**: None (PixiJS manages rendering)

## Accessibility

**Current State**: Limited (typical for canvas visualization)

**Considerations**:
- No keyboard navigation (pan/zoom)
- No ARIA labels on agents
- Tooltip text provides minimal context (name only)

**Future Improvements**:
- Keyboard shortcuts (arrow keys pan, +/- zoom)
- Agent detail panel for keyboard users
- Canvas title/description
- Tabindex for focus management

## Example: Full Simulation Visualization

```typescript
// In SimulationRunnerView.vue
import { useWebSocketStream } from '@/modules/simulation/composables/useWebSocketStream'

const props = defineProps<{ id: string }>()
const stream = useWebSocketStream(props.id)

onMounted(() => {
  stream.connect()
})

onUnmounted(() => {
  stream.disconnect()
})
```

```vue
<template>
  <div class="flex h-screen">
    <!-- Canvas on left -->
    <div class="flex-1">
      <WorldCanvas
        :agents="stream.latestAgents"
        :shapes="shapes"
        env-width="1000"
        env-height="1000"
        show-legend
      />
    </div>
    
    <!-- Controls on right -->
    <div class="w-64 bg-white p-4">
      <div>Step: {{ stream.currentStep }}</div>
      <div>Status: {{ stream.status }}</div>
      <div v-if="stream.progress">
        Progress: {{ stream.progress.percentage }}%
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useWebSocketStream } from '@/modules/simulation/composables/useWebSocketStream'
import { useSimulationApi } from '@/modules/simulation/composables/useSimulationApi'

const stream = useWebSocketStream(props.id)
const api = useSimulationApi()

const shapes = computed(() => {
  // Get shapes from simulation config or state
  return []
})
</script>
```

This example shows WorldCanvas integrated into a full simulation runner with real-time data streaming.
