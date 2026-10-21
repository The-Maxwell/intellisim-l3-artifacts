package simulations

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	datasims "intelli.sim.core/data/simulations"
)

// FrameType is the discriminator of a WebSocket stream frame envelope.
type FrameType string

const (
	// FrameStep carries one (chunked) per-step delta of changed entities.
	FrameStep FrameType = "step"
	// FrameKeyframe carries a full-state snapshot for late joiners / reconnects.
	FrameKeyframe FrameType = "keyframe"
	// FrameStatus carries a run lifecycle status change.
	FrameStatus FrameType = "status"
	// FrameProgress carries live run progress (reserved; SSE owns progress today).
	FrameProgress FrameType = "progress"
	// FrameError carries a run error notification.
	FrameError FrameType = "error"
	// FrameCompletion carries the terminal run status; the room tears down after.
	FrameCompletion FrameType = "completion"
)

// streamFrameDefaultBatch is the fallback agent-per-chunk size when the run
// config does not carry one (RunConfig has no stream batch knob today).
const streamFrameDefaultBatch = 200

// AgentFrame is one entity's slice of a stream frame. Position/Properties/State
// are omitted when absent so step (delta) frames stay compact. AgentUID is the
// MODEL DEFINITION UID of the agent's archetype (StepMetadata.UID) so the
// dashboard can resolve archetype-scoped widgets (population/state_distribution/
// agent_table and agent_uids-filtered sources) from live frames.
type AgentFrame struct {
	ID         uint64                 `json:"id"`
	AgentUID   string                 `json:"agent_uid,omitempty"`
	Position   *common.Point          `json:"position,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	State      string                 `json:"state,omitempty"`
	// Decision is the compact LLM decision summary applied to this agent THIS
	// step (only set on decision steps; feeds text_log "llm_decisions").
	Decision string `json:"decision,omitempty"`
}

// ShapeFrame is one shape instance's per-step property snapshot on the wire.
type ShapeFrame struct {
	ID         uint64                 `json:"id"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// StreamTokenDelta is the token spend contributed by a SINGLE step. reasoning_tokens
// is the reasoning portion of completion_tokens (a subset, already included in
// completion/total) — a breakdown detail, never a summand.
type StreamTokenDelta struct {
	PromptTokens     uint64 `json:"prompt_tokens"`
	CompletionTokens uint64 `json:"completion_tokens"`
	ReasoningTokens  uint64 `json:"reasoning_tokens"`
	TotalTokens      uint64 `json:"total_tokens"`
	Calls            uint64 `json:"calls"`
}

// StreamMessage is the typed JSON envelope sent over the run WebSocket. A single
// step may be split into several messages (chunk_index / total_chunks) when it
// carries more agents than the batch size.
type StreamMessage struct {
	Type             FrameType              `json:"type"`
	Step             uint64                 `json:"step,omitempty"`
	ChunkIndex       int                    `json:"chunk_index,omitempty"`
	TotalChunks      int                    `json:"total_chunks,omitempty"`
	Agents           []AgentFrame           `json:"agents,omitempty"`
	Shapes           []ShapeFrame           `json:"shapes,omitempty"`
	GlobalProperties map[string]interface{} `json:"global_properties,omitempty"`
	Statistics       any                    `json:"statistics,omitempty"`
	// TokenUsage is the cumulative run-so-far breakdown (Total + by_model/by_agent/
	// by_purpose/by_generation/by_run), identical in shape to the persisted DTO so
	// the run screen's TokenUsagePanel binds to it live. StepTokens is this step's
	// delta. Both ride a step/keyframe frame; neither is persisted per step.
	TokenUsage *SimulationTokenUsageDTO `json:"token_usage,omitempty"`
	StepTokens *StreamTokenDelta        `json:"step_tokens,omitempty"`
	Status     string                   `json:"status,omitempty"`
	Error      string                   `json:"error,omitempty"`
	Timestamp  int64                    `json:"timestamp"`
}

// tokenDelta computes the per-step spend from the current and previous cumulative
// totals. Totals are monotonic so each field is a non-negative subtraction.
// ReasoningTokens rides alongside as an independent counter; total is never
// re-derived from the parts.
func tokenDelta(cur, prev common.TokenUsage) StreamTokenDelta {
	return StreamTokenDelta{
		PromptTokens:     cur.PromptTokens - prev.PromptTokens,
		CompletionTokens: cur.CompletionTokens - prev.CompletionTokens,
		ReasoningTokens:  cur.ReasoningTokens - prev.ReasoningTokens,
		TotalTokens:      cur.TotalTokens - prev.TotalTokens,
		Calls:            cur.Calls - prev.Calls,
	}
}

// StreamHub fans run frames out to the WebSocket clients of each simulation,
// one room per simulation id. It satisfies the engine's stepBroadcaster port.
type StreamHub struct {
	mu     sync.RWMutex
	rooms  map[string]*streamRoom
	cfg    RunConfig
	logger *zap.Logger
}

// Compile-time guarantee that StreamHub remains a valid engine broadcaster port.
var _ stepBroadcaster = (*StreamHub)(nil)

// NewStreamHub builds an empty hub. A nil logger becomes a no-op logger.
func NewStreamHub(cfg RunConfig, logger *zap.Logger) *StreamHub {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &StreamHub{
		rooms:  make(map[string]*streamRoom),
		cfg:    cfg,
		logger: logger.Named("run-stream"),
	}
}

// OpenRoom creates the simulation's stream room (idempotent). The RunManager calls
// it at run start so the room — and its actor goroutine — exists for the duration
// of the live run and is torn down by CloseRoom on completion. Rooms are created
// ONLY here (and never lazily on Join) so a client connecting for a non-running
// simulation cannot orphan a room whose goroutine would block forever.
// batchSize is stored on the room and used by BroadcastStep/BroadcastKeyframe; 0
// causes chunkStep to fall back to streamFrameDefaultBatch (defense-in-depth).
func (h *StreamHub) OpenRoom(simID string, batchSize int) {
	h.getOrCreateRoom(simID, batchSize)
}

// Join registers a freshly-upgraded connection in the simulation's room and
// starts its read/write pumps. The caller (the WS handler) has already validated
// the connect ticket + ownership + streamable status. The room must already exist
// (opened by the RunManager at run start); if none exists — the run finished or
// was torn down between the ownership re-check and the upgrade — the connection is
// closed rather than lazily creating an orphan room. Reports whether a room was
// joined.
func (h *StreamHub) Join(simID string, conn *websocket.Conn) bool {
	room := h.room(simID)
	if room == nil {
		_ = conn.Close()
		return false
	}
	client := &streamClient{
		room: room,
		conn: conn,
		send: make(chan []byte, 256),
	}
	select {
	case room.register <- client:
	case <-room.done:
		_ = conn.Close()
		return false
	}
	go client.writePump()
	go client.readPump()
	return true
}

// BroadcastStep chunks one per-entity step row set into envelope messages and
// enqueues them to the room (best-effort; never blocks the engine). The engine
// calls this once per persisted SimulationStep row, so a single call usually
// yields one agent frame; chunkStep keeps the contract uniform for replay/keyframe.
func (h *StreamHub) BroadcastStep(simID string, step datasims.SimulationStep, changedOnly bool) {
	room := h.room(simID)
	if room == nil {
		return
	}
	for _, msg := range chunkStep(simID, []datasims.SimulationStep{step}, room.batchSize, changedOnly) {
		room.enqueue(msg)
	}
}

// BroadcastStepUsage streams the run's live token-usage breakdown (cumulative)
// plus this step's delta as its own step frame. The engine calls this once per
// step (never per entity row), so token totals are attached exactly once per
// step; the frontend merges it into the step by step number. usage may be nil.
func (h *StreamHub) BroadcastStepUsage(simID string, step uint64, usage *SimulationTokenUsageDTO, delta StreamTokenDelta) {
	room := h.room(simID)
	if room == nil {
		return
	}
	d := delta
	room.enqueue(StreamMessage{
		Type:       FrameStep,
		Step:       step,
		TokenUsage: usage,
		StepTokens: &d,
		Timestamp:  nowMillis(),
	})
}

// BroadcastKeyframe streams a full-state snapshot of every live agent so late
// joiners / reconnects can sync without replaying history.
func (h *StreamHub) BroadcastKeyframe(simID string, rs *RuntimeState) {
	room := h.room(simID)
	if room == nil || rs == nil {
		return
	}
	rows := buildStepRows(rs, rs.Step)
	msgs := chunkStep(simID, rows, room.batchSize, false)
	for i := range msgs {
		msgs[i].Type = FrameKeyframe
		room.enqueue(msgs[i])
	}
}

// BroadcastStatus streams a run lifecycle status change.
func (h *StreamHub) BroadcastStatus(simID string, status datasims.SimulationStatus) {
	room := h.room(simID)
	if room == nil {
		return
	}
	room.enqueue(StreamMessage{Type: FrameStatus, Status: string(status), Timestamp: nowMillis()})
}

// BroadcastCompletion streams the terminal run status. CloseRoom is expected to
// follow (the engine calls it), draining clients after the frame is delivered.
func (h *StreamHub) BroadcastCompletion(simID string, status datasims.SimulationStatus) {
	room := h.room(simID)
	if room == nil {
		return
	}
	room.enqueue(StreamMessage{Type: FrameCompletion, Status: string(status), Timestamp: nowMillis()})
}

// CloseRoom tears down the simulation's room, disconnecting every client.
func (h *StreamHub) CloseRoom(simID string) {
	h.mu.Lock()
	room, ok := h.rooms[simID]
	if ok {
		delete(h.rooms, simID)
	}
	h.mu.Unlock()
	if ok {
		room.close()
	}
}

// --- internals ------------------------------------------------------------------

func (h *StreamHub) room(simID string) *streamRoom {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.rooms[simID]
}

func (h *StreamHub) getOrCreateRoom(simID string, batchSize int) *streamRoom {
	h.mu.Lock()
	defer h.mu.Unlock()
	if room, ok := h.rooms[simID]; ok {
		return room
	}
	room := newStreamRoom(simID, batchSize, h.logger.With(zap.String("simulation_id", simID)))
	h.rooms[simID] = room
	go room.run()
	return room
}

// streamRoom is the per-simulation actor: a single goroutine owns the client set
// and serializes register / unregister / broadcast so no per-client lock is
// needed on the hot path.
type streamRoom struct {
	simID      string
	batchSize  int // per-room chunk size; 0 means chunkStep falls back to streamFrameDefaultBatch
	logger     *zap.Logger
	clients    map[*streamClient]bool
	register   chan *streamClient
	unregister chan *streamClient
	broadcast  chan []byte
	done       chan struct{}
	closeOnce  sync.Once
}

func newStreamRoom(simID string, batchSize int, logger *zap.Logger) *streamRoom {
	return &streamRoom{
		simID:      simID,
		batchSize:  batchSize,
		logger:     logger,
		clients:    make(map[*streamClient]bool),
		register:   make(chan *streamClient),
		unregister: make(chan *streamClient),
		broadcast:  make(chan []byte, 256),
		done:       make(chan struct{}),
	}
}

func (r *streamRoom) run() {
	defer func() {
		for client := range r.clients {
			close(client.send)
		}
	}()
	for {
		select {
		case <-r.done:
			return
		case client := <-r.register:
			r.clients[client] = true
		case client := <-r.unregister:
			if _, ok := r.clients[client]; ok {
				delete(r.clients, client)
				close(client.send)
			}
		case data := <-r.broadcast:
			for client := range r.clients {
				select {
				case client.send <- data:
				default:
					// Slow client: drop it (best-effort delivery; the client
					// refetches missed history via GET /:id/steps).
					delete(r.clients, client)
					close(client.send)
				}
			}
		}
	}
}

// enqueue marshals a frame once and pushes it to the room. A full broadcast
// buffer drops the frame so the engine is never blocked by a slow hub.
func (r *streamRoom) enqueue(msg StreamMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		r.logger.Warn("failed to marshal stream frame", zap.Error(err))
		return
	}
	select {
	case r.broadcast <- data:
	case <-r.done:
	default:
		r.logger.Debug("stream broadcast buffer full; dropping frame", zap.String("type", string(msg.Type)))
	}
}

// requestUnregister asks the room actor to drop a client without blocking the
// caller (the read/write pump) if the room is shutting down.
func (r *streamRoom) requestUnregister(client *streamClient) {
	select {
	case r.unregister <- client:
	case <-r.done:
	}
}

func (r *streamRoom) close() {
	r.closeOnce.Do(func() { close(r.done) })
}

const (
	streamWriteWait   = 10 * time.Second
	streamPongWait    = 60 * time.Second
	streamPingPeriod  = (streamPongWait * 9) / 10
	streamMaxMsgBytes = 1 << 20 // 1 MiB inbound cap (clients send only pongs/control)
)

// streamClient is one WebSocket connection and its outbound buffer.
type streamClient struct {
	room *streamRoom
	conn *websocket.Conn
	send chan []byte
}

// writePump drains the client's send buffer to the socket, coalescing any
// queued frames into a single websocket message, and emits periodic pings. A
// closed send channel or a write error tears the connection down.
func (c *streamClient) writePump() {
	ticker := time.NewTicker(streamPingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(streamWriteWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				_ = w.Close()
				return
			}
			// Coalesce any already-queued frames into the same websocket frame,
			// newline-delimited, to reduce per-message overhead under load.
			n := len(c.send)
			for i := 0; i < n; i++ {
				if _, err := w.Write([]byte{'\n'}); err != nil {
					_ = w.Close()
					return
				}
				if _, err := w.Write(<-c.send); err != nil {
					_ = w.Close()
					return
				}
			}
			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(streamWriteWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-c.room.done:
			_ = c.conn.SetWriteDeadline(time.Now().Add(streamWriteWait))
			_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}
	}
}

// readPump drains inbound frames (the protocol is server-push only; clients send
// nothing but control/pong) and refreshes the read deadline on each pong, so a
// dead peer is detected. On any read error it unregisters the client.
func (c *streamClient) readPump() {
	defer func() {
		c.room.requestUnregister(c)
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(streamMaxMsgBytes)
	_ = c.conn.SetReadDeadline(time.Now().Add(streamPongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(streamPongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.room.logger.Debug("websocket read error", zap.Error(err))
			}
			return
		}
	}
}

// chunkStep is the pure envelope-building core: it converts a set of per-entity
// SimulationStep rows (all sharing one step number) into one or more StreamMessage
// envelopes, splitting the agent frames into chunks of at most batchSize. When
// changedOnly is true, agent rows carrying no payload (no position, no properties,
// no state) are treated as unchanged and omitted. Environment rows contribute the
// frame's global_properties and shape rows (Type == MetadataTypeShape) contribute
// the frame's shapes; both are attached to the first chunk only. Under changedOnly,
// payload-less shape frames are dropped. Returns nil when there is nothing to send.
// Pure: no sockets, no shared state — unit-testable.
func chunkStep(simID string, rows []datasims.SimulationStep, batchSize int, changedOnly bool) []StreamMessage {
	if batchSize < 1 {
		batchSize = streamFrameDefaultBatch
	}

	var (
		frames []AgentFrame
		shapes []ShapeFrame
		global map[string]interface{}
		stats  any
		stepNo uint64
	)
	for i := range rows {
		row := rows[i]
		stepNo = uint64(row.Metadata.Step)
		switch row.Metadata.Type {
		case datasims.MetadataTypeEnvironment:
			if len(row.Properties) > 0 {
				if global == nil {
					global = make(map[string]interface{}, len(row.Properties))
				}
				for k, v := range row.Properties {
					global[k] = v
				}
			}
			// Stream the accumulated per-step statistics snapshot live (previously
			// only surfaced on replay). Same *SimulationStatistics the env row
			// carries, so the JSON is byte-identical to GET /:id/steps.
			if row.Statistics != nil {
				stats = row.Statistics
			}
		case datasims.MetadataTypeShape:
			sf := ShapeFrame{ID: row.Metadata.ReferenceID, Properties: row.Properties}
			if changedOnly && len(sf.Properties) == 0 {
				continue
			}
			shapes = append(shapes, sf)
		default: // agent (and any spatial entity carried as an agent row)
			frame := AgentFrame{
				ID:         row.Metadata.ReferenceID,
				AgentUID:   row.Metadata.UID,
				Position:   row.Position,
				Properties: row.Properties,
				State:      row.State,
				Decision:   row.Decision,
			}
			if changedOnly && frameUnchanged(frame) {
				continue
			}
			frames = append(frames, frame)
		}
	}

	if len(frames) == 0 && len(global) == 0 && len(shapes) == 0 && stats == nil {
		return nil
	}

	ts := nowMillis()

	// No agent frames: a single envelope carrying the global properties, shapes and
	// statistics. (This is the path the live run loop's env-row broadcast takes.)
	if len(frames) == 0 {
		return []StreamMessage{{
			Type:             FrameStep,
			Step:             stepNo,
			TotalChunks:      1,
			Shapes:           shapes,
			GlobalProperties: global,
			Statistics:       stats,
			Timestamp:        ts,
		}}
	}

	totalChunks := (len(frames) + batchSize - 1) / batchSize
	msgs := make([]StreamMessage, 0, totalChunks)
	for idx := 0; idx < totalChunks; idx++ {
		start := idx * batchSize
		end := start + batchSize
		if end > len(frames) {
			end = len(frames)
		}
		msg := StreamMessage{
			Type:        FrameStep,
			Step:        stepNo,
			ChunkIndex:  idx,
			TotalChunks: totalChunks,
			Agents:      frames[start:end],
			Timestamp:   ts,
		}
		// Carry global properties, shapes and statistics on the first chunk only.
		if idx == 0 {
			msg.Shapes = shapes
			msg.GlobalProperties = global
			msg.Statistics = stats
		}
		msgs = append(msgs, msg)
	}
	return msgs
}

// frameUnchanged reports whether an agent frame carries no observable delta.
// A decision-only frame IS an observable change (it feeds the llm_decisions
// log); AgentUID alone is identity, not a delta payload.
func frameUnchanged(f AgentFrame) bool {
	return f.Position == nil && len(f.Properties) == 0 && f.State == "" && f.Decision == ""
}

// nowMillis is the millisecond epoch timestamp stamped on every envelope.
func nowMillis() int64 {
	return nowFn().UnixMilli()
}
