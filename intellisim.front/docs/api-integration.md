# API Integration Guide

## Axios Client Setup

**File**: `src/shared/api/client.ts`

The Axios client is configured as a singleton instance used by all API composables.

### Configuration

```typescript
const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api/v1/intellisim',
  headers: {
    'Content-Type': 'application/json',
  },
  timeout: 60000, // 60 seconds
})
```

### Error Interceptor

```typescript
apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    const message = error.response?.data?.error || error.message || 'Request failed'
    return Promise.reject(new Error(message))
  }
)
```

**Behavior**:
- Successful responses pass through unchanged
- Failed responses extract `error` field from backend response or fallback to generic message
- Returns Promise.reject(Error) so composables can catch and display error messages

### Base URL

- **Default**: `/api/v1/intellisim`
- **Override**: Set `VITE_API_BASE_URL` environment variable
- **Development**: Proxied via Vite to `http://localhost:8080`

### Timeout

- **60 seconds** for all requests
- Suitable for long-running LLM operations and large data transfers

---

## Chat API Composable

**File**: `src/modules/chat/composables/useChatApi.ts`

Provides CRUD operations for general-purpose chats and their messages.

### Methods

#### `createChat(req: AddChatMessageRequest): Promise<ChatResponse>`
- **Endpoint**: `POST /chats`
- **Request**: `{ content: string, llmProviderId?: string }`
- **Response**: Full chat with initial message and LLM response
- **Usage**: Starting a new general chat
- **Example**:
  ```typescript
  const api = useChatApi()
  const chat = await api.createChat({ content: 'Hello', llmProviderId: 'openai' })
  console.log(chat.messages) // [...initial messages]
  ```

#### `getChat(id: string, page?: number, pageSize?: number): Promise<ChatResponse>`
- **Endpoint**: `GET /chats/:id?page=1&pageSize=50`
- **Request Query Params**: `page` (default 1), `pageSize` (default 50)
- **Response**: Chat object with paginated message list
- **Usage**: Loading existing chat with scrollback support
- **Example**:
  ```typescript
  const chat = await api.getChat('abc123', 2, 20) // Page 2, 20 per page
  ```

#### `listChats(page?: number, pageSize?: number, active?: boolean): Promise<ListChatsResponse>`
- **Endpoint**: `GET /chats?page=1&pageSize=20&active=true`
- **Request Query Params**: `page`, `pageSize`, `active` (optional filter)
- **Response**: `{ chats: ChatResponse[], total: number }`
- **Usage**: Fetching sidebar chat list
- **Example**:
  ```typescript
  const { chats, total } = await api.listChats(1, 20, true) // Active chats only
  ```

#### `addMessage(id: string, req: AddChatMessageRequest): Promise<ChatResponse>`
- **Endpoint**: `POST /chats/:id/messages`
- **Request**: `{ content: string, llmProviderId?: string }`
- **Response**: Updated chat with new user and assistant messages
- **Usage**: Sending a user message and receiving LLM response
- **Example**:
  ```typescript
  const updated = await api.addMessage('abc123', { content: 'Next question' })
  messages.value = updated.messages
  ```

#### `updateChat(id: string, req: UpdateChatRequest): Promise<ChatResponse>`
- **Endpoint**: `PUT /chats/:id`
- **Request**: `{ name?: string, description?: string }`
- **Response**: Updated chat object
- **Usage**: Renaming or updating chat metadata
- **Example**:
  ```typescript
  await api.updateChat('abc123', { name: 'My Chat' })
  ```

#### `deleteChat(id: string): Promise<void>`
- **Endpoint**: `DELETE /chats/:id`
- **Usage**: Removing a chat from history
- **Example**:
  ```typescript
  await api.deleteChat('abc123')
  ```

---

## Model Chat API Composable

**File**: `src/modules/model/composables/useModelChatApi.ts`

Provides CRUD operations for model creation chats, model editing, and image-based environment generation.

### Methods

#### `createChat(req: AddChatMessageRequest): Promise<ChatResponse>`
- **Endpoint**: `POST /modelchats`
- **Request**: `{ content: string, llmProviderId?: string }`
- **Response**: Initial model chat with LLM's first suggestions
- **Usage**: Starting a new model design session
- **Example**:
  ```typescript
  const chat = await api.createChat({ content: 'Design a predator-prey model' })
  ```

#### `getChat(id: string, page?: number, pageSize?: number): Promise<ChatResponse>`
- **Endpoint**: `GET /modelchats/:id?page=1&pageSize=50`
- **Response**: Model chat with paginated messages
- **Usage**: Loading conversation history

#### `addMessage(id: string, req: AddChatMessageRequest): Promise<ChatResponse>`
- **Endpoint**: `POST /modelchats/:id/messages`
- **Response**: Updated chat with new message + LLM refinement
- **Usage**: Continuing model design conversation
- **Example**:
  ```typescript
  const updated = await api.addMessage('abc123', { content: 'Add disease transmission' })
  ```

#### `getModel(id: string): Promise<ModelGenerationResponse>`
- **Endpoint**: `GET /modelchats/:id/model`
- **Response**: `{ agents: AgentArchetype[], environment: SimulationEnvironment, parameters: ModelParameters }`
- **Usage**: Fetching current model definition after chat refinement
- **Example**:
  ```typescript
  const model = await api.getModel('abc123')
  // model.agents[0] = { name: 'Predator', properties: {...} }
  ```

#### `updateModel(id: string, req: ManualUpdateModelRequest): Promise<{ model: ModelGenerationResponse; warnings?: string[] }>`
- **Endpoint**: `PATCH /modelchats/:id/model`
- **Request**: `{ agents: AgentArchetype[], environment?: SimulationEnvironment, parameters?: ModelParameters }`
- **Response**: Updated model + optional warnings array
- **Usage**: Manual JSON editing (ModelEditorView)
- **Example**:
  ```typescript
  const result = await api.updateModel('abc123', {
    agents: [...modifiedAgents],
    parameters: {...updatedParams}
  })
  if (result.warnings) console.warn(result.warnings)
  ```

#### `updateChat(id: string, req: UpdateChatRequest): Promise<ChatResponse>`
- **Endpoint**: `PUT /modelchats/:id`
- **Request**: `{ name?: string, description?: string }`
- **Response**: Updated chat
- **Usage**: Renaming model chat

#### `deleteChat(id: string): Promise<void>`
- **Endpoint**: `DELETE /modelchats/:id`
- **Usage**: Removing model and its chat history

#### `generateEnvironmentFromImage(id: string, file: File, prompt?: string): Promise<unknown>`
- **Endpoint**: `POST /modelchats/:id/environment/from-image`
- **Request**: FormData with `image` file + optional `prompt`
- **Response**: Generated environment definition
- **Usage**: Vision-based environment layout from image
- **Example**:
  ```typescript
  const file = new File([...], 'map.png', { type: 'image/png' })
  const env = await api.generateEnvironmentFromImage('abc123', file, 'Urban grid layout')
  ```

---

## Simulation API Composable

**File**: `src/modules/simulation/composables/useSimulationApi.ts`

Comprehensive API for simulation lifecycle, execution, and result queries.

### Configuration Methods

#### `initiate(modelId: string, req: InitiateSimulationRequest): Promise<SimulationDocument>`
- **Endpoint**: `POST /simulations?modelId=:modelId`
- **Request**: `{ name?: string, steps: number, environment?: {...}, parameters?: {...} }`
- **Response**: New simulation document with status 'initiated'
- **Usage**: Creating a simulation from a model
- **Example**:
  ```typescript
  const sim = await api.initiate('model123', {
    steps: 1000,
    environment: { width: 100, height: 100 }
  })
  ```

#### `get(id: string): Promise<SimulationDocument>`
- **Endpoint**: `GET /simulations/:id`
- **Response**: Current simulation document with all metadata
- **Usage**: Loading simulation details

#### `update(id: string, req: UpdateSimulationRequest): Promise<SimulationDocument>`
- **Endpoint**: `PATCH /simulations/:id`
- **Request**: `{ name?: string, steps?: number, parameters?: {...} }`
- **Response**: Updated simulation
- **Usage**: Modifying simulation configuration before execution

#### `remove(id: string): Promise<void>`
- **Endpoint**: `DELETE /simulations/:id`
- **Usage**: Deleting a simulation

### Execution Methods

#### `start(id: string): Promise<{ message: string; simulation_id: string; status: string }>`
- **Endpoint**: `POST /simulations/:id/start`
- **Response**: `{ message, simulation_id, status }`
- **Usage**: Beginning execution
- **Example**:
  ```typescript
  const result = await api.start('sim123')
  console.log(result.status) // 'running'
  // Now connect WebSocket to stream results
  ```

#### `stop(id: string): Promise<void>`
- **Endpoint**: `POST /simulations/:id/stop`
- **Usage**: Halting execution

#### `getStatus(id: string): Promise<SimulationStatusResponse>`
- **Endpoint**: `GET /simulations/:id/status`
- **Response**: `{ status: 'running' | 'finished' | 'error', currentStep: number, progress: number }`
- **Usage**: Polling simulation progress (not needed if using WebSocket)
- **Example**:
  ```typescript
  const status = await api.getStatus('sim123')
  ```

### Chat Methods (Simulation Discussion)

#### `addMessage(id: string, req: AddChatMessageRequest): Promise<ChatResponse>`
- **Endpoint**: `POST /simulations/:id/messages`
- **Request**: `{ content: string, llmProviderId?: string }`
- **Response**: Updated chat with analysis/insights
- **Usage**: Asking LLM questions about results during/after simulation

### Result Query Methods

#### `queryResults(id: string, query: ResultQuery): Promise<ResultData>`
- **Endpoint**: `POST /simulations/:id/query`
- **Request**: `{ type: string, property?: string, step?: number, ... }`
- **Response**: Structured result data (varies by query type)
- **Usage**: Flexible result queries

#### `getResultsSummary(id: string): Promise<ResultData>`
- **Endpoint**: `GET /simulations/:id/results/summary`
- **Response**: Overview metrics (total agents, steps completed, etc.)
- **Usage**: Initial results dashboard
- **Example**:
  ```typescript
  const summary = await api.getResultsSummary('sim123')
  // { totalAgents: 1000, stepsCompleted: 500, ... }
  ```

#### `getTimeSeries(id: string, property: string, params?: Record<string, string>): Promise<ResultData>`
- **Endpoint**: `GET /simulations/:id/results/timeseries/:property?...`
- **Response**: Array of `TimeSeriesPoint` objects: `{ step: number, value: number | string }`
- **Usage**: Plotting property changes over time (LinePlotWidget)
- **Example**:
  ```typescript
  const data = await api.getTimeSeries('sim123', 'population_density')
  // [{ step: 0, value: 50 }, { step: 1, value: 52 }, ...]
  ```

#### `getSpatialAtStep(id: string, step: number, properties?: string[]): Promise<SpatialAgentData[]>`
- **Endpoint**: `GET /simulations/:id/results/spatial/:step?properties=...`
- **Request Query**: `properties` comma-separated list (optional)
- **Response**: Array of agents with position + properties at step
- **Usage**: Rendering world at specific step or current step
- **Example**:
  ```typescript
  const agents = await api.getSpatialAtStep('sim123', 100, ['x', 'y', 'health'])
  // [{ position: {x, y}, properties: {...}, metadata: {...} }, ...]
  ```

#### `getPopulation(id: string, start?: number, end?: number): Promise<PopulationPoint[]>`
- **Endpoint**: `GET /simulations/:id/results/population?start=0&end=1000`
- **Response**: Array of `PopulationPoint`: `{ step: number, count: { [archetype]: number } }`
- **Usage**: Charting archetype populations (PopulationChartWidget)
- **Example**:
  ```typescript
  const pop = await api.getPopulation('sim123')
  // [{ step: 0, count: { Predator: 10, Prey: 100 } }, ...]
  ```

#### `getAgentTrajectory(id: string, agentId: string): Promise<unknown>`
- **Endpoint**: `GET /simulations/:id/results/agent/:agentId/trajectory`
- **Response**: Time-series position + state of agent
- **Usage**: Tracing individual agent path
- **Example**:
  ```typescript
  const trajectory = await api.getAgentTrajectory('sim123', 'agent_42')
  ```

#### `getAgentMemory(id: string, agentId: string): Promise<unknown>`
- **Endpoint**: `GET /simulations/:id/results/agent/:agentId/memory`
- **Response**: Agent's internal state/memory log
- **Usage**: Debugging agent decision-making

#### `getProperties(id: string): Promise<PropertyInfo[]>`
- **Endpoint**: `GET /simulations/:id/results/properties`
- **Response**: `[{ name: 'health', type: 'number', ... }, ...]`
- **Usage**: Discovering available result properties (for widget config)
- **Example**:
  ```typescript
  const props = await api.getProperties('sim123')
  props.forEach(p => console.log(p.name))
  ```

### Analysis Methods

#### `analyze(id: string, req: AnalysisRequest): Promise<AnalysisResult>`
- **Endpoint**: `POST /simulations/:id/analyze`
- **Request**: `{ prompt: string, properties?: string[] }`
- **Response**: `{ analysis: string, insights: string[] }`
- **Usage**: LLM analysis of results
- **Example**:
  ```typescript
  const result = await api.analyze('sim123', {
    prompt: 'What drove predator population decline?',
    properties: ['population', 'energy', 'reproduction_rate']
  })
  console.log(result.analysis) // LLM-generated text
  ```

#### `analyzePdf(id: string, req: AnalysisRequest): Promise<Blob>`
- **Endpoint**: `POST /simulations/:id/analyze/pdf`
- **Request**: Same as analyze
- **Response**: PDF binary blob
- **Config**: `responseType: 'blob'`
- **Usage**: Generating downloadable PDF report
- **Example**:
  ```typescript
  const blob = await api.analyzePdf('sim123', { prompt: 'Summary report' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = 'analysis.pdf'
  a.click()
  ```

### WebSocket Methods

#### `getStreamUrl(id: string): string`
- **Returns**: WebSocket URL for simulation streaming
- **Format**: `ws://localhost:8080/api/v1/intellisim/simulations/:id/stream`
- **Usage**: Used by `useWebSocketStream` composable
- **Example**:
  ```typescript
  const url = api.getStreamUrl('sim123')
  // 'ws://localhost:8080/api/v1/intellisim/simulations/sim123/stream'
  ```

---

## LLM Providers API

**File**: `src/shared/api/llmProviders.ts`

Provides endpoints for LLM provider configuration and discovery.

### Expected Methods

```typescript
export function useLlmProviderApi() {
  async function getProviders(): Promise<LlmProvider[]> {
    // GET /llm-providers
  }
  async function selectProvider(id: string): Promise<void> {
    // POST /llm-providers/:id/select
  }
  // ... more methods
}
```

**Integration**: Used by `useLlmProviderStore` (Pinia) to load and manage LLM provider selection.

---

## Type Alignment

All API response types are defined in `src/shared/types/` and match backend DTOs:

### Key Types

```typescript
// Chat
interface ChatResponse {
  id: string
  name: string
  messages: ChatMessageResponse[]
  createdAt: string
  updatedAt: string
}

interface ChatMessageResponse {
  type: 'user' | 'assistant' | 'system'
  content: string
  createdAt: string
  createdBy: string
}

// Simulation
interface SimulationDocument {
  id: string
  modelId: string
  name: string
  status: SimulationStatus
  steps: number
  currentStep: number
  createdAt: string
}

interface SimulationStepDocument {
  metadata: { name: string; type: 'agent' | 'shape' }
  position?: { x: number; y: number }
  properties?: Record<string, unknown>
}

// Results
interface TimeSeriesPoint {
  step: number
  value: number | string
}

interface PopulationPoint {
  step: number
  count: Record<string, number>
}

// Model
interface ModelGenerationResponse {
  agents: AgentArchetype[]
  environment: SimulationEnvironment
  parameters: ModelParameters
}
```

**Maintenance**: Keep these types in sync with backend definitions by:
1. Checking backend API documentation/comments
2. Running type-checking tests against live API
3. Updating on each backend API change

---

## Error Handling Pattern

All composables throw errors from the Axios interceptor:

```typescript
// Component usage
try {
  const chat = await useChatApi().createChat({ content: 'Hello' })
  messages.value = chat.messages
} catch (error) {
  errorMessage.value = error instanceof Error ? error.message : 'Failed'
  // User sees: "error response message" or fallback "Failed"
}
```

**Best Practices**:
1. Always try/catch in components or use `defineAsyncComponent`
2. Display `error.message` to user
3. Log full error to console for debugging
4. Set loading state before try, reset in finally

---

## API Request/Response Flow Example

Simulated chat message flow:

```
User Types "Hello" in ChatPanel
  ↓
ChatPanel emits send: "Hello"
  ↓
ChatView.handleSend() {
  loading.value = true
  try {
    const response = await useChatApi().addMessage(chatId, {
      content: "Hello",
      llmProviderId: selected
    })
    messages.value = response.messages
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
  ↓
Axios interceptor catches response
  ↓
Backend returns: { messages: [...], error?: null }
  ↓
Composable returns response data
  ↓
Component updates messages ref
  ↓
Template reactively re-renders new messages
  ↓
ChatPanel auto-scrolls to bottom
```
