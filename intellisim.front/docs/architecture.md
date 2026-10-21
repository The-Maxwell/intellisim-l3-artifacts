# Architecture Guide

## Overview

IntelliSim Frontend is a Vue 3 + TypeScript web application providing an LLM-powered interface for agent-based modeling (ABM) simulations. The architecture follows a modular, component-driven design with clear separation of concerns:

- **Layouts** - Shell structure (AppLayout, Sidebar, FloatingChat)
- **Modules** - Feature-based organization (Chat, Model, Simulation, Analysis)
- **Shared** - Reusable API clients, composables, components, stores, types, and theming
- **Routing** - Lazy-loaded routes with parameter passing
- **State Management** - Pinia stores for sidebar, LLM providers, and floating chat
- **Real-time Communication** - WebSocket for simulation streaming with chunked buffering
- **Visualization** - PixiJS 2D canvas for agent rendering with interactive controls

## Tech Stack

| Package | Version | Purpose |
|---------|---------|---------|
| Vue | 3.5.24 | Reactive framework with Composition API |
| TypeScript | ~5.9.3 | Static type safety (strict mode) |
| Vue Router | 4.6.4 | Client-side routing with lazy loading |
| Pinia | 3.0.4 | State management (sidebar, LLM providers, floating chat) |
| PrimeVue | 4.4.1 | UI component library (Aura preset, Zinc+Violet theme) |
| Tailwind CSS | 4.1.17 | Utility-first styling via @tailwindcss/vite |
| @tailwindcss/vite | 4.1.17 | Tailwind CSS build integration |
| Axios | 1.14.0 | HTTP client (baseURL: `/api/v1/intellisim`, timeout: 60s) |
| PixiJS | 8.17.1 | 2D WebGL canvas rendering (agents, shapes, navigation) |
| Chart.js | 4.5.1 | Chart rendering library |
| vue-chartjs | 5.3.3 | Vue 3 wrapper for Chart.js |
| markdown-it | 14.1.1 | Parse LLM responses from markdown to HTML |
| splitpanes | 4.0.4 | Resizable split panels for layouts |
| grid-layout-plus | 1.1.1 | Draggable/resizable widget grid |
| @vueuse/core | 14.2.1 | Utility composables (useDebounceFn, useResizeObserver) |
| @primeuix/themes | 1.2.5 | PrimeVue theme system |
| primeicons | 7.0.0 | Icon library |
| tailwind-merge | 3.4.0 | Merge Tailwind classes without conflicts |
| tailwindcss-primeui | 0.6.1 | Tailwind integration for PrimeVue |
| Vite | 7.2.2 | Build tool (port 3000, dev proxy to :8080) |
| Vue TSC | 3.1.3 | TypeScript compilation for Vue |
| Playwright | 1.59.1 | E2E testing |

## Project Structure

```
src/
├── app/
│   ├── router.ts                    # Route definitions (lazy-loaded modules)
│   └── main.ts                      # Application bootstrap
│
├── App.vue                           # Root component
│
├── layouts/
│   ├── AppLayout.vue               # Shell: Sidebar + RouterView + FloatingChat
│   ├── Sidebar.vue                 # Navigation, model/chat/simulation lists, LLM provider selector
│   └── FloatingChat.vue            # Floating chat button and panel (always visible)
│
├── modules/
│   ├── auth/
│   │   ├── views/
│   │   │   ├── LoginView.vue       # Login landing (public route)
│   │   │   └── AuthCallbackView.vue # OIDC code-flow callback handler
│   │   ├── stores/
│   │   │   └── authStore.ts        # Pinia store: user, me, init/login/renew/logout
│   │   └── authService.ts          # oidc-client-ts UserManager (PKCE, localStorage)
│   │
│   ├── chat/
│   │   ├── views/
│   │   │   └── ChatView.vue        # General chat interface (lazy-loaded)
│   │   └── composables/
│   │       ├── useChatApi.ts       # Chat REST: create, get, list, send, update, delete
│   │       ├── useChatSession.ts   # Shared chat state owner: load, SSE, pagination, send
│   │       └── useChatEvents.ts    # SSE subscriber (fetch-event-source, named events)
│   │
│   ├── model/
│   │   ├── views/
│   │   │   ├── ModelChatView.vue   # Model creation chat (lazy-loaded)
│   │   │   └── ModelEditorView.vue # Manual model JSON editor (lazy-loaded)
│   │   ├── components/
│   │   │   ├── ModelPreview.vue    # JSON preview panel
│   │   │   ├── AgentForm.vue       # Agent archetype form builder
│   │   │   └── JsonPanel.vue       # Expandable JSON viewer
│   │   └── composables/
│   │       └── useModelChatApi.ts  # Model chat + manual edit + image environment generation
│   │
│   ├── simulation/
│   │   ├── views/
│   │   │   ├── SimulationChatView.vue    # Simulation config chat (lazy-loaded)
│   │   │   └── SimulationRunnerView.vue  # Active simulation runner (lazy-loaded)
│   │   ├── components/
│   │   │   ├── WorldCanvas.vue            # PixiJS 2D world with agents/shapes
│   │   │   ├── SimulationControls.vue     # Play/pause/stop buttons
│   │   │   └── SimulationConfigPreview.vue # Config display
│   │   └── composables/
│   │       ├── useSimulationApi.ts        # All simulation REST endpoints
│   │       └── useWebSocketStream.ts      # Real-time step updates with chunk buffering
│   │
│   └── analysis/
│       └── views/
│           └── AnalysisView.vue          # Post-simulation analysis + PDF export (lazy-loaded)
│
├── shared/
│   ├── api/
│   │   ├── client.ts                     # Axios instance (baseURL, timeout, error interceptor)
│   │   └── llmProviders.ts               # LLM provider API endpoints
│   │
│   ├── components/
│   │   ├── MessageBubble.vue             # Chat message UI (user/system/assistant)
│   │   ├── ChatPanel.vue                 # Reusable chat container with input
│   │   ├── WidgetBase.vue                # Widget container (title, configure, remove buttons)
│   │   └── widgets/
│   │       ├── LinePlotWidget.vue        # Time-series line chart (via Chart.js)
│   │       ├── CounterWidget.vue         # Single value display
│   │       └── PopulationChartWidget.vue # Multi-archetype population chart
│   │
│   ├── composables/
│   │   ├── useWebSocket.ts               # Generic WebSocket with reconnection + exponential backoff
│   │   └── useFloatingChat.ts            # Floating chat state logic
│   │
│   ├── stores/
│   │   ├── sidebarStore.ts               # Sidebar state: collapsed + general chats list
│   │   ├── llmProviderStore.ts           # LLM provider selection and loading
│   │   └── floatingChatStore.ts          # Floating chat open/close state
│   │
│   ├── types/
│   │   ├── chat.ts                       # Chat DTOs (ChatResponse, ChatMessageResponse, etc.)
│   │   ├── model.ts                      # Model DTOs (Agent archetype, environment, etc.)
│   │   ├── simulation.ts                 # Simulation DTOs (status, step, results, etc.)
│   │   ├── llm.ts                        # LLM provider DTOs
│   │   ├── widget.ts                     # Widget config and presets
│   │   └── index.ts                      # Re-exports all types
│   │
│   ├── validation/
│   │   └── chatRules.ts                  # UI-side validation constants (CHAT_NAME_MAX, MESSAGE_CONTENT_MAX, etc.)
│   │
│   └── llm/
│       └── defaultTarget.ts              # localStorage persistence of the last-used LLM target
│   │
│   └── theme/
│       ├── primevue-preset.ts            # PrimeVue Aura preset (Zinc+Violet palette)
│       └── colors.ts                     # Semantic color tokens (accent, error, success, etc.)
│
├── main.ts                              # Entry point (mount app, setup PrimeVue, Tailwind)
├── index.css                            # Global styles
└── env.d.ts                             # TypeScript environment declarations
```

## Module Pattern

### Auth Module
- **Purpose**: OIDC/PKCE authentication, token lifecycle, and identity
- **Views**: LoginView (public), AuthCallbackView (public)
- **Store**: `authStore` — single source of truth for authentication state; exposes `user`, `me`, `isAuthenticated`, `isAdmin`, `getValidAccessToken()`, `renew()`, `login()`, `logout()`
- **Service**: `authService.ts` — `UserManager` singleton (oidc-client-ts); PKCE, localStorage store, `automaticSilentRenew`
- **Integration**: Axios interceptor calls `getValidAccessToken()` on every request; SSE per-attempt fetch injects the current token; router guard gates all non-public routes

### Chat Module
- **Purpose**: General-purpose LLM chat interface
- **Views**: ChatView (new and existing chats; shared `useChatSession` instance)
- **API**: `useChatApi` — REST calls: `createChat`, `getChat`, `listChats`, `listMessages`, `sendMessage`, `updateChat`, `deleteChat`
- **Session**: `useChatSession` — the single chat state owner used by both `ChatView` and `FloatingChat`. Manages the loaded chat, the message list (chronological, deduplicated by ID), the thinking flag, the `hasMore` flag, and the SSE subscription lifetime. `open(id)` loads the chat, subscribes to events, and calls `refresh()` on reconnect.
- **Real-time**: `useChatEvents` (`subscribeChatEvents`) — fetch-event-source based SSE subscriber. Handles named events (`message.created`, `chat.status`, `chat.updated`, `chat.deleted`, `chat.error`). Per-attempt `fetch` override injects the current bearer token. Retries every 3 seconds on disconnect, forever. No replay: the `onReconnect` callback triggers a full `refresh()`.
- **Infinite scroll**: Cursor-based (`before=<oldest-message-id>`). `loadOlder()` prepends pages; the `ChatPanel` component preserves scroll position during prepend. Autoscroll to bottom fires only on message append (new messages), not on prepend (older history).
- **Model picker**: Per-chat LLM target selection in the chat header (chat-capable models only, filtered by `chat` capability). Unavailable model banner (`model_available: false`) blocks the text input until the user selects a working target.
- **Validation constants**: `shared/validation/chatRules.ts` mirrors backend limits (`MESSAGE_CONTENT_MAX = 10000`, `MESSAGE_COUNTER_THRESHOLD = 9000`, `CHAT_NAME_MAX = 100`, `CHAT_DESCRIPTION_MAX = 500`). The character counter appears once the input exceeds 9000 characters.

### Model Module
- **Purpose**: Conversational model design or manual JSON editing
- **Views**: ModelChatView (LLM-guided), ModelEditorView (manual JSON)
- **Components**: ModelPreview (JSON panel), AgentForm (archetype builder), JsonPanel (expandable)
- **API**: useModelChatApi - `/modelchats`, `/modelchats/:id/model`, `/modelchats/:id/environment/from-image`
- **State**: Model definition cached in component (fetch on route change)

### Simulation Module
- **Purpose**: Configure, execute, and visualize agent-based simulations in real-time
- **Views**: SimulationChatView (config), SimulationRunnerView (active execution)
- **Components**: WorldCanvas (PixiJS), SimulationControls (play/pause/stop), SimulationConfigPreview
- **API**: useSimulationApi - 20+ endpoints (initiate, start, stop, query results, analyze, etc.)
- **WebSocket**: useWebSocketStream - Real-time steps with chunked buffering (1000 docs/chunk)
- **State**: Current step, status, progress, agents, shapes, LLM usage, errors, console logs

### Analysis Module
- **Purpose**: Post-simulation data exploration and PDF report generation
- **Views**: AnalysisView (widgets, charting, export)
- **Components**: Widget system (LinePlot, Counter, Population chart, plus 3 placeholders)
- **API**: useSimulationApi results endpoints (timeseries, spatial, population, analyze, analyzePdf)
- **State**: Widget grid layout, result caching

## Data Flow

```
Component (e.g., ChatView.vue)
  ↓
  useComposable (e.g., useChatApi.ts)
    ↓
    apiClient (Axios instance)
      ↓
      Backend REST API (/api/v1/intellisim/...)
        ↓
        Response → API Composable → Component State/Ref
          ↓
          Template (reactive rendering)
```

### Example: Chat Flow
1. User types a message in `ChatPanel`; send button is disabled if content is empty, whitespace-only, or exceeds 10 000 characters
2. `ChatPanel` emits `send` event to the parent view
3. View delegates to `useChatSession.send(content)`
4. `send()` calls `useChatApi.sendMessage(chatId, content)` → POST `/chats/:id/messages`
5. Backend persists the user message and returns 202 with `{message, chat_status: "thinking"}`
6. `useChatSession` inserts the user message via `upsertMessage` (deduplicates by ID) and sets `thinking = true`
7. Backend SSE delivers `message.created` (assistant message) and `chat.status {status: "idle"}` when generation completes
8. `useChatEvents.onMessage` calls `upsertMessage` again → Vue re-renders; `onStatus` clears the thinking flag

### Example: Simulation Real-time Flow
1. User clicks "Start" in SimulationRunnerView
2. API call: `start(id)` → POST `/simulations/:id/start`
3. `useWebSocketStream.connect()` opens WebSocket to `/simulations/:id/stream`
4. Backend streams `StepUpdateMessage` events (with chunk tracking)
5. Handler buffers steps → groups agents/shapes → updates reactive state
6. WorldCanvas watches latestAgents/latestShapes → re-renders PixiJS

## Layout System

### AppLayout (Shell)
- **Structure**: 3-column layout
  - Left: Sidebar (collapsible, 16px or 64px width)
  - Center: Main content (RouterView)
  - Right: FloatingChat (fixed overlay)
- **Classes**: `flex h-screen overflow-hidden bg-zinc-50`
- **Purpose**: Root layout for all pages

### Sidebar
- **Collapsible**: Toggles between expanded (w-64) and collapsed (w-16)
- **Sections**:
  1. Header (logo + title + toggle button)
  2. Action buttons (New Model, New General Chat)
  3. Navigation (Models, Simulations, General Chats)
- **Styling**: Dark background (zinc-900), white text, hover states
- **Persistence**: sidebarStore (Pinia) tracks collapsed state

### FloatingChat
- **Fixed**: `fixed bottom-6 right-6 z-50`
- **Button**: Violet circle (w-14 h-14), toggles chat panel
- **Panel**: 96 width, 500px height, white background, minimal header
- **Behavior**: Independent of main navigation, always accessible
- **API**: Direct Axios calls (not via composable), maintains separate chat session

## Routing

All routes are **lazy-loaded** via dynamic imports. Child routes inherit AppLayout. A global `beforeEach` guard protects every route that is not marked `meta.public`.

```
/login                                 → LoginView (public)
/auth/callback                         → AuthCallbackView (public)
/                                      → Root (redirects to /chat/new; protected)
├── /chat/new                          → ChatView (new chat)
├── /chat/:id                          → ChatView (existing chat)
├── /models/new                        → ModelChatView (new model chat)
├── /models/:id                        → ModelChatView (existing model chat)
├── /models/:id/editor                 → ModelEditorView (manual JSON editing)
├── /simulations/:id                   → SimulationChatView (config)
├── /simulations/:id/run               → SimulationRunnerView (execution)
└── /simulations/:id/analysis          → AnalysisView (results)
```

**Route Parameters**: Passed as props to views (e.g., `id` is auto-injected).

**Guard behaviour**: On a cold visit (no stored session) the guard redirects straight to `/login`. On a warm visit (stored session, token possibly expired) it attempts a single silent renew before redirecting. See [Frontend Auth Reference](auth.md) for full detail.

## State Management (Pinia)

### authStore
- `user: User | null` — oidc-client-ts session object (from localStorage)
- `me: MeResponse | null` — backend profile from `GET /auth/me`
- `isAuthenticated: boolean` — computed: `user != null && !user.expired`
- `isAdmin: boolean` — computed: `me?.is_admin ?? false`
- `displayName / email` — computed display strings from `me` or OIDC profile
- **Methods**: `init()`, `login(redirectPath?)`, `handleCallback()`, `renew()` (single-flight), `getValidAccessToken()` (proactive renew), `loadMe()`, `logout()`

### sidebarStore
- `collapsed: boolean` — Sidebar expanded/collapsed state
- `generalChats: ChatResponse[]` — List of general chats
- **Methods**: `toggle()`, `addGeneralChat()`, `removeGeneralChat()`
- **Active item highlight is route-driven** (`$route`), not stored state

### llmProviderStore
- `providers: LlmProvider[]` — Available LLM providers (OpenAI, etc.)
- `selectedProviderId: string | null` — Currently selected provider
- **Methods**: `loadProviders()`, `selectProvider(id)`
- **Usage**: Passed as `llmProviderId` query param in API calls

### floatingChatStore
- `isOpen: boolean` — Floating chat panel visible state
- `chatId: string | null` — Associated chat document ID
- **Methods**: `open()`, `close()`, `toggleOpen()`, `setChatId(id)`

## API Integration Pattern

### Composable Structure
Each module has a dedicated composable (e.g., `useChatApi`, `useSimulationApi`):

```typescript
export function useChatApi() {
  async function createChat(req: AddChatMessageRequest): Promise<ChatResponse> {
    const { data } = await apiClient.post<ChatResponse>(BASE, req)
    return data
  }
  // ... more methods
  return { createChat, getChat, ... }
}
```

### Axios Client Setup
- **Base URL**: `import.meta.env.VITE_API_BASE_URL` (default: `/api/v1/intellisim`)
- **Timeout**: 30 seconds (lowered from 60s now that sends are async 202)
- **Request Interceptor**: Calls `authStore.getValidAccessToken()` before every request; proactively renews if the token expires within 30 seconds; attaches `Authorization: Bearer <token>` header.
- **Response Interceptor**: On 401, attempts one silent renew + retry. On all errors, normalises the backend `{ code, message, fields? }` envelope into `ApiError` instances.
- **Type Safety**: All responses typed with DTOs (e.g., `ChatResponse`, `ChatMessageResponse`)

### Error Handling
- **`ApiError`** class carries `code` (machine-readable), `status` (HTTP), `fields` (per-field validation map), and `message` (human-readable). Composables throw `ApiError` on failures.
- **Component Usage**: Try/catch blocks set loading state + display error toast or inline message; use `error.code` to distinguish cases (e.g., `model_unavailable` → show banner; `generation_in_progress` → disable send).
- **Format**: Backend envelope `{ code, message, fields? }` → normalized by Axios interceptor into `ApiError`.

## Build System

### Vite Configuration
```javascript
{
  plugins: [vue(), tailwindcss()],
  resolve: {
    alias: { '@': './src' }
  },
  server: {
    port: 3000,
    proxy: {
      '/api': { target: 'http://localhost:8080', changeOrigin: true },
      '/ws': { target: 'ws://localhost:8080', ws: true }
    }
  }
}
```

- **Dev Server**: `npm run dev` → Port 3000
- **Build**: `npm run build` → TypeScript compilation + vite build
- **Type Check**: `vue-tsc -b` (run before build)
- **Preview**: `npm run preview` → Local production build

### Environment Variables
```
VITE_API_BASE_URL=/api/v1/intellisim
VITE_WS_BASE_URL=ws://localhost:8080/api/v1/intellisim
VITE_AUTH_AUTHORITY=http://localhost:9090      # OIDC issuer (dev mock; Keycloak in prod)
VITE_AUTH_CLIENT_ID=intellisim-front           # OIDC client ID
```

### TypeScript Config
- **Target**: ES2020+ (set in tsconfig.dom.json)
- **Strict Mode**: Enabled (strict: true)
- **Unused Detection**: noUnusedLocals, noUnusedParameters enabled
- **Path Aliases**: `@/*` → `src/*`
- **Vue Support**: vue-tsc for .vue file checking

## Performance Considerations

### Code Splitting
- Routes lazy-loaded with dynamic imports
- Module-based organization enables tree-shaking

### WebSocket Chunking
- Simulation steps streamed in chunks of 1000 documents
- Chunk buffer in useWebSocketStream prevents UI blocking
- Message handling parses newline-separated JSON (supports partial messages)

### Rendering
- PixiJS canvas uses WebGL for efficient agent/shape rendering
- Vue reactivity limited to step/status/progress updates (not per-agent)
- Event delegation on canvas for hover/click interactions

### Asset Optimization
- Tailwind CSS purged in production
- PrimeVue components imported on-demand
- Icons (primeicons) lazy-loaded via CSS

## Next Steps / Planned Features

1. **Widget Persistence**: Save/load analysis dashboards from backend
2. **Multi-step Replay**: Scrubber to jump to specific simulation steps
3. **Agent Filtering**: Filter agents by properties in WorldCanvas
4. **Export Formats**: CSV, PNG exports from analysis widgets
5. **Real-time Collaboration**: Multi-user session support
