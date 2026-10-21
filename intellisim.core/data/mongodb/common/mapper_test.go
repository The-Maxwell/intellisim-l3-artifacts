package common

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"intelli.sim.core/data/common"
)

func TestLLMTargetBSONRoundTrip(t *testing.T) {
	u := uint64(256)
	to := uint64(30000)
	in := common.LLMTarget{
		ProviderUID: "openai", ModelUID: "gpt-4o",
		Generation: common.GenerationParams{"max_tokens": {Uint: &u}, "stop": {List: []string{"END"}}},
		TimeoutMs:  &to,
	}
	raw, err := bson.Marshal(LLMTargetToDocument(in))
	if err != nil {
		t.Fatal(err)
	}
	var doc LLMTargetDocument
	if err := bson.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	out := LLMTargetFromDocument(doc)
	if out.Generation["max_tokens"].Uint == nil || *out.Generation["max_tokens"].Uint != 256 {
		t.Fatalf("max_tokens lost: %+v", out.Generation)
	}
	if out.TimeoutMs == nil || *out.TimeoutMs != 30000 {
		t.Fatalf("timeout lost: %+v", out.TimeoutMs)
	}
}

func TestConfigValueBSONAllVariants(t *testing.T) {
	d := 0.7
	i := int64(-3)
	u := uint64(256)
	b := true
	s := "json"
	cases := map[string]common.ConfigValue{
		"d": {Double: &d},
		"i": {Int: &i},
		"u": {Uint: &u},
		"b": {Bool: &b},
		"s": {String: &s},
		"l": {List: []string{"a", "b"}},
	}
	for name, in := range cases {
		raw, err := bson.Marshal(ConfigValueToDocument(in))
		if err != nil {
			t.Fatalf("%s: marshal: %v", name, err)
		}
		var doc ConfigValueDocument
		if err := bson.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}
		if got := ConfigValueFromDocument(doc); !reflect.DeepEqual(got, in) {
			t.Fatalf("%s: round-trip mismatch: got %+v want %+v", name, got, in)
		}
	}
}

func TestLLMTargetBSON_ChatOmitsExtras(t *testing.T) {
	raw, _ := bson.Marshal(LLMTargetToDocument(common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"}))
	var m bson.M
	_ = bson.Unmarshal(raw, &m)
	if _, ok := m["generation"]; ok {
		t.Fatal("empty generation must be omitted")
	}
	if _, ok := m["timeout_ms"]; ok {
		t.Fatal("nil timeout_ms must be omitted")
	}
}
