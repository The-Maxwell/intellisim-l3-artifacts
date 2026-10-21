package simulations

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	datasims "intelli.sim.core/data/simulations"
)

// roomCount returns the number of live rooms in the hub (test-only accessor).
func (h *StreamHub) roomCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms)
}

// dialHubWS spins up an httptest server whose only handler upgrades the request and
// calls hub.Join(simID, conn), returning Join's bool result on a channel; the test
// dials it as a WebSocket client. It returns the joined result and the client conn.
func dialHubWS(t *testing.T, hub *StreamHub, simID string) (joined bool, client *websocket.Conn, cleanup func()) {
	t.Helper()
	upgrader := websocket.Upgrader{}
	joinedCh := make(chan bool, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			joinedCh <- false
			return
		}
		joinedCh <- hub.Join(simID, conn)
	}))
	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		srv.Close()
		t.Fatalf("dial ws: %v", err)
	}
	select {
	case joined = <-joinedCh:
	case <-time.After(2 * time.Second):
		c.Close()
		srv.Close()
		t.Fatal("server handler did not report a Join result")
	}
	return joined, c, func() {
		c.Close()
		srv.Close()
	}
}

// TestStreamHub_JoinWithoutOpenRoomRefuses asserts the orphan-room guard: joining a
// simulation that has no live run (no OpenRoom called) must NOT create a room and
// must report failure, so no actor goroutine + map entry leaks per post-run connect.
func TestStreamHub_JoinWithoutOpenRoomRefuses(t *testing.T) {
	hub := NewStreamHub(RunConfig{}, zap.NewNop())

	joined, _, cleanup := dialHubWS(t, hub, "sim-dead")
	defer cleanup()

	if joined {
		t.Fatal("Join must refuse (return false) when no room exists for the simulation")
	}
	if n := hub.roomCount(); n != 0 {
		t.Fatalf("Join lazily created %d room(s); want 0 (orphan-room guard)", n)
	}
}

// TestStreamHub_OpenRoomThenJoin asserts the happy path: the RunManager opens the
// room at run start, then a client join succeeds and attaches to that room.
func TestStreamHub_OpenRoomThenJoin(t *testing.T) {
	hub := NewStreamHub(RunConfig{}, zap.NewNop())
	hub.OpenRoom("sim-live", 0)
	if n := hub.roomCount(); n != 1 {
		t.Fatalf("OpenRoom created %d room(s); want 1", n)
	}

	joined, _, cleanup := dialHubWS(t, hub, "sim-live")
	defer cleanup()

	if !joined {
		t.Fatal("Join must succeed when the run room is open")
	}

	// CloseRoom tears the room down (engine calls it on completion); the map entry
	// must be gone afterwards.
	hub.CloseRoom("sim-live")
	if n := hub.roomCount(); n != 0 {
		t.Fatalf("CloseRoom left %d room(s); want 0", n)
	}
}

// TestStreamHub_OpenRoomIdempotent asserts repeated OpenRoom calls (e.g. a re-run)
// do not stack rooms.
func TestStreamHub_OpenRoomIdempotent(t *testing.T) {
	hub := NewStreamHub(RunConfig{}, zap.NewNop())
	hub.OpenRoom("sim-x", 0)
	hub.OpenRoom("sim-x", 0)
	if n := hub.roomCount(); n != 1 {
		t.Fatalf("OpenRoom is not idempotent: %d rooms want 1", n)
	}
}

// agentRow builds one per-entity agent SimulationStep row with a position and a
// single property so it counts as "changed".
func agentRow(step uint64, id uint64) datasims.SimulationStep {
	pos := common.Point{X: id, Y: id}
	return datasims.SimulationStep{
		Metadata: datasims.StepMetadata{
			SimulationID: "sim",
			Step:         int64(step),
			ReferenceID:  id,
			Type:         datasims.MetadataTypeAgent,
			UID:          "wolf",
		},
		Position:   &pos,
		Properties: map[string]interface{}{"energy": id},
	}
}

// emptyAgentRow builds an agent row with no position and no properties (an
// "unchanged" entity that changedOnly should omit).
func emptyAgentRow(step uint64, id uint64) datasims.SimulationStep {
	return datasims.SimulationStep{
		Metadata: datasims.StepMetadata{
			SimulationID: "sim",
			Step:         int64(step),
			ReferenceID:  id,
			Type:         datasims.MetadataTypeAgent,
			UID:          "wolf",
		},
	}
}

func TestChunkStep_RespectsBatchSize(t *testing.T) {
	rows := make([]datasims.SimulationStep, 0, 50)
	for i := uint64(1); i <= 50; i++ {
		rows = append(rows, agentRow(7, i))
	}

	msgs := chunkStep("sim", rows, 20, false)
	if len(msgs) != 3 {
		t.Fatalf("chunks=%d want 3 (20+20+10)", len(msgs))
	}
	if msgs[0].TotalChunks != 3 {
		t.Fatalf("TotalChunks=%d want 3", msgs[0].TotalChunks)
	}
	if msgs[0].ChunkIndex != 0 || msgs[1].ChunkIndex != 1 || msgs[2].ChunkIndex != 2 {
		t.Fatalf("chunk indices wrong: %d,%d,%d", msgs[0].ChunkIndex, msgs[1].ChunkIndex, msgs[2].ChunkIndex)
	}
	if len(msgs[0].Agents) != 20 || len(msgs[1].Agents) != 20 || len(msgs[2].Agents) != 10 {
		t.Fatalf("chunk sizes wrong: %d,%d,%d", len(msgs[0].Agents), len(msgs[1].Agents), len(msgs[2].Agents))
	}
	for _, m := range msgs {
		if m.Type != FrameStep {
			t.Fatalf("Type=%q want %q", m.Type, FrameStep)
		}
		if m.Step != 7 {
			t.Fatalf("Step=%d want 7", m.Step)
		}
		// Every agent frame carries the archetype definition UID so the
		// dashboard can resolve archetype-scoped widgets from live frames.
		for _, a := range m.Agents {
			if a.AgentUID != "wolf" {
				t.Fatalf("agent %d AgentUID=%q want wolf", a.ID, a.AgentUID)
			}
		}
	}
}

func TestChunkStep_ChangedOnlyOmitsUnchanged(t *testing.T) {
	rows := []datasims.SimulationStep{
		agentRow(3, 1),
		emptyAgentRow(3, 2),
		agentRow(3, 3),
		emptyAgentRow(3, 4),
	}

	msgs := chunkStep("sim", rows, 100, true)
	if len(msgs) != 1 {
		t.Fatalf("chunks=%d want 1", len(msgs))
	}
	got := msgs[0].Agents
	if len(got) != 2 {
		t.Fatalf("changedOnly kept %d frames want 2 (omitting empty rows)", len(got))
	}
	ids := map[uint64]bool{}
	for _, a := range got {
		ids[a.ID] = true
	}
	if !ids[1] || !ids[3] {
		t.Fatalf("expected ids 1 and 3, got %v", ids)
	}
	if ids[2] || ids[4] {
		t.Fatalf("changedOnly must omit empty rows 2 and 4, got %v", ids)
	}
}

// TestChunkStep_CarriesDecision proves a per-step LLM decision summary rides the
// agent frame, and that a decision-ONLY row survives changedOnly (a decision is
// an observable change feeding the llm_decisions log).
func TestChunkStep_CarriesDecision(t *testing.T) {
	withDecision := agentRow(4, 1)
	withDecision.Decision = "flee"
	decisionOnly := emptyAgentRow(4, 2)
	decisionOnly.Decision = "hold"
	rows := []datasims.SimulationStep{withDecision, decisionOnly, emptyAgentRow(4, 3)}

	msgs := chunkStep("sim", rows, 100, true)
	if len(msgs) != 1 {
		t.Fatalf("chunks=%d want 1", len(msgs))
	}
	got := map[uint64]string{}
	for _, a := range msgs[0].Agents {
		got[a.ID] = a.Decision
	}
	if got[1] != "flee" || got[2] != "hold" {
		t.Fatalf("decisions on the wire=%v want {1:flee 2:hold}", got)
	}
	if _, present := got[3]; present {
		t.Fatalf("payload-less row 3 must still be omitted under changedOnly: %v", got)
	}
}

func TestChunkStep_IncludesGlobalProperties(t *testing.T) {
	envRow := datasims.SimulationStep{
		Metadata: datasims.StepMetadata{
			SimulationID: "sim",
			Step:         5,
			ReferenceID:  0,
			Type:         datasims.MetadataTypeEnvironment,
		},
		Properties: map[string]interface{}{"temperature": 21},
	}
	rows := []datasims.SimulationStep{agentRow(5, 1), envRow}

	msgs := chunkStep("sim", rows, 100, false)
	if len(msgs) != 1 {
		t.Fatalf("chunks=%d want 1", len(msgs))
	}
	if msgs[0].GlobalProperties == nil || msgs[0].GlobalProperties["temperature"] != 21 {
		t.Fatalf("global properties not carried: %+v", msgs[0].GlobalProperties)
	}
	if len(msgs[0].Agents) != 1 {
		t.Fatalf("agent frames=%d want 1", len(msgs[0].Agents))
	}
}

func TestChunkStep_EmptyRowsProducesNoMessages(t *testing.T) {
	if msgs := chunkStep("sim", nil, 20, false); len(msgs) != 0 {
		t.Fatalf("nil rows produced %d messages want 0", len(msgs))
	}
	// changedOnly with only unchanged rows yields nothing.
	rows := []datasims.SimulationStep{emptyAgentRow(1, 1), emptyAgentRow(1, 2)}
	if msgs := chunkStep("sim", rows, 20, true); len(msgs) != 0 {
		t.Fatalf("all-unchanged changedOnly produced %d messages want 0", len(msgs))
	}
}

func shapeRow(step int64, id uint64, props map[string]interface{}) datasims.SimulationStep {
	return datasims.SimulationStep{
		Metadata:   datasims.StepMetadata{SimulationID: "sim", Step: step, ReferenceID: id, Type: datasims.MetadataTypeShape, UID: "lake"},
		Properties: props,
	}
}

func TestChunkStep_ShapesRideFirstChunk(t *testing.T) {
	rows := []datasims.SimulationStep{agentRow(3, 1), agentRow(3, 2), agentRow(3, 3), shapeRow(3, 9, map[string]interface{}{"water_level": 42.0})}
	msgs := chunkStep("sim", rows, 2, false) // 3 agents / batch 2 → 2 chunks
	if len(msgs) != 2 {
		t.Fatalf("chunks=%d want 2", len(msgs))
	}
	if len(msgs[0].Shapes) != 1 || msgs[0].Shapes[0].ID != 9 || msgs[0].Shapes[0].Properties["water_level"] != 42.0 {
		t.Fatalf("first chunk shapes wrong: %+v", msgs[0].Shapes)
	}
	if len(msgs[1].Shapes) != 0 {
		t.Fatalf("shapes must ride the first chunk only, second has %+v", msgs[1].Shapes)
	}
}

func TestChunkStep_ShapeOnlyRow(t *testing.T) {
	msgs := chunkStep("sim", []datasims.SimulationStep{shapeRow(4, 9, map[string]interface{}{"water_level": 41.0})}, 20, true)
	if len(msgs) != 1 || len(msgs[0].Shapes) != 1 || len(msgs[0].Agents) != 0 {
		t.Fatalf("shape-only row not delivered: %+v", msgs)
	}
	// changedOnly drops payload-less shape frames entirely
	if msgs := chunkStep("sim", []datasims.SimulationStep{shapeRow(4, 9, nil)}, 20, true); len(msgs) != 0 {
		t.Fatalf("empty shape frame must be dropped under changedOnly, got %+v", msgs)
	}
}

// compile-time guarantee mirrored in stream.go; asserted here too so the test
// build fails fast if the hub drifts from the engine port.
var _ stepBroadcaster = (*StreamHub)(nil)

// TestOpenRoom_CarriesBatchSize asserts that the batch size passed to OpenRoom is
// honoured by BroadcastKeyframe: a keyframe with N agents must chunk by that size.
// A zero batch size must fall back to streamFrameDefaultBatch (the existing guard in
// chunkStep).
func TestOpenRoom_CarriesBatchSize(t *testing.T) {
	t.Run("custom batch size chunks keyframe", func(t *testing.T) {
		hub := NewStreamHub(RunConfig{}, zap.NewNop())
		const batchSize = 3
		hub.OpenRoom("sim-batch", batchSize)

		room := hub.room("sim-batch")
		if room == nil {
			t.Fatal("OpenRoom did not create a room")
		}
		if room.batchSize != batchSize {
			t.Fatalf("room.batchSize=%d want %d", room.batchSize, batchSize)
		}

		// Build a RuntimeState with 7 agents so the keyframe must split into
		// ceil(7/3)=3 chunks.
		rs := &RuntimeState{Step: 1}
		for i := uint64(1); i <= 7; i++ {
			pos := common.Point{X: i, Y: i}
			rs.Agents = append(rs.Agents, &RuntimeAgent{
				A: &datasims.SimulationAgent{
					ID:       i,
					AgentUID: "wolf",
					State:    "idle",
					Position: &pos,
				},
			})
		}

		// Capture the enqueued frames before BroadcastKeyframe drains them.
		msgs := chunkStep("sim-batch", buildStepRows(rs, rs.Step), batchSize, false)
		if len(msgs) != 3 {
			t.Fatalf("chunks=%d want 3 (ceil(7/3)=3) for batchSize=%d", len(msgs), batchSize)
		}
		for i, m := range msgs {
			if m.Type != FrameStep {
				t.Fatalf("chunk[%d].Type=%q want %q", i, m.Type, FrameStep)
			}
		}
		if len(msgs[0].Agents) != 3 || len(msgs[1].Agents) != 3 || len(msgs[2].Agents) != 1 {
			t.Fatalf("chunk sizes=%d,%d,%d want 3,3,1",
				len(msgs[0].Agents), len(msgs[1].Agents), len(msgs[2].Agents))
		}

		hub.CloseRoom("sim-batch")
	})

	t.Run("zero batch size falls back to default", func(t *testing.T) {
		hub := NewStreamHub(RunConfig{}, zap.NewNop())
		hub.OpenRoom("sim-zero", 0)

		room := hub.room("sim-zero")
		if room == nil {
			t.Fatal("OpenRoom did not create a room")
		}
		// A zero batchSize is stored as-is on the room; chunkStep's own guard
		// floors it to streamFrameDefaultBatch, so a 5-agent keyframe is one chunk.
		rows := make([]datasims.SimulationStep, 5)
		for i := range rows {
			rows[i] = agentRow(1, uint64(i+1))
		}
		msgs := chunkStep("sim-zero", rows, room.batchSize, false)
		// chunkStep clamps batchSize<1 to streamFrameDefaultBatch, so 5 agents fit
		// in one chunk.
		if len(msgs) != 1 {
			t.Fatalf("chunks=%d want 1 (zero batchSize clamps to default)", len(msgs))
		}

		hub.CloseRoom("sim-zero")
	})
}
