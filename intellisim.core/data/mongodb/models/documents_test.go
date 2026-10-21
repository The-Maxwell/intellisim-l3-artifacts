package models

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	mongocommon "intelli.sim.core/data/mongodb/common"
)

func TestModelImageAndConfigModelUIDRoundTrip(t *testing.T) {
	doc := ModelDocument{
		Name: "predator-prey",
		LLMConfig: LLMConfigDocument{
			Target: mongocommon.LLMTargetDocument{
				ProviderUID: "openai",
				ModelUID:    "gpt-4o",
				Effort:      common.LLMEffortHigh,
			},
		},
		Images: []ModelImageDocument{
			{UID: "img1", Name: "layout.png", MimeType: "image/png", Data: "aGVsbG8="},
		},
		Agents: []AgentDocument{
			{
				UID:  "predator",
				Name: "Predator",
				LLMConfig: &LLMAgentConfigDocument{
					LLMConfigDocument: LLMConfigDocument{
						Target: mongocommon.LLMTargetDocument{
							ModelUID: "gpt-4o-mini",
							Effort:   common.LLMEffortLow,
						},
					},
				},
				Properties: []AgentPropertyDocument{
					{
						UID:          "color",
						Name:         "Color",
						Type:         models.PropertyTypeColor,
						DefaultValue: "#FF8800",
						Immutable:    true,
					},
				},
			},
		},
	}

	data, err := bson.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out ModelDocument
	if err := bson.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// LLMConfig target preserved
	if out.LLMConfig.Target.ProviderUID != "openai" {
		t.Errorf("LLMConfig.Target.ProviderUID: got %q, want %q", out.LLMConfig.Target.ProviderUID, "openai")
	}
	if out.LLMConfig.Target.ModelUID != "gpt-4o" {
		t.Errorf("LLMConfig.Target.ModelUID: got %q, want %q", out.LLMConfig.Target.ModelUID, "gpt-4o")
	}
	if out.LLMConfig.Target.Effort != common.LLMEffortHigh {
		t.Errorf("LLMConfig.Target.Effort: got %q, want %q", out.LLMConfig.Target.Effort, common.LLMEffortHigh)
	}

	// Images preserved
	if len(out.Images) != 1 {
		t.Fatalf("Images: got %d, want 1", len(out.Images))
	}
	img := out.Images[0]
	if img.UID != "img1" {
		t.Errorf("Images[0].UID: got %q, want %q", img.UID, "img1")
	}
	if img.Name != "layout.png" {
		t.Errorf("Images[0].Name: got %q, want %q", img.Name, "layout.png")
	}
	if img.MimeType != "image/png" {
		t.Errorf("Images[0].MimeType: got %q, want %q", img.MimeType, "image/png")
	}
	if img.Data != "aGVsbG8=" {
		t.Errorf("Images[0].Data: got %q, want %q", img.Data, "aGVsbG8=")
	}

	// Agents preserved
	if len(out.Agents) != 1 {
		t.Fatalf("Agents: got %d, want 1", len(out.Agents))
	}
	agent := out.Agents[0]
	if agent.Name != "Predator" {
		t.Errorf("Agents[0].Name: got %q, want %q", agent.Name, "Predator")
	}
	if len(agent.Properties) != 1 {
		t.Fatalf("Agents[0].Properties: got %d, want 1", len(agent.Properties))
	}
	prop := agent.Properties[0]
	if prop.Type != models.PropertyTypeColor {
		t.Errorf("property Type: got %q, want %q", prop.Type, models.PropertyTypeColor)
	}
	if !prop.Immutable {
		t.Error("property Immutable: got false, want true")
	}
	if prop.DefaultValue != "#FF8800" {
		t.Errorf("property DefaultValue: got %v, want %q", prop.DefaultValue, "#FF8800")
	}

	// Per-agent LLM config target preserved
	if agent.LLMConfig == nil {
		t.Fatal("agent LLMConfig: got nil")
	}
	if agent.LLMConfig.Target.ModelUID != "gpt-4o-mini" {
		t.Errorf("agent LLMConfig.Target.ModelUID: got %q, want %q", agent.LLMConfig.Target.ModelUID, "gpt-4o-mini")
	}
	if agent.LLMConfig.Target.Effort != common.LLMEffortLow {
		t.Errorf("agent LLMConfig.Target.Effort: got %q, want %q", agent.LLMConfig.Target.Effort, common.LLMEffortLow)
	}
}

func TestShapeGeometryRoundTrip(t *testing.T) {
	minRadius, maxRadius := uint64(4), uint64(8)
	width, height := uint64(20), uint64(10)
	strokeWidth := uint(2)
	fillTrue := true

	env := EnvironmentDocument{
		UID: models.EnvironmentUID,
		Shapes: []ShapeDocument{
			{
				UID:  "tree",
				Name: "Tree",
				Type: models.ShapeTypeCircle,
				Geometry: &ShapeGeometryDocument{
					Radius:      &ScalarSpecDocument{Min: &minRadius, Max: &maxRadius},
					Fill:        &fillTrue,
					StrokeWidth: &strokeWidth,
				},
				Initialization: AgentInitializationDocument{
					PopulationCount: 5,
					Placement:       &PlacementDocument{Type: models.PlacementRandom},
				},
			},
			{
				UID:  "wall",
				Name: "Wall",
				Type: models.ShapeTypePolygon,
				Geometry: &ShapeGeometryDocument{
					Position: &mongocommon.PointDocument{X: 30, Y: 40},
					Width:    &ScalarSpecDocument{Value: &width},
					Height:   &ScalarSpecDocument{Value: &height},
					Points: []mongocommon.PointDocument{
						{X: 0, Y: 0},
						{X: 10, Y: 0},
						{X: 10, Y: 5},
					},
				},
				Initialization: AgentInitializationDocument{
					PopulationCount: 1,
					Placement:       &PlacementDocument{Type: models.PlacementManual},
				},
			},
		},
	}

	data, err := bson.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var out EnvironmentDocument
	if err := bson.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(out.Shapes) != 2 {
		t.Fatalf("Shapes: got %d, want 2", len(out.Shapes))
	}

	// Circle shape assertions
	circle := out.Shapes[0]
	if circle.Type != models.ShapeTypeCircle {
		t.Errorf("circle Type: got %q, want %q", circle.Type, models.ShapeTypeCircle)
	}
	if circle.Geometry == nil || circle.Geometry.Radius == nil {
		t.Fatal("circle Geometry.Radius: got nil")
	}
	if circle.Geometry.Radius.Min == nil || *circle.Geometry.Radius.Min != minRadius {
		t.Errorf("circle Radius.Min: got %v, want %d", circle.Geometry.Radius.Min, minRadius)
	}
	if circle.Geometry.Radius.Max == nil || *circle.Geometry.Radius.Max != maxRadius {
		t.Errorf("circle Radius.Max: got %v, want %d", circle.Geometry.Radius.Max, maxRadius)
	}
	if circle.Geometry.Fill == nil || !*circle.Geometry.Fill {
		t.Error("circle Fill: got nil or false, want true")
	}
	if circle.Geometry.StrokeWidth == nil || *circle.Geometry.StrokeWidth != strokeWidth {
		t.Errorf("circle StrokeWidth: got %v, want %d", circle.Geometry.StrokeWidth, strokeWidth)
	}
	if circle.Initialization.PopulationCount != 5 {
		t.Errorf("circle PopulationCount: got %d, want 5", circle.Initialization.PopulationCount)
	}
	if circle.Initialization.Placement == nil || circle.Initialization.Placement.Type != models.PlacementRandom {
		t.Errorf("circle Placement.Type: got %v, want %q", circle.Initialization.Placement, models.PlacementRandom)
	}

	// Polygon shape assertions
	polygon := out.Shapes[1]
	if polygon.Type != models.ShapeTypePolygon {
		t.Errorf("polygon Type: got %q, want %q", polygon.Type, models.ShapeTypePolygon)
	}
	if polygon.Geometry == nil {
		t.Fatal("polygon Geometry: got nil")
	}
	if polygon.Geometry.Position == nil || *polygon.Geometry.Position != (mongocommon.PointDocument{X: 30, Y: 40}) {
		t.Errorf("polygon Position: got %+v, want {X:30 Y:40}", polygon.Geometry.Position)
	}
	if polygon.Geometry.Width == nil || polygon.Geometry.Width.Value == nil || *polygon.Geometry.Width.Value != width {
		t.Errorf("polygon Width.Value: got %v, want %d", polygon.Geometry.Width, width)
	}
	if polygon.Geometry.Height == nil || polygon.Geometry.Height.Value == nil || *polygon.Geometry.Height.Value != height {
		t.Errorf("polygon Height.Value: got %v, want %d", polygon.Geometry.Height, height)
	}
	if len(polygon.Geometry.Points) != 3 {
		t.Fatalf("polygon Points: got %d, want 3", len(polygon.Geometry.Points))
	}
	expectedPoints := []mongocommon.PointDocument{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 5}}
	for i, pt := range polygon.Geometry.Points {
		if pt != expectedPoints[i] {
			t.Errorf("polygon Points[%d]: got %+v, want %+v", i, pt, expectedPoints[i])
		}
	}
	if polygon.Initialization.PopulationCount != 1 {
		t.Errorf("polygon PopulationCount: got %d, want 1", polygon.Initialization.PopulationCount)
	}
	if polygon.Initialization.Placement == nil || polygon.Initialization.Placement.Type != models.PlacementManual {
		t.Errorf("polygon Placement.Type: got %v, want %q", polygon.Initialization.Placement, models.PlacementManual)
	}
}

func countKeys(t *testing.T, v interface{}, key string) int {
	t.Helper()
	data, err := bson.Marshal(v)
	if err != nil {
		t.Fatalf("countKeys marshal: %v", err)
	}
	var d bson.D
	if err := bson.Unmarshal(data, &d); err != nil {
		t.Fatalf("countKeys unmarshal: %v", err)
	}
	count := 0
	for _, e := range d {
		if e.Key == key {
			count++
		}
	}
	return count
}

func TestLLMShapeStrategyNoDuplicateKeys(t *testing.T) {
	s := LLMShapeStrategyDocument{
		LLMStrategyDocument: LLMStrategyDocument{
			Type:           models.LLMStrategyFull,
			UseForMovement: true,
		},
	}
	if n := countKeys(t, s, "use_for_movement"); n != 1 {
		t.Errorf("use_for_movement key count: got %d, want 1", n)
	}
}

func TestLLMSimulationConfigInlineRoundTrip(t *testing.T) {
	doc := LLMSimulationExecutionConfigDocument{
		LLMSimulationAgentConfigDocument: LLMSimulationAgentConfigDocument{
			PerceptionRadius: 50,
			MemorySize:       20,
		},
		ParallelWorkers: 3,
		RetryTimeoutMs:  30000,
		MaxRetries:      3,
		StreamBatchSize: 100,
	}

	// exactly one inlined "perception_radius" key
	if n := countKeys(t, doc, "perception_radius"); n != 1 {
		t.Errorf("perception_radius key count: got %d, want 1", n)
	}

	data, err := bson.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out LLMSimulationExecutionConfigDocument
	if err := bson.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if out.PerceptionRadius != 50 {
		t.Errorf("PerceptionRadius: got %v, want 50", out.PerceptionRadius)
	}
	if out.MemorySize != 20 {
		t.Errorf("MemorySize: got %v, want 20", out.MemorySize)
	}
	if out.ParallelWorkers != 3 {
		t.Errorf("ParallelWorkers: got %v, want 3", out.ParallelWorkers)
	}
	if out.RetryTimeoutMs != 30000 {
		t.Errorf("RetryTimeoutMs: got %v, want 30000", out.RetryTimeoutMs)
	}
	if out.MaxRetries != 3 {
		t.Errorf("MaxRetries: got %v, want 3", out.MaxRetries)
	}
	if out.StreamBatchSize != 100 {
		t.Errorf("StreamBatchSize: got %v, want 100", out.StreamBatchSize)
	}
}
