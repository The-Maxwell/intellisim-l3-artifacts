package simulations

import (
	"strings"
	"testing"

	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

func TestSummarizeIntent_DecisionLabel(t *testing.T) {
	in := &AgentIntent{Decisions: []DecisionIntent{{Label: "flee"}}}
	if got := summarizeIntent(in); got != "flee" {
		t.Fatalf("summarizeIntent=%q want flee", got)
	}
}

func TestSummarizeIntent_MultipleLabelsJoined(t *testing.T) {
	in := &AgentIntent{Decisions: []DecisionIntent{{Label: "flee"}, {Label: "hide"}}}
	if got := summarizeIntent(in); got != "flee, hide" {
		t.Fatalf("summarizeIntent=%q want %q", got, "flee, hide")
	}
}

func TestSummarizeIntent_LLMNoteFallback(t *testing.T) {
	// Full-control intents carry no Decisions; the compact LLMNote wins.
	in := &AgentIntent{LLMNote: "actions: set_property,move"}
	if got := summarizeIntent(in); got != "actions: set_property,move" {
		t.Fatalf("summarizeIntent=%q want the LLMNote", got)
	}
}

func TestSummarizeIntent_DecisionWinsOverNote(t *testing.T) {
	in := &AgentIntent{
		Decisions: []DecisionIntent{{Label: "hunt"}},
		LLMNote:   "chose: hunt",
	}
	if got := summarizeIntent(in); got != "hunt" {
		t.Fatalf("summarizeIntent=%q want the decision label, not the note", got)
	}
}

func TestSummarizeIntent_Empty(t *testing.T) {
	if got := summarizeIntent(nil); got != "" {
		t.Fatalf("summarizeIntent(nil)=%q want empty", got)
	}
	if got := summarizeIntent(&AgentIntent{}); got != "" {
		t.Fatalf("summarizeIntent(zero)=%q want empty", got)
	}
	// Empty labels are skipped, not joined into ", ".
	in := &AgentIntent{Decisions: []DecisionIntent{{Label: ""}}}
	if got := summarizeIntent(in); got != "" {
		t.Fatalf("summarizeIntent(empty label)=%q want empty", got)
	}
}

func TestSummarizeIntent_Truncation(t *testing.T) {
	long := strings.Repeat("д", stepDecisionMaxRunes+40) // multi-byte runes: cap counts runes, not bytes
	in := &AgentIntent{LLMNote: long}
	got := summarizeIntent(in)
	if r := []rune(got); len(r) != stepDecisionMaxRunes {
		t.Fatalf("summarizeIntent length=%d runes want %d", len(r), stepDecisionMaxRunes)
	}
}

func TestRecordStepDecisions_SetAndClear(t *testing.T) {
	rs, group := manyWolves(2, models.ArchetypeModeIdentical, 0)
	a1, a2 := group.Members[0], group.Members[1]

	// Pre-dirty a2 to prove recordStepDecisions clears agents that received no
	// intent this step.
	a2.stepDecision = "stale"

	intents := map[uint64]*AgentIntent{
		a1.A.ID: {Decisions: []DecisionIntent{{Label: "hunt"}}},
	}
	recordStepDecisions(rs, intents)
	if a1.stepDecision != "hunt" {
		t.Fatalf("agent 1 stepDecision=%q want hunt", a1.stepDecision)
	}
	if a2.stepDecision != "" {
		t.Fatalf("agent 2 stepDecision=%q want cleared (no intent this step)", a2.stepDecision)
	}

	clearStepDecisions(rs)
	if a1.stepDecision != "" {
		t.Fatalf("clearStepDecisions left %q on agent 1", a1.stepDecision)
	}
}

func TestRecordStepDecisions_SkipsRemovedAndUnknown(t *testing.T) {
	rs, group := manyWolves(2, models.ArchetypeModeIdentical, 0)
	a1 := group.Members[0]
	a1.removed = true

	intents := map[uint64]*AgentIntent{
		a1.A.ID: {Decisions: []DecisionIntent{{Label: "hunt"}}},
		999:     {Decisions: []DecisionIntent{{Label: "ghost"}}}, // unknown id: ignored
	}
	recordStepDecisions(rs, intents)
	if a1.stepDecision != "" {
		t.Fatalf("removed agent got stepDecision=%q want empty", a1.stepDecision)
	}
}

// TestBuildStepRows_EmitsDecision proves the transient stepDecision lands on the
// agent's timeseries row (SimulationStep.Decision) and only on agents that have one.
func TestBuildStepRows_EmitsDecision(t *testing.T) {
	rs, group := manyWolves(2, models.ArchetypeModeIdentical, 0)
	a1, a2 := group.Members[0], group.Members[1]
	a1.stepDecision = "flee"

	rows := buildStepRows(rs, rs.Step)

	found1, found2 := false, false
	for _, row := range rows {
		if row.Metadata.Type != datasims.MetadataTypeAgent {
			continue
		}
		switch row.Metadata.ReferenceID {
		case a1.A.ID:
			found1 = true
			if row.Decision != "flee" {
				t.Fatalf("agent 1 row Decision=%q want flee", row.Decision)
			}
		case a2.A.ID:
			found2 = true
			if row.Decision != "" {
				t.Fatalf("agent 2 row Decision=%q want empty", row.Decision)
			}
		}
	}
	if !found1 || !found2 {
		t.Fatalf("expected both agent rows present (found1=%v found2=%v)", found1, found2)
	}
}
