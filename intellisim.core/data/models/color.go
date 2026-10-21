package models

import "regexp"

// ColorPropertyUID is the conventional UID of the property that renderers read to
// colour an agent/shape. See PropertyTypeColor.
const ColorPropertyUID = "color"

// colorPalette mirrors the frontend SHAPE_PALETTE (violet-first) so an auto-stamped
// default colour matches what the canvas legend / palette-fallback would otherwise pick.
var colorPalette = []string{
	"#8B5CF6", "#3B82F6", "#22C55E", "#EF4444", "#F59E0B",
	"#EC4899", "#14B8A6", "#F97316", "#6366F1", "#84CC16",
}

// paletteColor returns a deterministic palette colour for a zero-based index.
func paletteColor(index int) string {
	if index < 0 {
		index = 0
	}
	return colorPalette[index%len(colorPalette)]
}

var hexColorRe = regexp.MustCompile(`^#([0-9A-Fa-f]{3,4}|[0-9A-Fa-f]{6}|[0-9A-Fa-f]{8})$`)

// IsHexColor reports whether s is a valid #RGB / #RGBA / #RRGGBB / #RRGGBBAA hex colour.
func IsHexColor(s string) bool {
	return hexColorRe.MatchString(s)
}

// hasColorProperty reports whether props already contains a colour-typed property. The
// check keys on Type (not UID) to match the renderer's "first colour-typed property"
// resolution, so we never add a second colour property.
func hasColorProperty(props []AgentProperty) bool {
	for i := range props {
		if props[i].Type == PropertyTypeColor {
			return true
		}
	}
	return false
}

// StampDefaultColorProperty ensures every agent and environment shape of a SPATIAL model
// carries the conventional "color" property, so entities always render with an explicit,
// editable colour instead of a positional palette fallback. Each entity gets a distinct
// palette default by index.
//
// Like StampDefaultArchetypeMode, it is applied ONLY in model creation/generation paths
// (never on a generic update, and never inside Canonical/NormalizeSpatial which run on
// every save) — so a user who deletes the property keeps it deleted (backward-safe). It
// only adds a colour property when the entity has none (keyed on Type == color), so an
// LLM- or user-supplied colour is preserved verbatim.
func (m *Model) StampDefaultColorProperty() {
	if m == nil || !m.IsSpatial {
		return
	}
	for i := range m.Agents {
		if !hasColorProperty(m.Agents[i].Properties) {
			m.Agents[i].Properties = append(m.Agents[i].Properties, AgentProperty{
				UID:          ColorPropertyUID,
				Name:         "color",
				Type:         PropertyTypeColor,
				DefaultValue: paletteColor(i),
			})
		}
	}
	if m.Environment != nil {
		for j := range m.Environment.Shapes {
			if !hasColorProperty(m.Environment.Shapes[j].Properties) {
				m.Environment.Shapes[j].Properties = append(m.Environment.Shapes[j].Properties, AgentProperty{
					UID:          ColorPropertyUID,
					Name:         "color",
					Type:         PropertyTypeColor,
					DefaultValue: paletteColor(j),
				})
			}
		}
	}
}
