package simulations

import (
	"sort"

	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// ComputeStatistics derives a SimulationStatistics snapshot from the current
// in-memory runtime state. It fills the fields that are determinable from a
// single tick snapshot:
//
//   - StepsCompleted   — rs.Step (current tick count).
//   - TotalAgents      — number of live (non-removed) agents.
//   - Population        — per archetype: current count + live state distribution
//     (ByState keyed by SimulationAgent.State).
//   - Properties        — per archetype, per number-typed AgentProperty:
//     avg/min/max across that archetype's live agents.
//
// The accumulating counters (AgentsSpawned/AgentsRemoved/Interactions/Events/
// RuleExecutions/MovementDistance and the ByRule/ByEvent/ByInteraction
// breakdowns) and the Population Peak/Min/Spawned/Removed fields are run-history
// metrics that a single snapshot cannot reconstruct; they are left zero here and
// are owned by the run loop (Task 15), which accumulates them across ticks and
// merges them into the persisted statistics. Custom is left nil.
func ComputeStatistics(rs *RuntimeState) *datasims.SimulationStatistics {
	if rs == nil {
		return &datasims.SimulationStatistics{}
	}
	st := &datasims.SimulationStatistics{
		StepsCompleted: rs.Step,
	}

	// Group live agents by archetype UID, preserving first-seen order so the
	// output Population/Properties slices are stable for a given agent set.
	order := make([]string, 0, len(rs.Agents))
	byArchetype := make(map[string][]*RuntimeAgent, len(rs.Agents))
	for _, ra := range rs.Agents {
		if ra == nil || ra.A == nil || ra.removed {
			continue
		}
		uid := ra.A.AgentUID
		if _, seen := byArchetype[uid]; !seen {
			order = append(order, uid)
		}
		byArchetype[uid] = append(byArchetype[uid], ra)
	}

	for _, uid := range order {
		members := byArchetype[uid]
		st.TotalAgents += uint64(len(members))

		pop := datasims.AgentPopulation{
			AgentUID: uid,
			Current:  uint64(len(members)),
		}
		byState := make(map[string]uint64)
		for _, ra := range members {
			if ra.A.State != "" {
				byState[ra.A.State]++
			}
		}
		if len(byState) > 0 {
			pop.ByState = byState
		}
		st.Population = append(st.Population, pop)

		st.Properties = append(st.Properties, numericPropertyStats(uid, members)...)
	}

	// Merge run-history counters from the stats accumulator when present.
	// The guard keeps a nil accumulator producing the same zero output as before
	// (backward-compat: callers that never wired an accumulator are unaffected).
	if rs.stats != nil {
		acc := rs.stats

		// Flat counters: copy directly from the accumulator.
		st.AgentsSpawned = acc.AgentsSpawned
		st.AgentsRemoved = acc.AgentsRemoved
		st.Interactions = acc.Interactions
		st.Events = acc.Events
		st.RuleExecutions = acc.RuleExecutions
		st.MovementDistance = acc.MovementDistance

		// Per-archetype Peak/Min/Spawned/Removed: annotate the Population rows that
		// were just built for live archetypes.
		for i := range st.Population {
			uid := st.Population[i].AgentUID
			st.Population[i].Peak = acc.PopPeak[uid]
			st.Population[i].Min = acc.PopMin[uid]
			st.Population[i].Spawned = acc.PopSpawned[uid]
			st.Population[i].Removed = acc.PopRemoved[uid]
		}

		// Extinct archetypes: append a zero-Current row for every archetype UID
		// that appears in the accumulator maps but has no live agents (not in order).
		emitted := make(map[string]bool, len(order))
		for _, uid := range order {
			emitted[uid] = true
		}
		// Collect extinct UIDs from all three accumulator maps so none are missed.
		extinctSeen := make(map[string]bool)
		collectExtinct := func(m map[string]uint64) {
			for uid := range m {
				if !emitted[uid] && !extinctSeen[uid] {
					extinctSeen[uid] = true
				}
			}
		}
		collectExtinct(acc.PopSpawned)
		collectExtinct(acc.PopRemoved)
		collectExtinct(acc.PopPeak)

		// Sort extinct UIDs for deterministic output order.
		extinct := make([]string, 0, len(extinctSeen))
		for uid := range extinctSeen {
			extinct = append(extinct, uid)
		}
		sort.Strings(extinct)

		for _, uid := range extinct {
			st.Population = append(st.Population, datasims.AgentPopulation{
				AgentUID: uid,
				Current:  0,
				Peak:     acc.PopPeak[uid],
				Min:      acc.PopMin[uid],
				Spawned:  acc.PopSpawned[uid],
				Removed:  acc.PopRemoved[uid],
			})
		}

		// ByRule: key-sorted slice from the ByRule map.
		if len(acc.ByRule) > 0 {
			ruleKeys := make([]string, 0, len(acc.ByRule))
			for k := range acc.ByRule {
				ruleKeys = append(ruleKeys, k)
			}
			sort.Strings(ruleKeys)
			st.ByRule = make([]datasims.RuleStatistic, 0, len(ruleKeys))
			for _, k := range ruleKeys {
				st.ByRule = append(st.ByRule, datasims.RuleStatistic{RuleID: k, Executions: acc.ByRule[k]})
			}
		}

		// ByEvent: key-sorted slice from the ByEvent map.
		if len(acc.ByEvent) > 0 {
			evKeys := make([]string, 0, len(acc.ByEvent))
			for k := range acc.ByEvent {
				evKeys = append(evKeys, k)
			}
			sort.Strings(evKeys)
			st.ByEvent = make([]datasims.EventStatistic, 0, len(evKeys))
			for _, k := range evKeys {
				st.ByEvent = append(st.ByEvent, datasims.EventStatistic{EventUID: k, Emitted: acc.ByEvent[k]})
			}
		}

		// ByInteraction: key-sorted slice from the ByInteraction map.
		if len(acc.ByInteraction) > 0 {
			intKeys := make([]string, 0, len(acc.ByInteraction))
			for k := range acc.ByInteraction {
				intKeys = append(intKeys, k)
			}
			sort.Strings(intKeys)
			st.ByInteraction = make([]datasims.InteractionStatistic, 0, len(intKeys))
			for _, k := range intKeys {
				st.ByInteraction = append(st.ByInteraction, datasims.InteractionStatistic{InteractionID: k, Count: acc.ByInteraction[k]})
			}
		}
	}

	return st
}

// numericPropertyStats computes avg/min/max for every number-typed property of an
// archetype across its live members. Properties absent or non-numeric on a member
// are skipped for that member; a property contributing no numeric samples is
// omitted entirely. The property set comes from the resolved archetype definition
// so the output order is deterministic and only declared numeric properties are
// aggregated.
func numericPropertyStats(uid string, members []*RuntimeAgent) []datasims.PropertyStatistic {
	if len(members) == 0 {
		return nil
	}

	// Resolve the archetype definition once (all members share it).
	var archetype *models.Agent
	for _, ra := range members {
		if ra.Archetype != nil {
			archetype = ra.Archetype
			break
		}
	}
	if archetype == nil {
		return nil
	}

	out := make([]datasims.PropertyStatistic, 0)
	for _, prop := range archetype.Properties {
		if prop.Type != models.PropertyTypeNumber {
			continue
		}
		var (
			sum   float64
			min   float64
			max   float64
			count uint64
		)
		for _, ra := range members {
			raw, ok := ra.A.Properties[prop.UID]
			if !ok {
				continue
			}
			v, ok := coerceFloat(raw)
			if !ok {
				continue
			}
			if count == 0 {
				min, max = v, v
			} else {
				if v < min {
					min = v
				}
				if v > max {
					max = v
				}
			}
			sum += v
			count++
		}
		if count == 0 {
			continue
		}
		out = append(out, datasims.PropertyStatistic{
			AgentUID:    uid,
			PropertyUID: prop.UID,
			Avg:         sum / float64(count),
			Min:         min,
			Max:         max,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
