package simulations

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// defaultMemoryCap is the FIFO bound applied when a caller passes cap<=0.
const defaultMemoryCap = 20

// resolveMemoryCap returns the effective per-agent memory capacity by walking the
// three-level resolution chain:
//
//  1. arch.LLMConfig.SimulationConfig.MemorySize if > 0  (agent-level override)
//  2. model.SimulationConfig.LLMSimulationExecutionConfig.MemorySize if > 0  (model-level default)
//  3. 0  (AppendMemory applies its floor of defaultMemoryCap=20)
//
// Each pointer in the chain (LLMConfig, SimulationConfig, DefaultSimulationConfig)
// may be nil; all nil paths fall through safely to 0.
func resolveMemoryCap(model *models.Model, arch *models.Agent) int {
	// 1. Agent-level override.
	if arch != nil && arch.LLMConfig != nil && arch.LLMConfig.SimulationConfig != nil {
		if v := arch.LLMConfig.SimulationConfig.MemorySize; v > 0 {
			return v
		}
	}
	// 2. Model-level default.
	if model != nil {
		if v := model.SimulationConfig.LLMSimulationExecutionConfig.MemorySize; v > 0 {
			return v
		}
	}
	// 3. Zero → AppendMemory applies defaultMemoryCap=20.
	return 0
}

// AppendMemory adds an event to the agent's memory log and FIFO-evicts the oldest
// entries beyond cap (cap<=0 => default 20). now is injected so callers/tests
// control the timestamp.
func AppendMemory(ra *RuntimeAgent, step uint64, t datasims.AgentMemoryType, summary string, details any, cap int, now time.Time) {
	if ra == nil || ra.A == nil {
		return
	}
	if cap <= 0 {
		cap = defaultMemoryCap
	}
	ra.A.Memory = append(ra.A.Memory, datasims.AgentMemory{
		Step:      step,
		Type:      t,
		Summary:   summary,
		Details:   details,
		Timestamp: now,
	})
	if over := len(ra.A.Memory) - cap; over > 0 {
		// Drop the oldest `over` entries (front of the slice is oldest). Copy into a
		// fresh slice so the evicted backing array can be reclaimed.
		kept := make([]datasims.AgentMemory, cap)
		copy(kept, ra.A.Memory[over:])
		ra.A.Memory = kept
	}
}

// memoryLineWriter renders newest-first memory lines, collapsing CONSECUTIVE
// entries with an identical (Type, Summary) into one line suffixed " ×N" (N>1
// only; a run of one is byte-identical to the uncollapsed form). The rendered
// step is the run's NEWEST step. Each collapsed run consumes ONE line of the
// budget — more information per prompt token. Deterministic given the
// deterministic memory order.
type memoryLineWriter struct {
	b       strings.Builder
	written int
	max     int

	runOpen    bool
	runStep    uint64
	runType    datasims.AgentMemoryType
	runSummary string
	runCount   int
}

// add feeds one newest-first entry; returns false when the budget is exhausted.
func (w *memoryLineWriter) add(e datasims.AgentMemory) bool {
	if w.runOpen && e.Type == w.runType && e.Summary == w.runSummary {
		w.runCount++
		return true
	}
	if !w.flushRun() {
		return false
	}
	w.runOpen = true
	w.runStep = e.Step
	w.runType = e.Type
	w.runSummary = e.Summary
	w.runCount = 1
	return true
}

// flushRun writes the open run (if any) as one line; returns false when the
// budget was already exhausted.
func (w *memoryLineWriter) flushRun() bool {
	if !w.runOpen {
		return true
	}
	if w.written >= w.max {
		return false
	}
	w.b.WriteString("- step ")
	w.b.WriteString(strconv.FormatUint(w.runStep, 10))
	w.b.WriteString(" [")
	w.b.WriteString(string(w.runType))
	w.b.WriteString("]: ")
	w.b.WriteString(w.runSummary)
	if w.runCount > 1 {
		fmt.Fprintf(&w.b, " ×%d", w.runCount)
	}
	w.b.WriteByte('\n')
	w.written++
	w.runOpen = false
	return true
}

func (w *memoryLineWriter) result() string {
	w.flushRun()
	return strings.TrimRight(w.b.String(), "\n")
}

// SummarizeMemory renders the most recent events (newest first) as compact prompt
// text, stopping near maxEvents lines. Consecutive identical (Type, Summary)
// entries collapse into one "… ×N" line. Used by llm_runtime prompt assembly.
// Returns the empty string when there is nothing to report.
func SummarizeMemory(ra *RuntimeAgent, maxEvents int) string {
	if ra == nil || ra.A == nil || len(ra.A.Memory) == 0 {
		return ""
	}
	if maxEvents <= 0 {
		maxEvents = defaultMemoryCap
	}
	mem := ra.A.Memory
	w := &memoryLineWriter{max: maxEvents}
	// Iterate newest-first (memory is appended oldest-first).
	for i := len(mem) - 1; i >= 0; i-- {
		if !w.add(mem[i]) {
			break
		}
	}
	return w.result()
}

// SummarizeMemoryByType renders up to max newest-first memory lines whose Type is
// in types (adjacency for the ×N collapse is judged on the FILTERED stream).
// Empty result when none match. The trailing newline is trimmed so the output is
// symmetric with SummarizeMemory.
func SummarizeMemoryByType(ra *RuntimeAgent, types []datasims.AgentMemoryType, max int) string {
	if ra == nil || ra.A == nil || max <= 0 {
		return ""
	}
	want := map[datasims.AgentMemoryType]bool{}
	for _, t := range types {
		want[t] = true
	}
	w := &memoryLineWriter{max: max}
	for i := len(ra.A.Memory) - 1; i >= 0; i-- {
		e := ra.A.Memory[i]
		if !want[e.Type] {
			continue
		}
		if !w.add(e) {
			break
		}
	}
	return w.result()
}

// MemoryMatch supports ConditionTypeMemory: reports whether any event of type t
// whose Summary contains substr occurred within the last `within` steps. An empty
// substr matches any event of that type. within<=0 ignores recency. The recency
// window is measured relative to the newest event in the log.
func MemoryMatch(ra *RuntimeAgent, t datasims.AgentMemoryType, substr string, within int) bool {
	if ra == nil || ra.A == nil || len(ra.A.Memory) == 0 {
		return false
	}
	mem := ra.A.Memory
	// The newest recorded step stands in for "now" so recency is self-contained.
	current := mem[len(mem)-1].Step
	for i := len(mem) - 1; i >= 0; i-- {
		e := mem[i]
		if e.Type != t {
			continue
		}
		if within > 0 && current-e.Step >= uint64(within) {
			// Entries are ordered oldest-first; once we pass the window, the rest
			// are older still, so we can stop.
			break
		}
		if substr == "" || strings.Contains(e.Summary, substr) {
			return true
		}
	}
	return false
}
