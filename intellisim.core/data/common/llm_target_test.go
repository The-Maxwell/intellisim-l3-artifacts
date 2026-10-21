package common

import (
	"encoding/json"
	"testing"
)

func TestConfigValueJSON_TaggedSingleKey(t *testing.T) {
	f := 0.7
	b, _ := json.Marshal(ConfigValue{Double: &f})
	if string(b) != `{"double":0.7}` {
		t.Fatalf("got %s", b)
	}
	var v ConfigValue
	if err := json.Unmarshal([]byte(`{"uint":256}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.Uint == nil || *v.Uint != 256 || v.Double != nil {
		t.Fatalf("round-trip mismatch: %+v", v)
	}
}

func TestLLMTargetJSON_OmitsEmptyGenerationAndTimeout(t *testing.T) {
	b, _ := json.Marshal(LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"})
	if string(b) != `{"provider_uid":"openai","model_uid":"gpt-4o"}` {
		t.Fatalf("got %s", b)
	}
}
