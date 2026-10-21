package simulations

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"

	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// exportPageSize bounds each repository pull while streaming an export — rows
// are written page by page, never materialized in full.
const exportPageSize = 2000

// stepExporter streams a simulation's persisted step history (U3) as CSV or
// JSONL. Built by runService.ExportSteps after ownership/range resolution so
// the HTTP handler can set headers before any byte is written.
type stepExporter struct {
	steps    datasims.SimulationStepsRepository
	simID    string
	format   string // "csv" | "jsonl"
	from, to uint64
	propCols []string // sorted union of declared property UIDs (CSV columns)
}

// ExportSteps prepares a bulk step-history export (U3). Ownership is enforced
// like every other simulation read; an omitted `to` resolves to the last
// recorded step (mirrors GetSteps). Any simulation status is exportable — a
// running simulation yields its persisted prefix.
func (s *runService) ExportSteps(ctx context.Context, id, format string, from uint64, to *uint64) (*stepExporter, error) {
	sim, _, err := s.getOwnedSimulation(ctx, id)
	if err != nil {
		return nil, err
	}
	end := uint64(0)
	if to != nil {
		end = *to
	} else {
		_, max, err := s.steps.GetStepRange(ctx, id)
		if err != nil {
			return nil, err
		}
		if max > 0 {
			end = uint64(max)
		}
	}
	return &stepExporter{
		steps:    s.steps,
		simID:    id,
		format:   format,
		from:     from,
		to:       end,
		propCols: exportPropertyColumns(sim.ModelSnapshot),
	}, nil
}

func (e *stepExporter) ContentType() string {
	if e.format == "jsonl" {
		return "application/x-ndjson"
	}
	return "text/csv"
}

func (e *stepExporter) Filename() string {
	ext := "csv"
	if e.format == "jsonl" {
		ext = "jsonl"
	}
	return e.simID + "-steps." + ext
}

// WriteTo streams every row in [from, to] to w. Errors after the first byte
// cannot change the HTTP status — the handler logs them.
func (e *stepExporter) WriteTo(ctx context.Context, w io.Writer) error {
	if e.format == "jsonl" {
		return e.writeJSONL(ctx, w)
	}
	return e.writeCSV(ctx, w)
}

// forEachRow pages through the steps repository in order, invoking fn per row.
func (e *stepExporter) forEachRow(ctx context.Context, fn func(*datasims.SimulationStep) error) error {
	for page := uint64(1); ; page++ {
		rows, hasMore, err := e.steps.GetSteps(ctx, e.simID, e.from, e.to, page, exportPageSize)
		if err != nil {
			return err
		}
		for i := range rows {
			if err := fn(&rows[i]); err != nil {
				return err
			}
		}
		if !hasMore {
			return nil
		}
	}
}

func (e *stepExporter) writeCSV(ctx context.Context, w io.Writer) error {
	cw := csv.NewWriter(w)
	header := append([]string{"step", "type", "uid", "reference_id", "x", "y", "state", "decision"}, e.propCols...)
	header = append(header, "statistics")
	if err := cw.Write(header); err != nil {
		return err
	}
	err := e.forEachRow(ctx, func(row *datasims.SimulationStep) error {
		rec := make([]string, 0, len(header))
		rec = append(rec,
			strconv.FormatInt(row.Metadata.Step, 10),
			string(row.Metadata.Type),
			row.Metadata.UID,
			strconv.FormatUint(row.Metadata.ReferenceID, 10),
		)
		if row.Position != nil {
			rec = append(rec, strconv.FormatUint(row.Position.X, 10), strconv.FormatUint(row.Position.Y, 10))
		} else {
			rec = append(rec, "", "")
		}
		rec = append(rec, row.State, row.Decision)
		for _, col := range e.propCols {
			rec = append(rec, exportValueString(row.Properties[col]))
		}
		stats := ""
		if row.Statistics != nil {
			b, err := json.Marshal(row.Statistics)
			if err != nil {
				return err
			}
			stats = string(b)
		}
		rec = append(rec, stats)
		return cw.Write(rec)
	})
	if err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

// exportJSONRow is one JSONL line: self-describing, only-present fields, loads
// directly via pandas read_json(lines=True).
type exportJSONRow struct {
	Step        int64                           `json:"step"`
	Type        string                          `json:"type"`
	UID         string                          `json:"uid"`
	ReferenceID uint64                          `json:"reference_id"`
	X           *uint64                         `json:"x,omitempty"`
	Y           *uint64                         `json:"y,omitempty"`
	State       string                          `json:"state,omitempty"`
	Decision    string                          `json:"decision,omitempty"`
	Properties  map[string]interface{}          `json:"properties,omitempty"`
	Statistics  *datasims.SimulationStatistics  `json:"statistics,omitempty"`
}

func (e *stepExporter) writeJSONL(ctx context.Context, w io.Writer) error {
	enc := json.NewEncoder(w)
	return e.forEachRow(ctx, func(row *datasims.SimulationStep) error {
		out := exportJSONRow{
			Step:        row.Metadata.Step,
			Type:        string(row.Metadata.Type),
			UID:         row.Metadata.UID,
			ReferenceID: row.Metadata.ReferenceID,
			State:       row.State,
			Decision:    row.Decision,
			Properties:  row.Properties,
			Statistics:  row.Statistics,
		}
		if row.Position != nil {
			x, y := row.Position.X, row.Position.Y
			out.X, out.Y = &x, &y
		}
		return enc.Encode(out)
	})
}

// exportPropertyColumns derives the CSV property-column set: the sorted union
// of every property UID the model snapshot declares (agent archetypes, shape
// templates, environment globals). Values outside the declared set are not
// invented; missing values render empty.
func exportPropertyColumns(m *datamodels.Model) []string {
	if m == nil {
		return nil
	}
	set := map[string]bool{}
	for i := range m.Agents {
		for _, p := range m.Agents[i].Properties {
			set[p.UID] = true
		}
	}
	if m.Environment != nil {
		for _, p := range m.Environment.GlobalProperties {
			set[p.UID] = true
		}
		for i := range m.Environment.Shapes {
			for _, p := range m.Environment.Shapes[i].Properties {
				set[p.UID] = true
			}
		}
	}
	cols := make([]string, 0, len(set))
	for uid := range set {
		cols = append(cols, uid)
	}
	sort.Strings(cols)
	return cols
}

// exportValueString renders a property value for a CSV cell: primitives as
// plain text (floats without a trailing .0), everything else as compact JSON.
func exportValueString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(t)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(b)
	}
}
