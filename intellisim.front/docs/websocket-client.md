# WebSocket Client Guide

Real-time simulation streaming is implemented via two composables: a generic `useWebSocket` for low-level connection management, and `useWebSocketStream` for simulation-specific message handling and state aggregation.

## useWebSocket Composable

**File**: `src/shared/composables/useWebSocket.ts`

A reusable WebSocket client with automatic reconnection, exponential backoff, and configurable lifecycle hooks.

### Exports

```typescript
export interface UseWebSocketOptions {
  onMessage?: (data: unknown) => void    // Receives parsed JSON
  onOpen?: () => void                     // Connection established
  onClose?: () => void                    // Connection closed
  onError?: (error: Event) => void        // Connection error
  maxRetries?: number                     // Max reconnection attempts (default: 10)
  baseDelay?: number                      // Initial retry delay in ms (default: 1000)
}

export function useWebSocket(url: string, options: UseWebSocketOptions = {}) {
  const connected = ref(false)            // Current connection state
  const retryCount = ref(0)               // Number of retry attempts
  // ... returns: { connected, retryCount, connect, disconnect }
}
```

### State

- **connected**: Reactive ref (true when open, false when closed)
- **retryCount**: Ref tracking reconnection attempt count
- **intentionalClose**: Internal flag to prevent reconnect on manual disconnect

### Methods

#### `connect()`
Initiates WebSocket connection to the provided URL.

**Behavior**:
1. Creates new WebSocket(url)
2. Sets up listeners (onopen, onmessage, onclose, onerror)
3. Parses incoming messages as JSON (newline-separated for multi-doc payloads)
4. Sets connected.value = true on open, false on close

**Error Handling**:
- Non-JSON messages silently ignored (logged only to console)
- Connection errors trigger onerror callback

#### `disconnect()`
Gracefully closes the connection.

**Behavior**:
1. Sets intentionalClose = true (prevents auto-reconnect)
2. Clears reconnect timer if pending
3. Closes WebSocket
4. Sets connected.value = false

**Note**: Automatically called on component unmount via onUnmounted hook.

### Reconnection Strategy

If connection closes and retryCount < maxRetries:

```javascript
const delay = Math.min(baseDelay * Math.pow(2, retryCount), 30000)
// Exponential backoff: 1s, 2s, 4s, 8s, 16s... capped at 30s
retryCount++
reconnectTimer = setTimeout(connect, delay)
```

**Default Configuration**:
- Max 10 retries (30 seconds total wait time)
- Initial delay 1000ms
- Max delay capped at 30000ms

**Override Example**:
```typescript
useWebSocket(url, {
  maxRetries: 5,
  baseDelay: 500,
})
```

### Message Parsing

Incoming data is split by newlines and parsed as JSON:

```typescript
ws.onmessage = (event) => {
  try {
    const parts = (event.data as string).split('\n').filter(Boolean)
    for (const part of parts) {
      const data = JSON.parse(part)
      options.onMessage?.(data)  // One callback per JSON object
    }
  } catch {
    // Non-JSON message, ignore
  }
}
```

**Use Case**: Backend may buffer multiple JSON objects on one line, or spread one step across multiple lines.

### Usage Example

```typescript
const { connected, retryCount, connect, disconnect } = useWebSocket(
  'ws://localhost:8080/api/v1/intellisim/simulations/sim123/stream',
  {
    onMessage: (data) => console.log('Received:', data),
    onOpen: () => console.log('Connected'),
    onClose: () => console.log('Disconnected'),
    onError: (error) => console.error('WS Error:', error),
  }
)

// Establish connection
connect()

// Check connection state
watch(connected, (isConnected) => {
  console.log(isConnected ? 'Online' : 'Offline')
})

// Graceful shutdown
onUnmounted(disconnect)
```

---

## useWebSocketStream Composable

**File**: `src/modules/simulation/composables/useWebSocketStream.ts`

Higher-level composable wrapping useWebSocket with simulation-specific message types, chunk buffering for large steps, and reactive state aggregation.

### Exports

```typescript
export function useWebSocketStream(simulationId: string) {
  const currentStep = ref(0)
  const status = ref<SimulationStatus>('initiated')
  const progress = ref<ProgressInfo | null>(null)
  const latestAgents = ref<SimulationStepDocument[]>([])
  const latestShapes = ref<SimulationStepDocument[]>([])
  const llmUsageLog = ref<LLMUsageInfo[]>([])
  const errors = ref<Array<{ step: number; message: string }>>([])
  const consoleLog = ref<Array<{ type: string; message: string; timestamp: string }>>([])
  
  const connected = ref(boolean)
  const connect = () => void
  const disconnect = () => void
  
  return {
    connected, currentStep, status, progress, latestAgents, latestShapes,
    llmUsageLog, errors, consoleLog, connect, disconnect,
  }
}
```

### Reactive State

- **currentStep**: Current simulation step number
- **status**: Current simulation status ('initiated', 'running', 'finished', 'error')
- **progress**: Progress info object (e.g., `{ percentage: 45, estimated_remaining_time: '30s' }`)
- **latestAgents**: Array of agent documents at current step with position/properties
- **latestShapes**: Array of environment shape documents at current step
- **llmUsageLog**: Running log of LLM API calls with token counts
- **errors**: Array of step-level errors encountered
- **consoleLog**: Timestamped log entries (status changes, errors, completions)

### Connection Management

```typescript
const { connected, connect, disconnect } = useWebSocket(streamUrl, {
  onMessage: handleMessage,
})
```

Internally uses useWebSocket with the stream URL from `useSimulationApi.getStreamUrl(simulationId)`.

**Automatic Reconnection**: Enabled (10 retries, exponential backoff).

### Message Types and Handling

The composable handles 6 message types from the backend:

#### 1. **step_update** - Agent/Shape Data
```typescript
interface StepUpdateMessage {
  type: 'step_update'
  step: number
  updates: SimulationStepDocument[]
  totalChunks?: number        // Number of chunks for this step
  chunkIndex?: number         // This chunk's index
  timestamp: string
}
```

**Handler Logic**:
1. Extract step number
2. Initialize chunk buffer for step if new
3. Append updates to buffer
4. Increment received count
5. When all chunks received: separate agents/shapes, update reactive state
6. Clear buffer

**Example Flow**:
```
Step 100, Chunk 1/3: Buffer 1000 agents
Step 100, Chunk 2/3: Buffer 500 agents
Step 100, Chunk 3/3: Buffer 300 agents
→ All 1800 agents assigned to latestAgents
→ WorldCanvas watches and re-renders
```

#### 2. **progress** - Execution Progress
```typescript
interface ProgressMessage {
  type: 'progress'
  progress: ProgressInfo       // { percentage: 50, estimated_remaining_time: '5m' }
  timestamp: string
}
```

**Handler**: Updates `progress.value = msg.progress`

#### 3. **status_change** - Simulation State Transition
```typescript
interface StatusChangeMessage {
  type: 'status_change'
  status: {
    old_status: SimulationStatus
    new_status: SimulationStatus
  }
  timestamp: string
}
```

**Handler**:
- Updates `status.value`
- Logs to consoleLog: `"Status: old → new"`

#### 4. **completion** - Execution Finished
```typescript
interface CompletionMessage {
  type: 'completion'
  timestamp: string
}
```

**Handler**:
- Sets `status.value = 'finished'`
- Adds to consoleLog: `"Simulation completed"`

#### 5. **error** - Step Execution Error
```typescript
interface ErrorMessage {
  type: 'error'
  error: {
    step: number
    message: string
  }
  timestamp: string
}
```

**Handler**:
- Appends to `errors` array
- Logs to consoleLog: `"Step N: error message"`

#### 6. **llm_usage** - LLM API Call Tracking
```typescript
interface LlmUsageMessage {
  type: 'llm_usage'
  llm_usage: LLMUsageInfo       // { step: N, tokens_input: X, tokens_output: Y, ... }
  timestamp: string
}
```

**Handler**: Appends to `llmUsageLog`

### Chunk Buffering

For large simulations with many agents/shapes, the backend may split a step into multiple chunks:

```typescript
const chunkBuffer = reactive<Map<number, SimulationStepDocument[]>>(new Map())
const chunkCounts = reactive<Map<number, { received: number; total: number }>>(new Map())
```

**Design Rationale**:
- Prevents UI blocking from parsing/rendering 10,000+ documents
- Chunks of 1000 docs ensure reasonable message sizes
- Frontend assembles complete step before updating reactive state

**Example Scenario**:
```
Step 500: 2500 agents total
→ Backend sends 3 chunks (1000, 1000, 500)
→ Each chunk: { step: 500, updates: [...], totalChunks: 3, chunkIndex: 0/1/2 }
→ Frontend buffers all 3
→ On chunk 3, updates latestAgents with all 2500
```

### Usage Example

**In SimulationRunnerView.vue**:

```typescript
import { useWebSocketStream } from '@/modules/simulation/composables/useWebSocketStream'
import { useSimulationApi } from '@/modules/simulation/composables/useSimulationApi'

const props = defineProps<{ id: string }>()
const api = useSimulationApi()

const stream = useWebSocketStream(props.id)

onMounted(async () => {
  // Start simulation execution
  await api.start(props.id)
  
  // Connect WebSocket stream
  stream.connect()
})

onUnmounted(() => {
  stream.disconnect()
})
```

**In Template**:

```vue
<template>
  <!-- Display current step -->
  <div>Step: {{ stream.currentStep }} / 1000</div>
  
  <!-- Display status -->
  <div>Status: {{ stream.status }}</div>
  
  <!-- Display progress bar -->
  <div v-if="stream.progress">
    {{ stream.progress.percentage }}% 
    ({{ stream.progress.estimated_remaining_time }} remaining)
  </div>
  
  <!-- Pass agents/shapes to canvas -->
  <WorldCanvas 
    :agents="stream.latestAgents"
    :shapes="stream.latestShapes"
    env-width="1000"
    env-height="1000"
  />
  
  <!-- Error display -->
  <div v-for="err in stream.errors" :key="err.step" class="text-red-500">
    Step {{ err.step }}: {{ err.message }}
  </div>
  
  <!-- Console log -->
  <pre>{{ stream.consoleLog.map(l => l.message).join('\n') }}</pre>
</template>

<script setup lang="ts">
import { useWebSocketStream } from '@/modules/simulation/composables/useWebSocketStream'

const stream = useWebSocketStream(props.id)

// Watch for completion
watch(() => stream.status, (status) => {
  if (status === 'finished') {
    console.log('Simulation complete!')
  }
})
</script>
```

### Connection Lifecycle

```
Component Mount
  ↓
useWebSocketStream(simulationId) initialized
  ↓
stream.connect() called (in onMounted)
  ↓
WebSocket opens → useWebSocket sets connected = true
  ↓
Backend starts streaming messages
  ↓
Each message → handleMessage → update reactive state
  ↓
Component watches state (agents, status, progress)
  ↓
Template reactively re-renders
  ↓
Component Unmount
  ↓
stream.disconnect() called (in onUnmounted)
  ↓
WebSocket closes
  ↓
useWebSocket clears timers, sets connected = false
```

### State Consumption Patterns

**In Components**:

```typescript
// Watch current step
watch(() => stream.currentStep, (step) => {
  console.log(`Rendering step ${step}`)
})

// Check if simulation active
const isRunning = computed(() => stream.status === 'running')

// Monitor errors
watch(() => stream.errors.length, (count) => {
  if (count > 0) showErrorAlert()
})

// Get latest agent count
const agentCount = computed(() => stream.latestAgents.length)
```

**In Templates**:

```vue
<template>
  <!-- Conditional rendering -->
  <div v-if="stream.connected" class="text-green-500">Connected</div>
  <div v-else class="text-red-500">Disconnected (retrying...)</div>
  
  <!-- Display data -->
  <p>{{ stream.currentStep }} agents active</p>
  <p>Progress: {{ stream.progress?.percentage }}%</p>
  
  <!-- Iterate logs -->
  <ul>
    <li v-for="log in stream.consoleLog" :key="log.timestamp">
      [{{ log.type }}] {{ log.message }}
    </li>
  </ul>
</template>
```

### Performance Notes

1. **Message Throughput**: With chunk buffering, typical simulation sends 50-200 messages/second during active execution
2. **UI Updates**: Vue batches updates; agents/shapes prop change triggers WorldCanvas re-render (~60fps target)
3. **Memory**: Chunk buffers cleared after assembly; old step data kept in latestAgents/latestShapes only
4. **CPU**: JSON parsing done incrementally (line-by-line), PixiJS rendering optimized with WebGL

### Troubleshooting

**Connection Drops**:
- Check browser console for WebSocket errors
- Verify backend /simulations/:id/stream endpoint accessible
- Check firewall/proxy WebSocket support

**Messages Not Appearing**:
- Verify `stream.connected === true`
- Check backend is streaming (check server logs)
- Ensure message type case matches (case-sensitive)

**Step Counter Stuck**:
- Check chunk buffer not stalling: open DevTools, inspect chunkBuffer map size
- Verify `totalChunks` matches expected chunk count

**High Memory Usage**:
- Latestents held indefinitely; clear old data if needed: `latestAgents.value = []`
- Consider paging/windowing for very long simulations
