package common

import "testing"

func TestModelCapabilityValid(t *testing.T) {
	for _, c := range AllModelCapabilities {
		if !c.Valid() {
			t.Errorf("capability %q must be valid", c)
		}
	}
	for _, c := range []ModelCapability{"", "custom_thing", "CHAT"} {
		if c.Valid() {
			t.Errorf("capability %q must be invalid", c)
		}
	}
}
