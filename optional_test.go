package leadpush

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestOptionalJSONStates(t *testing.T) {
	t.Parallel()

	type payload struct {
		Omitted Optional[string] `json:"omitted,omitzero"`
		Value   Optional[bool]   `json:"value,omitzero"`
		Null    Optional[string] `json:"null,omitzero"`
		Empty   Optional[[]int]  `json:"empty,omitzero"`
	}

	encoded, err := json.Marshal(payload{
		Value: Some(false),
		Null:  Null[string](),
		Empty: Some([]int{}),
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	assertJSONEqual(t, encoded, `{"value":false,"null":null,"empty":[]}`)

	var decoded payload
	if err := json.Unmarshal([]byte(`{"value":true,"null":null,"empty":[1,2]}`), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.Omitted.IsSet() || !decoded.Null.IsNull() {
		t.Fatalf("decoded optional states = %#v", decoded)
	}
	value, ok := decoded.Value.Value()
	if !ok || !value {
		t.Fatalf("decoded value = %v, %v", value, ok)
	}
	empty, ok := decoded.Empty.Value()
	if !ok || !reflect.DeepEqual(empty, []int{1, 2}) {
		t.Fatalf("decoded slice = %#v, %v", empty, ok)
	}
}

func TestPtrPreservesZeroValues(t *testing.T) {
	t.Parallel()

	value := Ptr(false)
	if value == nil || *value {
		t.Fatalf("Ptr(false) = %#v", value)
	}
}
