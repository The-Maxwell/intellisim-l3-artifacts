package simulations

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
	"intelli.sim.core/modules/auth"
)

// fakeStepsQuery is a queryable steps repository: canned rows served through the
// same paginated GetSteps contract the Mongo implementation honors.
type fakeStepsQuery struct {
	fakeStepSink
	rows []datasims.SimulationStep
}

func (f *fakeStepsQuery) GetStepRange(_ context.Context, _ string) (int64, int64, error) {
	if len(f.rows) == 0 {
		return 0, 0, nil
	}
	lo, hi := f.rows[0].Metadata.Step, f.rows[0].Metadata.Step
	for _, r := range f.rows {
		if r.Metadata.Step < lo {
			lo = r.Metadata.Step
		}
		if r.Metadata.Step > hi {
			hi = r.Metadata.Step
		}
	}
	return lo, hi, nil
}

func (f *fakeStepsQuery) GetSteps(_ context.Context, _ string, from, to, page, pageSize uint64) ([]datasims.SimulationStep, bool, error) {
	var in []datasims.SimulationStep
	for _, r := range f.rows {
		s := uint64(r.Metadata.Step)
		if s >= from && s <= to {
			in = append(in, r)
		}
	}
	start := (page - 1) * pageSize
	if start >= uint64(len(in)) {
		return nil, false, nil
	}
	end := start + pageSize
	if end > uint64(len(in)) {
		end = uint64(len(in))
	}
	return in[start:end], end < uint64(len(in)), nil
}

// exportFixture: a finished sim whose snapshot declares agent property "energy"
// and env global "alarm", plus three persisted rows across two steps.
func exportFixture() (*datasims.Simulation, *fakeStepsQuery) {
	sim := &datasims.Simulation{
		ID:        "sim-1",
		Name:      "export me",
		Status:    datasims.SimulationStatusFinished,
		CreatedBy: "alice",
		ModelSnapshot: &datamodels.Model{
			Agents: []datamodels.Agent{{
				UID:        "walker",
				Properties: []datamodels.AgentProperty{{UID: "energy", Type: datamodels.PropertyTypeNumber}},
			}},
			Environment: &datamodels.Environment{
				GlobalProperties: []datamodels.AgentProperty{{UID: "alarm", Type: datamodels.PropertyTypeBoolean}},
			},
		},
	}
	steps := &fakeStepsQuery{rows: []datasims.SimulationStep{
		{
			Metadata:   datasims.StepMetadata{SimulationID: "sim-1", Step: 0, Type: datasims.MetadataTypeAgent, UID: "walker", ReferenceID: 1},
			Position:   &common.Point{X: 10, Y: 20},
			State:      "moving",
			Properties: map[string]interface{}{"energy": 75.0},
		},
		{
			Metadata:   datasims.StepMetadata{SimulationID: "sim-1", Step: 0, Type: datasims.MetadataTypeEnvironment, UID: "environment", ReferenceID: 0},
			Properties: map[string]interface{}{"alarm": true},
			Statistics: &datasims.SimulationStatistics{StepsCompleted: 1, TotalAgents: 1},
		},
		{
			Metadata:   datasims.StepMetadata{SimulationID: "sim-1", Step: 1, Type: datasims.MetadataTypeAgent, UID: "walker", ReferenceID: 1},
			Position:   &common.Point{X: 11, Y: 20},
			State:      "moving",
			Decision:   "flee",
			Properties: map[string]interface{}{"energy": 74.0},
		},
	}}
	return sim, steps
}

// U3: CSV export streams every persisted row with flattened columns — fixed
// leading columns, then the sorted union of the snapshot's declared property
// UIDs, then a statistics JSON column (env rows only).
func TestExportSteps_CSV(t *testing.T) {
	sim, steps := exportFixture()
	svc := newTestRunService(newFakeSimRepo(sim), &fakeRunStarter{}, &fakeTickets{}, steps)

	exp, err := svc.ExportSteps(ctxAs("alice", false), "sim-1", "csv", 0, nil)
	if err != nil {
		t.Fatalf("ExportSteps: %v", err)
	}
	if got := exp.ContentType(); got != "text/csv" {
		t.Fatalf("ContentType=%q want text/csv", got)
	}
	var buf bytes.Buffer
	if err := exp.WriteTo(context.Background(), &buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	recs, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(recs) != 4 {
		t.Fatalf("rows=%d want 4 (header + 3 rows)", len(recs))
	}
	wantHeader := []string{"step", "type", "uid", "reference_id", "x", "y", "state", "decision", "alarm", "energy", "statistics"}
	if strings.Join(recs[0], ",") != strings.Join(wantHeader, ",") {
		t.Fatalf("header=%v want %v", recs[0], wantHeader)
	}
	agent0 := recs[1]
	if agent0[0] != "0" || agent0[1] != "agent" || agent0[2] != "walker" || agent0[3] != "1" ||
		agent0[4] != "10" || agent0[5] != "20" || agent0[6] != "moving" || agent0[7] != "" ||
		agent0[8] != "" || agent0[9] != "75" || agent0[10] != "" {
		t.Fatalf("agent row 0 mismatch: %v", agent0)
	}
	env := recs[2]
	if env[1] != "environment" || env[8] != "true" || env[9] != "" {
		t.Fatalf("env row mismatch: %v", env)
	}
	if !strings.Contains(env[10], "steps_completed") {
		t.Fatalf("env statistics column must carry the JSON snapshot, got %q", env[10])
	}
	if agent1 := recs[3]; agent1[0] != "1" || agent1[7] != "flee" || agent1[9] != "74" {
		t.Fatalf("agent row 1 mismatch: %v", agent1)
	}
}

// U3: JSONL export writes one self-describing JSON object per row (pandas
// read_json(lines=True) compatible) with only-present fields.
func TestExportSteps_JSONL(t *testing.T) {
	sim, steps := exportFixture()
	svc := newTestRunService(newFakeSimRepo(sim), &fakeRunStarter{}, &fakeTickets{}, steps)

	exp, err := svc.ExportSteps(ctxAs("alice", false), "sim-1", "jsonl", 0, nil)
	if err != nil {
		t.Fatalf("ExportSteps: %v", err)
	}
	if got := exp.ContentType(); got != "application/x-ndjson" {
		t.Fatalf("ContentType=%q want application/x-ndjson", got)
	}
	var buf bytes.Buffer
	if err := exp.WriteTo(context.Background(), &buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines=%d want 3", len(lines))
	}
	var first map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("line 1 not JSON: %v", err)
	}
	if first["type"] != "agent" || first["uid"] != "walker" {
		t.Fatalf("line 1 mismatch: %v", first)
	}
	props, _ := first["properties"].(map[string]interface{})
	if props["energy"] != 75.0 {
		t.Fatalf("line 1 properties mismatch: %v", first)
	}
	var envRow map[string]interface{}
	if err := json.Unmarshal([]byte(lines[1]), &envRow); err != nil {
		t.Fatalf("line 2 not JSON: %v", err)
	}
	if _, ok := envRow["statistics"]; !ok {
		t.Fatalf("env line must carry statistics: %v", envRow)
	}
}

// U3: export honors ownership like every other simulation read.
func TestExportSteps_ForeignForbidden(t *testing.T) {
	sim, steps := exportFixture()
	svc := newTestRunService(newFakeSimRepo(sim), &fakeRunStarter{}, &fakeTickets{}, steps)

	_, err := svc.ExportSteps(ctxAs("bob", false), "sim-1", "csv", 0, nil)
	if !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("err=%v want auth.ErrForbidden", err)
	}
}

// U3: an explicit step range is honored (from/to inclusive).
func TestExportSteps_Range(t *testing.T) {
	sim, steps := exportFixture()
	svc := newTestRunService(newFakeSimRepo(sim), &fakeRunStarter{}, &fakeTickets{}, steps)

	to := uint64(0)
	exp, err := svc.ExportSteps(ctxAs("alice", false), "sim-1", "jsonl", 0, &to)
	if err != nil {
		t.Fatalf("ExportSteps: %v", err)
	}
	var buf bytes.Buffer
	if err := exp.WriteTo(context.Background(), &buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if lines := strings.Split(strings.TrimSpace(buf.String()), "\n"); len(lines) != 2 {
		t.Fatalf("lines=%d want 2 (step 0 only)", len(lines))
	}
}
