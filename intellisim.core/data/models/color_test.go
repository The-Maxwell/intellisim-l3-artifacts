package models

import "testing"

func TestIsHexColor(t *testing.T) {
	valid := []string{"#fff", "#FFFF", "#8B5CF6", "#8b5cf6", "#12345678"}
	for _, v := range valid {
		if !IsHexColor(v) {
			t.Errorf("IsHexColor(%q) = false, want true", v)
		}
	}
	invalid := []string{"", "fff", "#", "#12", "#12345", "red", "#GGGGGG", "#1234567", "8B5CF6"}
	for _, v := range invalid {
		if IsHexColor(v) {
			t.Errorf("IsHexColor(%q) = true, want false", v)
		}
	}
}

func TestStampDefaultColorProperty(t *testing.T) {
	// Non-spatial: never stamps.
	ns := &Model{IsSpatial: false, Agents: []Agent{{UID: "a"}}}
	ns.StampDefaultColorProperty()
	if hasColorProperty(ns.Agents[0].Properties) {
		t.Fatal("non-spatial model should not get a colour property")
	}

	m := &Model{
		IsSpatial: true,
		Agents: []Agent{
			{UID: "a0"},
			{UID: "a1", Properties: []AgentProperty{{UID: "mycol", Type: PropertyTypeColor, DefaultValue: "#123456"}}},
		},
		Environment: &Environment{Shapes: []Shape{{UID: "s0"}}},
	}
	m.StampDefaultColorProperty()

	// a0 gets a stamped colour with palette[0].
	if !hasColorProperty(m.Agents[0].Properties) {
		t.Fatal("agent 0 should have a colour property")
	}
	got := m.Agents[0].Properties[len(m.Agents[0].Properties)-1]
	if got.UID != ColorPropertyUID || got.Type != PropertyTypeColor || got.DefaultValue != colorPalette[0] {
		t.Errorf("agent 0 colour = %+v, want uid=color type=color default=%s", got, colorPalette[0])
	}
	// a1 already had a colour-typed property (different UID) → not doubled.
	cnt := 0
	for _, p := range m.Agents[1].Properties {
		if p.Type == PropertyTypeColor {
			cnt++
		}
	}
	if cnt != 1 {
		t.Errorf("agent 1 colour-typed property count = %d, want 1 (no duplicate)", cnt)
	}
	// shape gets a stamped colour with palette[0].
	if !hasColorProperty(m.Environment.Shapes[0].Properties) {
		t.Fatal("shape 0 should have a colour property")
	}

	// Idempotent: a second stamp adds nothing.
	before := len(m.Agents[0].Properties)
	m.StampDefaultColorProperty()
	if len(m.Agents[0].Properties) != before {
		t.Error("second stamp must be idempotent")
	}
}
