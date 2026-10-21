package simulations

import (
	"fmt"

	"intelli.sim.core/data/models"
)

// terminationConditionTypes is the restricted condition subset legal for a run
// termination condition (U2). Agent-positional/state conditions are meaningless
// for a whole-run stop; random/event would make the stop nondeterministic or
// order-coupled. composite may only nest these same types.
var terminationConditionTypes = map[models.ConditionType]bool{
	models.ConditionTypePopulation:     true,
	models.ConditionTypeStep:           true,
	models.ConditionTypeGlobalProperty: true,
	models.ConditionTypeComposite:      true,
}

// evalTermination evaluates a run-termination condition (U2) at a step boundary,
// after the step's mutations have committed. Only the restricted subset above is
// honored; every other condition type is never-true (defense in depth behind
// validation).
func evalTermination(rs *RuntimeState, c models.Condition) bool {
	switch c.Type {
	case models.ConditionTypePopulation:
		return compareValues(float64(livePopulation(rs, populationFilter(c))), c.Operator, c.Value, c.MinValue, c.MaxValue)
	case models.ConditionTypeStep:
		return compareValues(float64(rs.Step), c.Operator, c.Value, c.MinValue, c.MaxValue)
	case models.ConditionTypeGlobalProperty:
		return evalGlobalProperty(rs, c)
	case models.ConditionTypeComposite:
		return evalTerminationComposite(rs, c)
	default:
		return false
	}
}

func evalTerminationComposite(rs *RuntimeState, c models.Condition) bool {
	if len(c.SubConditions) == 0 {
		return false
	}
	if c.LogicOperator == models.LogicOpNOT {
		return !evalTermination(rs, c.SubConditions[0])
	}
	isAnd := c.LogicOperator == models.LogicOpAND || c.LogicOperator == ""
	for _, sub := range c.SubConditions {
		ok := evalTermination(rs, sub)
		if isAnd && !ok {
			return false
		}
		if !isAnd && ok {
			return true
		}
	}
	return isAnd
}

// populationFilter collects the archetype-UID filter of a population condition
// (TargetUIDs plus the single TargetUID). Empty set = count every archetype.
func populationFilter(c models.Condition) map[string]bool {
	if len(c.TargetUIDs) == 0 && c.TargetUID == "" {
		return nil
	}
	set := make(map[string]bool, len(c.TargetUIDs)+1)
	for _, uid := range c.TargetUIDs {
		set[uid] = true
	}
	if c.TargetUID != "" {
		set[c.TargetUID] = true
	}
	return set
}

// livePopulation counts live (non-removed) agents, optionally restricted to the
// given archetype-UID set (nil = all).
func livePopulation(rs *RuntimeState, uids map[string]bool) int {
	n := 0
	for _, ra := range rs.Agents {
		if ra == nil || ra.A == nil || ra.removed {
			continue
		}
		if uids != nil && !uids[ra.A.AgentUID] {
			continue
		}
		n++
	}
	return n
}

// validateTerminationCondition enforces the termination-condition contract
// recursively: only the restricted type subset, an operator + a comparable
// value for the leaf types, a property_uid for global_property, and non-empty
// sub_conditions for composite. Errors go through addErr with a
// "termination[.sub_conditions[i]]" path.
func validateTerminationCondition(addErr func(path, reason string), path string, c models.Condition) {
	if !terminationConditionTypes[c.Type] {
		addErr(path, fmt.Sprintf("unsupported termination condition type %q (allowed: population, step, global_property, composite)", c.Type))
		return
	}
	if c.Type == models.ConditionTypeComposite {
		if len(c.SubConditions) == 0 {
			addErr(path, "composite termination condition requires sub_conditions")
		}
		for i := range c.SubConditions {
			validateTerminationCondition(addErr, fmt.Sprintf("%s.sub_conditions[%d]", path, i), c.SubConditions[i])
		}
		return
	}
	if c.Operator == "" {
		addErr(path, string(c.Type)+" termination condition requires an operator")
	}
	if c.Value == nil && c.MinValue == nil && c.MaxValue == nil {
		addErr(path, string(c.Type)+" termination condition requires a value (or min_value/max_value)")
	}
	if c.Type == models.ConditionTypeGlobalProperty && c.PropertyUID == "" {
		addErr(path, "global_property termination condition requires property_uid")
	}
}

// cloneTerminationCondition deep-copies a termination condition so stamped /
// duplicated simulations never alias the source's condition tree.
func cloneTerminationCondition(c *models.Condition) *models.Condition {
	if c == nil {
		return nil
	}
	cp := *c
	if c.MinValue != nil {
		v := *c.MinValue
		cp.MinValue = &v
	}
	if c.MaxValue != nil {
		v := *c.MaxValue
		cp.MaxValue = &v
	}
	if c.Radius != nil {
		v := *c.Radius
		cp.Radius = &v
	}
	cp.TargetUIDs = append([]string(nil), c.TargetUIDs...)
	if c.SubConditions != nil {
		cp.SubConditions = make([]models.Condition, len(c.SubConditions))
		for i := range c.SubConditions {
			cp.SubConditions[i] = *cloneTerminationCondition(&c.SubConditions[i])
		}
	}
	if c.Parameters != nil {
		params := make(map[string]interface{}, len(c.Parameters))
		for k, v := range c.Parameters {
			params[k] = v
		}
		cp.Parameters = params
	}
	return &cp
}
