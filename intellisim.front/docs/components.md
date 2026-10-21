# Component Catalog

This document catalogs all Vue 3 components in the IntelliSim frontend application, organized by module with props, emits, imports, and API usage.

## Layouts

### AppLayout
**File**: `src/layouts/AppLayout.vue`

**Purpose**: Root shell component providing 3-column layout with sidebar, main content area, and floating chat overlay.

**Props**: None

**Emits**: None

**Imports**:
- Sidebar (layout)
- FloatingChat (layout)

**API Endpoints**: None (layout-only, child components make API calls)

**Dependencies**:
- Vue Router (RouterView for main content)

**Structure**:
```
<div class="flex h-screen">
  <Sidebar />
  <main><RouterView /></main>
  <FloatingChat />
</div>
```

**Notes**:
- Always visible (not lazy-loaded)
- Parent of all routed views
- Background: zinc-50

---

### Sidebar
**File**: `src/layouts/Sidebar.vue`

**Purpose**: Navigation sidebar with collapsible state, model/chat/simulation lists, action buttons, and LLM provider selector.

**Props**: None

**Emits**: None (uses router.push for navigation, Pinia stores for state)

**Imports**:
- useSidebarStore (Pinia)
- useLlmProviderStore (Pinia)

**API Endpoints**:
- `useLlmProviderStore.loadProviders()` — Fetches available LLM providers on mount

**Dependencies**:
- Vue Router (router.push, router-link)
- Pinia (sidebarStore, llmProviderStore)
- PrimeIcons (pi-plus, pi-comment, pi-box, pi-play, pi-comments, etc.)

**Key Features**:
- Collapsible (w-64 → w-16) with smooth transitions
- Models and Simulations navigation links
- General chats list
- Action buttons: "New Model" (violet), "New General Chat" (outline)
- Active item highlighting (violet background)

**State Management**:
- `sidebarStore.collapsed` — Expand/collapse toggle
- Active item highlight derived from the current route (`$route`)
- `llmProviderStore.selectedProviderId` — Selected LLM provider

---

### FloatingChat
**File**: `src/layouts/FloatingChat.vue`

**Purpose**: Floating chat button and panel, accessible from any page for quick questions.

**Props**: None

**Emits**: None

**Imports**:
- ChatPanel (shared component)
- useFloatingChatStore (Pinia)
- useLlmProviderStore (Pinia)
- apiClient (Axios)

**API Endpoints**:
- `POST /chats` — Create new floating chat
- `POST /chats/:id/messages` — Add message to existing floating chat

**Dependencies**:
- PrimeVue ProgressSpinner (loading indicator)
- ChatPanel component

**Key Features**:
- Fixed position button (bottom-right, z-50)
- Toggles panel visibility
- Maintains separate chat session (chatId)
- Passes llmProviderId to API calls
- Auto-scrolls to latest message

**State**:
- `messages: ChatMessageResponse[]` — Chat history (local)
- `loading: boolean` — API call in progress
- `floatingStore.isOpen` — Panel visible
- `floatingStore.chatId` — Associated chat document

---

## Chat Module

### ChatView
**File**: `src/modules/chat/views/ChatView.vue`

**Purpose**: General-purpose LLM chat interface for creating and continuing chats.

**Props**:
- `id?: string` — Existing chat ID (from route params)

**Emits**: None

**Imports**:
- useChatApi (composable)
- ChatPanel (shared)
- useRoute (Vue Router)

**API Endpoints** (via useChatApi):
- `POST /chats` — Create new chat
- `GET /chats/:id` — Load existing chat with pagination
- `POST /chats/:id/messages` — Add user message and get LLM response

**Dependencies**:
- Vue Router (useRoute, route.params.id)
- ChatPanel component

**Key Features**:
- Route ID optional (new vs existing chat)
- Loads chat history on mount if ID provided
- Pagination support (page, pageSize)
- Sends message with optional llmProviderId
- Message list reactive updates

---

## Model Module

### ModelChatView
**File**: `src/modules/model/views/ModelChatView.vue`

**Purpose**: Conversational model design interface where LLM helps create/refine agent-based models.

**Props**:
- `id?: string` — Existing model chat ID

**Emits**: None

**Imports**:
- useModelChatApi (composable)
- ModelPreview (component)
- ChatPanel (shared)

**API Endpoints** (via useModelChatApi):
- `POST /modelchats` — Create new model chat
- `GET /modelchats/:id` — Load chat history
- `POST /modelchats/:id/messages` — Add message (LLM generates/refines model)
- `GET /modelchats/:id/model` — Fetch current model definition (ModelGenerationResponse)

**Dependencies**:
- ModelPreview component (displays JSON)
- ChatPanel component

**Key Features**:
- Split panel: Chat on left, JSON preview on right
- Real-time model generation during conversation
- Displays agent archetypes, environment, parameters
- "Go to Editor" button to switch to manual editing

---

### ModelEditorView
**File**: `src/modules/model/views/ModelEditorView.vue`

**Purpose**: Manual JSON editor for fine-tuning model definitions created via chat or from scratch.

**Props**:
- `id: string` — Model chat ID (required for loading/saving)

**Emits**: None

**Imports**:
- useModelChatApi (composable)
- JsonPanel (component)

**API Endpoints** (via useModelChatApi):
- `GET /modelchats/:id/model` — Load model on mount
- `PATCH /modelchats/:id/model` — Save manual edits (ManualUpdateModelRequest)

**Dependencies**:
- JsonPanel component (editable code view)

**Key Features**:
- Syntax validation on save
- Error/warning display
- Saves back to model document
- JSON structure with agents, environment, parameters, initial_state

---

### ModelPreview
**File**: `src/modules/model/components/ModelPreview.vue`

**Purpose**: Read-only JSON panel displaying the current model definition.

**Props**:
- `model: ModelGenerationResponse` — Model data to display

**Emits**: None

**Imports**:
- JsonPanel (component)

**API Endpoints**: None (display-only)

**Dependencies**:
- JsonPanel component

**Key Features**:
- Expandable/collapsible JSON tree
- Syntax highlighting
- Scroll container for large models

---

### JsonPanel
**File**: `src/modules/model/components/JsonPanel.vue`

**Purpose**: Reusable JSON viewer/editor with expandable tree structure.

**Props**:
- `value: unknown` — Data to display
- `editable?: boolean` — Allow editing (default: false)

**Emits**:
- `update:value` — When editable and content changes

**Imports**: None (vanilla Vue)

**API Endpoints**: None

**Dependencies**: None

**Key Features**:
- Syntax highlighting
- Copy button for snippets
- Expand/collapse nodes
- Search/filter (optional)
- Line numbers

---

### AgentForm
**File**: `src/modules/model/components/AgentForm.vue`

**Purpose**: Form builder for creating/editing agent archetypes in models.

**Props**:
- `archetype: AgentArchetype` — Agent definition
- `modelId: string` — Associated model

**Emits**:
- `save` — User submits form
- `cancel` — User cancels

**Imports**: None

**API Endpoints**: None (form-only, parent handles API)

**Dependencies**:
- PrimeVue form components (InputText, InputNumber, Select, etc.)

**Key Features**:
- Dynamic property inputs
- Archetype metadata (name, description, color)
- Initial state configuration
- Validation

---

## Simulation Module

### SimulationChatView
**File**: `src/modules/simulation/views/SimulationChatView.vue`

**Purpose**: Conversational simulation configuration interface where LLM helps configure parameters and environment.

**Props**:
- `id: string` — Model ID for creating simulation

**Emits**: None

**Imports**:
- useSimulationApi (composable)
- SimulationConfigPreview (component)
- ChatPanel (shared)

**API Endpoints** (via useSimulationApi):
- `POST /simulations?modelId=:modelId` — Initiate simulation
- `GET /simulations/:id` — Load simulation config
- `POST /simulations/:id/messages` — Add config message (LLM refines)

**Dependencies**:
- SimulationConfigPreview component
- ChatPanel component

**Key Features**:
- Chat-driven parameter refinement
- Real-time config preview
- "Start Simulation" button navigates to runner

---

### SimulationRunnerView
**File**: `src/modules/simulation/views/SimulationRunnerView.vue`

**Purpose**: Active simulation execution display with real-time world visualization, controls, and metrics.

**Props**:
- `id: string` — Simulation ID

**Emits**: None

**Imports**:
- useSimulationApi (composable)
- useWebSocketStream (composable)
- WorldCanvas (component)
- SimulationControls (component)

**API Endpoints** (via useSimulationApi):
- `POST /simulations/:id/start` — Start execution
- `POST /simulations/:id/stop` — Stop execution
- `GET /simulations/:id/status` — Poll status

**WebSocket** (via useWebSocketStream):
- Stream URL: `/simulations/:id/stream`
- Messages: step_update, progress, status_change, completion, error, llm_usage

**Dependencies**:
- WorldCanvas component (PixiJS agent rendering)
- SimulationControls component (play/pause/stop)
- ChartJS for progress/metrics visualization

**Key Features**:
- Real-time agent rendering
- Current step counter
- Progress bar
- Status indicator (initiated, running, finished, error)
- Console log output (errors, status changes)
- LLM usage tracking

---

### WorldCanvas
**File**: `src/modules/simulation/components/WorldCanvas.vue`

**Purpose**: PixiJS-based 2D world visualization with agent rendering, shape rendering, panning, zooming, and hover tooltips.

**Props**:
- `agents: SimulationStepDocument[]` — Agent data to render
- `shapes: SimulationEnvironmentShape[]` — Environment shapes (circles, rectangles)
- `envWidth: number` — World width
- `envHeight: number` — World height
- `showLegend?: boolean` — Display archetype color legend

**Emits**: None

**Imports**:
- PixiJS (Application, Graphics, Container)

**API Endpoints**: None (data-driven, no API calls)

**Dependencies**:
- PixiJS 8 (canvas rendering)

**Key Features**:
- WebGL rendering (efficient for many agents)
- Agent circles colored by archetype_name property
- Shape rendering (rectangle, circle with alpha fill)
- Pan: Click and drag to move world
- Zoom: Mouse wheel or +/- buttons
- Reset view: Fit to screen
- Hover: Show agent name/type tooltip
- Legend: Archetype colors with names
- World boundary box (2px zinc border)

**Performance Notes**:
- Re-renders on prop change (agents/shapes watch)
- No per-agent event listeners (delegate via container)
- Resolution scaled by devicePixelRatio
- Antialias enabled for smooth rendering

---

### SimulationControls
**File**: `src/modules/simulation/components/SimulationControls.vue`

**Purpose**: Playback controls (play, pause, stop) and speed adjustment for simulation execution.

**Props**:
- `status: SimulationStatus` — Current simulation state
- `step: number` — Current execution step
- `progress?: ProgressInfo` — Progress percentage

**Emits**:
- `start` — User clicks play
- `stop` — User clicks stop
- `pause` — User clicks pause (if implemented)

**Imports**: None

**API Endpoints**: None (emits for parent to handle)

**Dependencies**:
- PrimeVue Button component

**Key Features**:
- Play button (enabled only if status not running)
- Pause button (enabled if running)
- Stop button
- Step counter display
- Progress percentage
- Status indicator badge

---

### SimulationConfigPreview
**File**: `src/modules/simulation/components/SimulationConfigPreview.vue`

**Purpose**: Display-only panel showing simulation configuration (parameters, environment, initial state).

**Props**:
- `config: SimulationDocument` — Simulation definition

**Emits**: None

**Imports**:
- JsonPanel (component)

**API Endpoints**: None (display-only)

**Dependencies**:
- JsonPanel component

**Key Features**:
- Organized sections: Parameters, Environment, Initial State
- Collapsible sections
- JSON syntax highlighting

---

## Analysis Module

### AnalysisView
**File**: `src/modules/analysis/views/AnalysisView.vue`

**Purpose**: Post-simulation data exploration with interactive widgets (charts, counters), drag-and-drop dashboard, and PDF export.

**Props**:
- `id: string` — Simulation ID

**Emits**: None

**Imports**:
- useSimulationApi (composable)
- WidgetBase (shared)
- LinePlotWidget, CounterWidget, PopulationChartWidget (widgets)
- grid-layout-plus (layout engine)

**API Endpoints** (via useSimulationApi):
- `GET /simulations/:id/results/summary` — Initial results overview
- `GET /simulations/:id/results/timeseries/:property` — Time-series data for property
- `GET /simulations/:id/results/spatial/:step` — Spatial agent data at step
- `GET /simulations/:id/results/population` — Population by archetype over time
- `GET /simulations/:id/results/properties` — Available result properties
- `POST /simulations/:id/analyze` — Generate analysis text (AnalysisRequest)
- `POST /simulations/:id/analyze/pdf` — Generate PDF report

**Dependencies**:
- grid-layout-plus (draggable/resizable grid)
- Widget system (WidgetBase + implementations)
- ChartJS for rendering

**Key Features**:
- Widget dashboard (6 types defined, 3 implemented)
- Drag to reorder, resize to expand
- Configure widget (change property/query)
- Remove widget from layout
- Add/preset buttons
- LLM analysis generation
- PDF export
- Real-time and cached data modes

---

## Shared Components

### ChatPanel
**File**: `src/shared/components/ChatPanel.vue`

**Purpose**: Reusable chat container with message list and text input, used by ChatView and FloatingChat.

**Props**:
- `messages: ChatMessageResponse[]` — Message history
- `loading?: boolean` — API call in progress (default: false)
- `placeholder?: string` — Input placeholder text (default: "Type a message...")

**Emits**:
- `send` — User submits message (payload: string text)

**Imports**:
- MessageBubble (component)
- PrimeVue InputText, Button, ProgressSpinner

**API Endpoints**: None (emit-based, parent handles API)

**Dependencies**:
- MessageBubble component
- @vueuse/core (nextTick for scroll behavior)

**Key Features**:
- Auto-scroll to bottom on new message
- Empty state slot
- Loading spinner while waiting
- Disabled input during API call
- Enter key to send (or click button)
- Customizable input prefix/footer slots

---

### MessageBubble
**File**: `src/shared/components/MessageBubble.vue`

**Purpose**: Individual chat message display with type-specific styling (user, assistant, system).

**Props**:
- `content: string` — Message text (supports markdown)
- `type: 'user' | 'assistant' | 'system'` — Message sender type
- `createdAt: string` — ISO timestamp

**Emits**: None

**Imports**:
- markdown-it (for rendering LLM responses)

**API Endpoints**: None

**Dependencies**:
- markdown-it library

**Key Features**:
- Different styling per type (user: blue/right, assistant: white/left, system: yellow/center)
- Markdown rendering (code blocks, lists, emphasis)
- Timestamp display (formatted relative time)
- Copyable code blocks (optional button)
- XSS-safe rendering (v-html with markdown-it sanitization)

---

### WidgetBase
**File**: `src/shared/components/WidgetBase.vue`

**Purpose**: Container frame for all dashboard widgets with title, configure, and remove controls.

**Props**:
- `title: string` — Widget title

**Emits**:
- `configure` — User clicks gear icon
- `remove` — User clicks X icon

**Imports**: None

**API Endpoints**: None

**Dependencies**: PrimeIcons (pi-cog, pi-times)

**Key Features**:
- White background, border, rounded corners
- Header with title and controls
- Content slot for child widget
- Compact header (12px spacing)

---

### LinePlotWidget
**File**: `src/shared/components/widgets/LinePlotWidget.vue`

**Purpose**: Time-series line chart widget for displaying property values over simulation steps.

**Props**:
- `config: WidgetConfig` — Widget configuration (title, query, position)
- `data: TimeSeriesPoint[] | unknown` — Time-series data points

**Emits**:
- `configure` — User clicks configure
- `remove` — User clicks remove

**Imports**:
- WidgetBase (component)
- vue-chartjs Line chart
- Chart.js scales, plugins

**API Endpoints**: None (data passed as prop)

**Dependencies**:
- Chart.js (CategoryScale, LinearScale, PointElement, LineElement)
- vue-chartjs

**Key Features**:
- Violet line color (#8B5CF6) with light fill
- X-axis: step numbers
- Y-axis: property values (auto-scaled)
- No data fallback message
- Responsive to container size
- Tooltip on hover
- Legend hidden (single series)

---

### CounterWidget
**File**: `src/shared/components/widgets/CounterWidget.vue`

**Purpose**: Large-number display widget for single metrics (total agents, generations, etc.).

**Props**:
- `config: WidgetConfig` — Widget configuration
- `data: unknown` — Single numeric or string value

**Emits**:
- `configure`
- `remove`

**Imports**:
- WidgetBase (component)

**API Endpoints**: None

**Dependencies**: None

**Key Features**:
- Centered, large bold text (text-3xl)
- Numeric formatting (round to 2 decimals)
- Null/undefined fallback: "—"
- Zinc-800 text color

---

### PopulationChartWidget
**File**: `src/shared/components/widgets/PopulationChartWidget.vue`

**Purpose**: Multi-line chart showing population of each agent archetype over time.

**Props**:
- `config: WidgetConfig` — Widget configuration
- `data: PopulationPoint[] | unknown` — Array of PopulationPoint objects

**Emits**:
- `configure`
- `remove`

**Imports**:
- WidgetBase
- vue-chartjs Line chart
- Chart.js

**API Endpoints**: None

**Dependencies**:
- Chart.js, vue-chartjs

**Key Features**:
- Multi-series (one per archetype)
- 7-color palette (rotates for many archetypes)
- X-axis: steps
- Y-axis: agent count
- Legend at bottom with archetype names
- Auto-detects unique archetypes from data
- Smooth curves (tension: 0.3)

---

## Widget Placeholder Components (Not Yet Implemented)

The following widget types are defined in `widget.ts` but components not yet created:

1. **bar_chart** - Bar chart for categorical data
2. **histogram** - Distribution histogram for property values
3. **agent_table** - Sortable/filterable table of agents at current step

These can be added following the pattern of LinePlotWidget:
- Extend WidgetBase
- Accept WidgetConfig + data props
- Emit configure/remove
- Use Chart.js or simple DOM rendering

---

## Component Import Map

For quick reference, import paths for all components:

```typescript
// Layouts
import AppLayout from '@/layouts/AppLayout.vue'
import Sidebar from '@/layouts/Sidebar.vue'
import FloatingChat from '@/layouts/FloatingChat.vue'

// Chat
import ChatView from '@/modules/chat/views/ChatView.vue'

// Model
import ModelChatView from '@/modules/model/views/ModelChatView.vue'
import ModelEditorView from '@/modules/model/views/ModelEditorView.vue'
import ModelPreview from '@/modules/model/components/ModelPreview.vue'
import AgentForm from '@/modules/model/components/AgentForm.vue'
import JsonPanel from '@/modules/model/components/JsonPanel.vue'

// Simulation
import SimulationChatView from '@/modules/simulation/views/SimulationChatView.vue'
import SimulationRunnerView from '@/modules/simulation/views/SimulationRunnerView.vue'
import WorldCanvas from '@/modules/simulation/components/WorldCanvas.vue'
import SimulationControls from '@/modules/simulation/components/SimulationControls.vue'
import SimulationConfigPreview from '@/modules/simulation/components/SimulationConfigPreview.vue'

// Analysis
import AnalysisView from '@/modules/analysis/views/AnalysisView.vue'

// Shared
import ChatPanel from '@/shared/components/ChatPanel.vue'
import MessageBubble from '@/shared/components/MessageBubble.vue'
import WidgetBase from '@/shared/components/WidgetBase.vue'
import LinePlotWidget from '@/shared/components/widgets/LinePlotWidget.vue'
import CounterWidget from '@/shared/components/widgets/CounterWidget.vue'
import PopulationChartWidget from '@/shared/components/widgets/PopulationChartWidget.vue'
```
