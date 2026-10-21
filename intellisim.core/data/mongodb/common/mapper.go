package common

import (
	"intelli.sim.core/data/common"
)

// ---- Point ----

func PointToDocument(p common.Point) PointDocument   { return PointDocument{X: p.X, Y: p.Y} }
func PointFromDocument(d PointDocument) common.Point { return common.Point{X: d.X, Y: d.Y} }

func PointPtrToDocument(p *common.Point) *PointDocument {
	if p == nil {
		return nil
	}
	d := PointToDocument(*p)
	return &d
}

func PointPtrFromDocument(d *PointDocument) *common.Point {
	if d == nil {
		return nil
	}
	p := PointFromDocument(*d)
	return &p
}

// ---- LLMTarget ----

func ConfigValueToDocument(v common.ConfigValue) ConfigValueDocument {
	return ConfigValueDocument{Double: v.Double, Int: v.Int, Uint: v.Uint, Bool: v.Bool, String: v.String, List: v.List}
}

func ConfigValueFromDocument(d ConfigValueDocument) common.ConfigValue {
	return common.ConfigValue{Double: d.Double, Int: d.Int, Uint: d.Uint, Bool: d.Bool, String: d.String, List: d.List}
}

func generationToDocument(g common.GenerationParams) map[string]ConfigValueDocument {
	if len(g) == 0 {
		return nil
	}
	out := make(map[string]ConfigValueDocument, len(g))
	for k, v := range g {
		out[k] = ConfigValueToDocument(v)
	}
	return out
}

func generationFromDocument(d map[string]ConfigValueDocument) common.GenerationParams {
	if len(d) == 0 {
		return nil
	}
	out := make(common.GenerationParams, len(d))
	for k, v := range d {
		out[k] = ConfigValueFromDocument(v)
	}
	return out
}

func LLMTargetToDocument(t common.LLMTarget) LLMTargetDocument {
	return LLMTargetDocument{
		ProviderUID: t.ProviderUID, ModelUID: t.ModelUID, Effort: t.Effort,
		Generation: generationToDocument(t.Generation), TimeoutMs: t.TimeoutMs,
	}
}

func LLMTargetFromDocument(d LLMTargetDocument) common.LLMTarget {
	return common.LLMTarget{
		ProviderUID: d.ProviderUID, ModelUID: d.ModelUID, Effort: d.Effort,
		Generation: generationFromDocument(d.Generation), TimeoutMs: d.TimeoutMs,
	}
}

func LLMTargetPtrToDocument(t *common.LLMTarget) *LLMTargetDocument {
	if t == nil {
		return nil
	}
	d := LLMTargetToDocument(*t)
	return &d
}

func LLMTargetPtrFromDocument(d *LLMTargetDocument) *common.LLMTarget {
	if d == nil {
		return nil
	}
	t := LLMTargetFromDocument(*d)
	return &t
}

// ---- TokenUsage ----

func TokenUsageToDocument(u common.TokenUsage) TokenUsageDocument {
	return TokenUsageDocument{
		LLMTargetDocument: LLMTargetToDocument(u.LLMTarget),
		PromptTokens:      u.PromptTokens,
		CompletionTokens:  u.CompletionTokens,
		ReasoningTokens:   u.ReasoningTokens,
		TotalTokens:       u.TotalTokens,
		Calls:             u.Calls,
		LatencyMs:         u.LatencyMs,
	}
}

func TokenUsageFromDocument(d TokenUsageDocument) common.TokenUsage {
	return common.TokenUsage{
		LLMTarget:        LLMTargetFromDocument(d.LLMTargetDocument),
		PromptTokens:     d.PromptTokens,
		CompletionTokens: d.CompletionTokens,
		ReasoningTokens:  d.ReasoningTokens,
		TotalTokens:      d.TotalTokens,
		Calls:            d.Calls,
		LatencyMs:        d.LatencyMs,
	}
}

func TokenUsagePtrToDocument(u *common.TokenUsage) *TokenUsageDocument {
	if u == nil {
		return nil
	}
	d := TokenUsageToDocument(*u)
	return &d
}

func TokenUsagePtrFromDocument(d *TokenUsageDocument) *common.TokenUsage {
	if d == nil {
		return nil
	}
	u := TokenUsageFromDocument(*d)
	return &u
}

func TokenUsageSummaryToDocument(s common.TokenUsageSummary) TokenUsageSummaryDocument {
	return TokenUsageSummaryDocument{
		Total:   TokenUsageToDocument(s.Total),
		ByModel: MapSlice(s.ByModel, TokenUsageToDocument),
	}
}

func TokenUsageSummaryFromDocument(d TokenUsageSummaryDocument) common.TokenUsageSummary {
	return common.TokenUsageSummary{
		Total:   TokenUsageFromDocument(d.Total),
		ByModel: MapSlice(d.ByModel, TokenUsageFromDocument),
	}
}

func TokenUsageSummaryPtrToDocument(s *common.TokenUsageSummary) *TokenUsageSummaryDocument {
	if s == nil {
		return nil
	}
	d := TokenUsageSummaryToDocument(*s)
	return &d
}

func TokenUsageSummaryPtrFromDocument(d *TokenUsageSummaryDocument) *common.TokenUsageSummary {
	if d == nil {
		return nil
	}
	s := TokenUsageSummaryFromDocument(*d)
	return &s
}

// ---- File ----

func FileToDocument(f common.File) (FileDocument, error) {
	gridID, err := ParseOptionalID(f.GridFSID)
	if err != nil {
		return FileDocument{}, err
	}
	return FileDocument{
		UID: f.UID, Name: f.Name, Type: f.Type, MimeType: f.MimeType, Size: f.Size,
		GridFSID: gridID, Step: f.Step, Description: f.Description,
		CreatedAt: f.CreatedAt, CreatedBy: f.CreatedBy,
	}, nil
}

func FileFromDocument(d FileDocument) common.File {
	return common.File{
		UID: d.UID, Name: d.Name, Type: d.Type, MimeType: d.MimeType, Size: d.Size,
		GridFSID: FormatOptionalID(d.GridFSID), Step: d.Step, Description: d.Description,
		CreatedAt: d.CreatedAt, CreatedBy: d.CreatedBy,
	}
}
