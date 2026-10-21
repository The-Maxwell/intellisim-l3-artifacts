package models

import (
	"intelli.sim.core/data/models"
	mongocommon "intelli.sim.core/data/mongodb/common"
)

// ChoiceWeightDocument is the bson form of models.ChoiceWeight.
type ChoiceWeightDocument struct {
	Value  interface{} `bson:"value"`
	Weight float64     `bson:"weight"`
}

// PropertyGenerationDocument is the bson form of models.PropertyGeneration.
type PropertyGenerationDocument struct {
	Mode          models.PropertyGenerationMode `bson:"mode"`
	Distribution  models.DistributionType       `bson:"distribution,omitempty"`
	Mean          *float64                      `bson:"mean,omitempty"`
	StdDev        *float64                      `bson:"std_dev,omitempty"`
	Min           *float64                      `bson:"min,omitempty"`
	Max           *float64                      `bson:"max,omitempty"`
	Choices       []ChoiceWeightDocument        `bson:"choices,omitempty"`
	PromptContext string                        `bson:"prompt_context,omitempty"`
}

// ---- mappers ----

func choiceWeightToDocument(c models.ChoiceWeight) ChoiceWeightDocument {
	return ChoiceWeightDocument{Value: c.Value, Weight: c.Weight}
}

func choiceWeightFromDocument(d ChoiceWeightDocument) models.ChoiceWeight {
	return models.ChoiceWeight{Value: mongocommon.NormalizeValue(d.Value), Weight: d.Weight}
}

func propertyGenerationPtrToDocument(g *models.PropertyGeneration) *PropertyGenerationDocument {
	if g == nil {
		return nil
	}
	d := PropertyGenerationDocument{
		Mode:          g.Mode,
		Distribution:  g.Distribution,
		Mean:          g.Mean,
		StdDev:        g.StdDev,
		Min:           g.Min,
		Max:           g.Max,
		Choices:       mongocommon.MapSlice(g.Choices, choiceWeightToDocument),
		PromptContext: g.PromptContext,
	}
	return &d
}

func propertyGenerationPtrFromDocument(d *PropertyGenerationDocument) *models.PropertyGeneration {
	if d == nil {
		return nil
	}
	return &models.PropertyGeneration{
		Mode:          d.Mode,
		Distribution:  d.Distribution,
		Mean:          d.Mean,
		StdDev:        d.StdDev,
		Min:           d.Min,
		Max:           d.Max,
		Choices:       mongocommon.MapSlice(d.Choices, choiceWeightFromDocument),
		PromptContext: d.PromptContext,
	}
}
