package common

import (
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestNormalizeValueNested(t *testing.T) {
	in := bson.D{
		{Key: "a", Value: bson.A{int32(1), bson.D{{Key: "b", Value: "x"}}}},
		{Key: "c", Value: bson.M{"d": int64(2)}},
	}
	want := map[string]interface{}{
		"a": []interface{}{int32(1), map[string]interface{}{"b": "x"}},
		"c": map[string]interface{}{"d": int64(2)},
	}
	if got := NormalizeValue(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("normalize mismatch:\n got  %#v\n want %#v", got, want)
	}
}

func TestNormalizeMapRebuildsContainers(t *testing.T) {
	in := map[string]interface{}{"k": bson.A{"v"}}
	out := NormalizeMap(in)
	if _, isBsonA := out["k"].(bson.A); isBsonA {
		t.Fatal("bson.A leaked through NormalizeMap")
	}
	if NormalizeMap(nil) != nil {
		t.Fatal("nil map must stay nil")
	}
}

func TestNormalizeDateTime(t *testing.T) {
	dt := bson.NewDateTimeFromTime(time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC))
	got, ok := NormalizeValue(dt).(time.Time)
	if !ok {
		t.Fatalf("bson.DateTime not converted to time.Time: %T", NormalizeValue(dt))
	}
	if !got.Equal(dt.Time()) {
		t.Fatalf("time mismatch: got %v want %v", got, dt.Time())
	}
}
